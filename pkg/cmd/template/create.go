package template

import (
	"fmt"
	"path/filepath"

	"github.com/kubectl-cwide/pkg/cmd/completions"
	"github.com/kubectl-cwide/pkg/models"
	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func NewCmdCreate() *cobra.Command {
	createCMD := &cobra.Command{
		Use:        "create",
		Aliases:    []string{"new"},
		SuggestFor: []string{"add"},
		Short:      "Create a new column template",
		Long: `Scaffold a new YAML column template for the specified resource type.

The template is created with a single NAME column as a starting point.
Edit the generated file to add more columns.`,
		Example: `  # Create a template called "custom" for pods
  kubectl cwide template create -r pod -n custom

  # Create a template in a specific directory
  kubectl cwide template create -r deployment -n minimal --template-path ~/my-templates`,
		RunE: func(cmd *cobra.Command, args []string) error {
			absPath, err := utils.ResolveTemplatePath(cmd)
			if err != nil {
				return fmt.Errorf("failed to resolve template path: %w", err)
			}

			resourceType := cmd.Flag("resource").Value.String()

			pattern := filepath.Join(absPath, fmt.Sprintf("%s-*", resourceType))
			files, err := filepath.Glob(pattern)
			if err != nil {
				return fmt.Errorf("failed to search for resource directories: %w", err)
			}

			if len(files) == 0 {
				return fmt.Errorf("no resource directory found for %q; run 'init' first", resourceType)
			}

			if len(files) != 1 {
				return fmt.Errorf("found multiple directories for %q: %v; specify a more precise resource type", resourceType, files)
			}

			name := cmd.Flag("name").Value.String()
			newFilePath := filepath.Join(files[0], fmt.Sprintf("%s.yaml", name))

			if utils.CheckFileExists(newFilePath) {
				return fmt.Errorf("template %q already exists at %s", name, newFilePath)
			}

			scaffold, err := yaml.Marshal(&models.YAMLTemplate{
				Columns: []models.YAMLColumn{
					{Header: "NAME", FieldSpec: ".metadata.name"},
				},
			})
			if err != nil {
				return fmt.Errorf("failed to marshal template scaffold: %w", err)
			}

			if err := utils.CreateFileIfNotExists(newFilePath, scaffold); err != nil {
				return fmt.Errorf("failed to create template file: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created template: %s\n", newFilePath)

			return nil
		},
	}

	createCMD.Flags().StringP("name", "n", "", "Name for the new template (without extension)")
	createCMD.Flags().StringP("resource", "r", "", "Resource type to create the template for (e.g. pod, deployment)")
	_ = createCMD.RegisterFlagCompletionFunc("resource", completions.ResourceTypes)
	createCMD.MarkFlagRequired("resource")
	createCMD.MarkFlagRequired("name")

	return createCMD
}
