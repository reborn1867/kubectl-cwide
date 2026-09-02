package template

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kubectl-cwide/pkg/cmd/completions"
	"github.com/kubectl-cwide/pkg/models"
	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"k8s.io/cli-runtime/pkg/printers"
)

func NewCmdTemplateList() *cobra.Command {
	var output string

	templateCMD := &cobra.Command{
		Use:        "list",
		Aliases:    []string{"ls"},
		SuggestFor: []string{"show"},
		Short:      "List available templates for a resource type",
		Long: `List all column templates available for the specified resource type.

Templates are discovered from both .yaml and .tpl files in the template
directory. Duplicates (same name, different extension) are shown once.

Default output is one template name per line for shell-scripting friendliness.
With -o wide, prints a table including any provenance metadata written by
'marketplace install' or hand-authored: VERSION, SOURCE, REF, INSTALLED.`,
		Example: `  # List all templates for pods
  kubectl cwide template list -r pod

  # Include version + source in a wide table
  kubectl cwide template list -r pod -o wide`,
		RunE: func(cmd *cobra.Command, args []string) error {
			absPath, err := utils.ResolveTemplatePath(cmd)
			if err != nil {
				return fmt.Errorf("failed to resolve template path: %w", err)
			}

			resourceType := cmd.Flag("resource").Value.String()

			// Discover template files under any <resource>-*/ dir. Same
			// glob pattern the original bare-list used, so behavior is
			// preserved for callers that don't pass -o wide.
			type entry struct {
				name string
				meta *models.TemplateMetadata
				path string // the winning file, for callers who want it
			}
			byName := map[string]*entry{}
			for _, ext := range []string{"*.yaml", "*.tpl"} {
				pattern := filepath.Join(absPath, fmt.Sprintf("%s-*/%s", resourceType, ext))
				files, err := filepath.Glob(pattern)
				if err != nil {
					return fmt.Errorf("failed to search for templates: %w", err)
				}
				for _, file := range files {
					name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
					if _, seen := byName[name]; seen {
						// First-seen wins — .yaml is enumerated before
						// .tpl, matching the resolver's precedence.
						continue
					}
					byName[name] = &entry{
						name: name,
						path: file,
						meta: readTemplateMetadata(file),
					}
				}
			}

			if len(byName) == 0 {
				return fmt.Errorf("no templates found for resource type: %s", resourceType)
			}

			// Deterministic ordering — old behavior printed in glob order
			// (usually alphabetical on most filesystems), but relying on
			// that is fragile. Sort explicitly.
			names := make([]string, 0, len(byName))
			for n := range byName {
				names = append(names, n)
			}
			sort.Strings(names)

			switch output {
			case "", "name":
				for _, n := range names {
					fmt.Fprintln(cmd.OutOrStdout(), n)
				}
				return nil
			case "wide":
				// falls through to the tab-writer block below
			default:
				return fmt.Errorf("unsupported -o value %q (expected: wide, name, or empty)", output)
			}

			w := printers.GetNewTabWriter(cmd.OutOrStdout())
			defer w.Flush()
			fmt.Fprintln(w, "NAME\tVERSION\tSOURCE\tREF\tINSTALLED")
			for _, n := range names {
				e := byName[n]
				ver, src, ref, when := "-", "-", "-", "-"
				if e.meta != nil {
					if e.meta.Version != "" {
						ver = e.meta.Version
					}
					if e.meta.Source != "" {
						src = e.meta.Source
					}
					if e.meta.SourceRef != "" {
						ref = e.meta.SourceRef
					}
					if e.meta.InstalledAt != "" {
						when = e.meta.InstalledAt
					}
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", n, ver, src, ref, when)
			}
			return nil
		},
	}

	templateCMD.Flags().StringP("resource", "r", "", "Resource type to list templates for (e.g. pod, deployment)")
	_ = templateCMD.RegisterFlagCompletionFunc("resource", completions.ResourceTypes)
	templateCMD.Flags().StringVarP(&output, "output", "o", "", "Output format. One of: name (default), wide")
	_ = templateCMD.RegisterFlagCompletionFunc("output", cobra.FixedCompletions([]string{"name", "wide"}, cobra.ShellCompDirectiveNoFileComp))
	templateCMD.MarkFlagRequired("resource")

	return templateCMD
}

// readTemplateMetadata returns the Metadata block of a .yaml template if
// present. Any parse failure returns nil so 'template list' stays robust
// against hand-authored / non-cwide YAML in the tree.
func readTemplateMetadata(path string) *models.TemplateMetadata {
	if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
		return nil // .tpl files carry no metadata
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var tmpl models.YAMLTemplate
	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		return nil
	}
	return tmpl.Metadata
}
