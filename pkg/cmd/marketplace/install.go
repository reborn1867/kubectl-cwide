package marketplace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kubectl-cwide/pkg/models"
	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func NewCmdInstall() *cobra.Command {
	var force bool

	installCMD := &cobra.Command{
		Use:        "install",
		Aliases:    []string{"add"},
		SuggestFor: []string{"download"},
		Short:      "Install a template from the marketplace",
		Long: `Download a community template and save it to the local template directory.

The template is fetched from the remote GitHub repository and saved under the
matching resource directory. Existing files are not overwritten unless --force
is specified.`,
		Example: `  # Install the "debug" template for pods
  kubectl cwide marketplace install -r pod -t debug

  # Overwrite an existing template
  kubectl cwide marketplace install -r pod -t debug --force

  # Install from a custom repository
  kubectl cwide marketplace install -r pod -t debug --repo myorg/my-templates`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := cmd.Flag("repo").Value.String()
			resource := cmd.Flag("resource").Value.String()
			templateName := cmd.Flag("template").Value.String()
			ref := cmd.Flag("ref").Value.String()

			// Resolve the remote directory for this resource
			entries, err := listContentsAt(repo, basePath, ref)
			if err != nil {
				return fmt.Errorf("failed to list marketplace: %w", err)
			}

			prefix := strings.ToLower(resource) + "-"
			var matchedDirs []string
			for _, e := range entries {
				if e.Type == "dir" && strings.HasPrefix(e.Name, prefix) {
					matchedDirs = append(matchedDirs, e.Name)
				}
			}

			if len(matchedDirs) == 0 {
				return fmt.Errorf("no resource directory found for %q in the marketplace", resource)
			}
			if len(matchedDirs) > 1 {
				return fmt.Errorf("multiple directories match %q: %v; specify a more precise resource type", resource, matchedDirs)
			}

			remoteDir := matchedDirs[0]
			fileName := templateName + ".yaml"

			// Find the file in the remote directory
			files, err := listContentsAt(repo, basePath+"/"+remoteDir, ref)
			if err != nil {
				return fmt.Errorf("failed to list templates in %s: %w", remoteDir, err)
			}

			var downloadURL string
			for _, f := range files {
				if f.Name == fileName {
					downloadURL = f.DownloadURL
					break
				}
			}
			if downloadURL == "" {
				return fmt.Errorf("template %q not found in %s", templateName, remoteDir)
			}

			// Resolve local template path
			absPath, err := utils.ResolveTemplatePath(cmd)
			if err != nil {
				return fmt.Errorf("failed to resolve template path: %w", err)
			}

			localDir := filepath.Join(absPath, remoteDir)
			localPath := filepath.Join(localDir, fileName)

			if !force && utils.CheckFileExists(localPath) {
				return fmt.Errorf("template already exists at %s; use --force to overwrite", localPath)
			}

			// Download the template
			data, err := downloadFile(downloadURL)
			if err != nil {
				return fmt.Errorf("failed to download template: %w", err)
			}

			// Stamp provenance metadata into the YAML so 'template list' and
			// future 'template diff' commands can tell where each template
			// came from and at what version. Best-effort: if the download
			// isn't a parseable YAML template, we skip stamping and write it
			// through unchanged.
			data = stampMarketplaceMetadata(data, repo, ref)

			// Ensure the directory exists and write the file
			if err := os.MkdirAll(localDir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", localDir, err)
			}
			if err := os.WriteFile(localPath, data, 0644); err != nil {
				return fmt.Errorf("failed to write template: %w", err)
			}

			// Record the pin (best-effort; a failure here doesn't fail the install).
			if lf, err := LoadLockFile(); err == nil {
				lf.Upsert(MarketplacePin{
					Repo: repo, Resource: resource, Template: templateName, Ref: ref,
				})
				_ = lf.Save()
			}

			if ref != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Installed template: %s (pinned to %s)\n", localPath, ref)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Installed template: %s\n", localPath)
			}
			return nil
		},
	}

	installCMD.Flags().StringP("resource", "r", "", "Resource type (e.g. pod, deployment)")
	installCMD.Flags().StringP("template", "t", "", "Template name to install (without extension)")
	installCMD.Flags().String("ref", "", "Pin to a specific git ref (branch, tag, or commit SHA). Recorded in ~/.kubectl-cwide/marketplace.lock.")
	installCMD.Flags().BoolVar(&force, "force", false, "Overwrite existing template file")
	installCMD.MarkFlagRequired("resource")
	installCMD.MarkFlagRequired("template")

	return installCMD
}

// stampMarketplaceMetadata rewrites the provenance block on a downloaded
// template's YAML so 'template list' and future 'template diff' commands can
// tell where each file came from. Best-effort: on any parse failure the
// input is returned unchanged, so an unparseable download (or a .tpl file
// masquerading as .yaml) still writes cleanly.
//
// If the template already had a Metadata block (e.g. the upstream author
// baked one in), we OVERWRITE Source/SourceRepo/SourceRef/InstalledAt
// because those describe the CURRENT install action, not the upstream
// state — but we preserve their Version if they set one, since that is
// the marketplace author's declared version and outranks a ref that
// could be a mutable branch.
func stampMarketplaceMetadata(data []byte, repo, ref string) []byte {
	var tmpl models.YAMLTemplate
	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		return data
	}
	if len(tmpl.Columns) == 0 {
		// Not a template shape — leave alone.
		return data
	}

	preservedVersion := ""
	if tmpl.Metadata != nil {
		preservedVersion = tmpl.Metadata.Version
	}
	tmpl.Metadata = &models.TemplateMetadata{
		Source:      "marketplace",
		SourceRepo:  repo,
		SourceRef:   ref,
		Version:     preservedVersion,
		InstalledAt: time.Now().UTC().Format(time.RFC3339),
	}
	// Default Version to the ref when the upstream didn't declare one.
	// Refs that look like tags/versions are usually what users mean.
	if tmpl.Metadata.Version == "" {
		tmpl.Metadata.Version = ref
	}

	out, err := yaml.Marshal(&tmpl)
	if err != nil {
		return data
	}
	return out
}
