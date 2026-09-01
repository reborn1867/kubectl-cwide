package alias

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/kubectl-cwide/pkg/models"
	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// readAll drains r fully. Broken out so both file and stdin paths share it.
func readAll(r io.Reader) ([]byte, error) { return io.ReadAll(r) }

// aliasBundle is the on-wire format for alias export/import. It intentionally
// mirrors the alias-relevant subset of models.Config so bundles can round-trip
// with schema-aware tools (yq, jq via json, etc.) without pulling in the
// unrelated fields like TemplatePath. Version stamps let the format evolve
// without silently corrupting older imports.
type aliasBundle struct {
	Version      int                          `yaml:"version"`
	Aliases      map[string]string            `yaml:"aliases,omitempty"`
	AliasEntries map[string]models.AliasEntry `yaml:"aliasEntries,omitempty"`
}

const aliasBundleVersion = 1

// NewCmdAliasExport writes the local alias section to a YAML file (or stdout)
// so users can share their alias set without pushing to a cluster ConfigMap.
func NewCmdAliasExport() *cobra.Command {
	var outPath string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export configured aliases to a YAML file",
		Long: `Write the local alias configuration (both legacy string aliases and rich
alias entries with template bindings) to a portable YAML bundle. Use --out to
write to a file; omit it to write to stdout.

The bundle can be shared over any channel (email, chat, git) and re-applied
with 'kubectl cwide alias import'.`,
		Example: `  # Export to a file
  kubectl cwide alias export --out my-aliases.yaml

  # Export to stdout for piping
  kubectl cwide alias export | ssh other-host 'kubectl cwide alias import -'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := utils.LoadConfig()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			b := aliasBundle{
				Version:      aliasBundleVersion,
				Aliases:      cfg.Aliases,
				AliasEntries: cfg.AliasEntries,
			}
			if len(b.Aliases) == 0 && len(b.AliasEntries) == 0 {
				return fmt.Errorf("no aliases to export")
			}
			data, err := yaml.Marshal(&b)
			if err != nil {
				return fmt.Errorf("marshal bundle: %w", err)
			}
			if outPath == "" || outPath == "-" {
				_, err = cmd.OutOrStdout().Write(data)
				return err
			}
			if err := os.WriteFile(outPath, data, 0644); err != nil {
				return fmt.Errorf("write %s: %w", outPath, err)
			}
			// Deterministic count reporting for user-facing output.
			legacy := len(b.Aliases)
			rich := len(b.AliasEntries)
			fmt.Fprintf(cmd.OutOrStdout(), "Exported %d alias(es) (%d simple, %d with templates) to %s\n",
				legacy+rich, legacy, rich, outPath)
			return nil
		},
	}
	cmd.Flags().StringVar(&outPath, "out", "", "Output path; '-' or empty writes to stdout")
	return cmd
}

// NewCmdAliasImport reads an alias bundle (produced by 'alias export') and
// merges it into the local config. Existing aliases are preserved unless
// --force is passed.
func NewCmdAliasImport() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import aliases from a YAML bundle produced by 'alias export'",
		Long: `Merge aliases from a bundle file into the local config. Pass '-' as the
path to read from stdin (pairs with 'export' piping).

By default, aliases already present locally are preserved. Pass --force to
overwrite them with the bundle's version.`,
		Example: `  # Import from a file
  kubectl cwide alias import my-aliases.yaml

  # Import from stdin
  kubectl cwide alias export | kubectl cwide alias import -

  # Force overwrite on conflict
  kubectl cwide alias import my-aliases.yaml --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var raw []byte
			var err error
			if args[0] == "-" {
				raw, err = readAll(cmd.InOrStdin())
			} else {
				raw, err = os.ReadFile(args[0])
			}
			if err != nil {
				return fmt.Errorf("read bundle: %w", err)
			}

			var b aliasBundle
			if err := yaml.Unmarshal(raw, &b); err != nil {
				return fmt.Errorf("parse bundle: %w", err)
			}
			if b.Version == 0 {
				// Very old / hand-authored bundles won't stamp a version.
				// Accept it but warn — the fields still map through.
				fmt.Fprintln(cmd.ErrOrStderr(), "Warning: bundle has no version stamp; parsed as version 1")
			} else if b.Version > aliasBundleVersion {
				return fmt.Errorf("bundle version %d is newer than this build supports (%d); upgrade kubectl-cwide", b.Version, aliasBundleVersion)
			}

			cfg, err := utils.LoadConfig()
			if err != nil {
				return fmt.Errorf("load local config: %w", err)
			}
			if cfg.Aliases == nil {
				cfg.Aliases = map[string]string{}
			}
			if cfg.AliasEntries == nil {
				cfg.AliasEntries = map[string]models.AliasEntry{}
			}

			var added, kept, replaced int
			imported := make([]string, 0, len(b.Aliases)+len(b.AliasEntries))
			for name := range b.Aliases {
				imported = append(imported, name)
			}
			for name := range b.AliasEntries {
				imported = append(imported, name)
			}
			sort.Strings(imported)

			for _, name := range imported {
				// If the imported name is a rich entry it wins over any
				// legacy entry of the same name in the same bundle.
				var newRich *models.AliasEntry
				if e, ok := b.AliasEntries[name]; ok {
					newRich = &e
				}
				newLegacy, hasLegacy := b.Aliases[name]

				// Determine current local state.
				_, localLegacy := cfg.Aliases[name]
				_, localRich := cfg.AliasEntries[name]
				exists := localLegacy || localRich

				if exists && !force {
					kept++
					continue
				}

				// Wipe both sides so the imported form is unambiguous.
				delete(cfg.Aliases, name)
				delete(cfg.AliasEntries, name)

				if newRich != nil {
					cfg.AliasEntries[name] = *newRich
				} else if hasLegacy {
					cfg.Aliases[name] = newLegacy
				}

				if exists {
					replaced++
				} else {
					added++
				}
			}

			if err := utils.SaveConfig(cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"Imported: %d added, %d replaced, %d kept (use --force to overwrite existing).\n",
				added, replaced, kept)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing local aliases on conflict")
	return cmd
}
