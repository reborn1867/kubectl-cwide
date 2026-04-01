package template

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
)

func NewCmdTemplateList() *cobra.Command {
	templateCMD := &cobra.Command{
		Use:   "list",
		Short: "List available templates for a resource type",
		Long: `List all column templates available for the specified resource type.

Templates are discovered from both .yaml and .tpl files in the template
directory. Duplicates (same name, different extension) are shown once.`,
		Example: `  # List all templates for pods
  kubectl cwide template list -r pod

  # List templates from a specific directory
  kubectl cwide template list -r deployment --template-path ~/my-templates`,
		RunE: func(cmd *cobra.Command, args []string) error {
			absPath, err := utils.ResolveTemplatePath(cmd)
			if err != nil {
				return fmt.Errorf("failed to resolve template path: %w", err)
			}

			resourceType := cmd.Flag("resource").Value.String()

			// list templates from both .yaml and .tpl files, deduplicating by name
			seen := make(map[string]bool)
			for _, ext := range []string{"*.yaml", "*.tpl"} {
				pattern := filepath.Join(absPath, fmt.Sprintf("%s-*/%s", resourceType, ext))
				files, err := filepath.Glob(pattern)
				if err != nil {
					return fmt.Errorf("failed to search for templates: %w", err)
				}
				for _, file := range files {
					name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
					if !seen[name] {
						seen[name] = true
						fmt.Println(name)
					}
				}
			}

			if len(seen) == 0 {
				return fmt.Errorf("no templates found for resource type: %s", resourceType)
			}

			return nil
		},
	}

	templateCMD.Flags().StringP("resource", "r", "", "Resource type to list templates for (e.g. pod, deployment)")
	templateCMD.MarkFlagRequired("resource")

	return templateCMD
}
