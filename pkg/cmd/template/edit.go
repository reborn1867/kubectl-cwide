package template

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
)

func NewCmdEdit() *cobra.Command {
	editCMD := &cobra.Command{
		Use:   "edit",
		Short: "Open a template file in an editor",
		Long: `Open the column template for the specified resource type in your preferred
editor. The editor is determined by the EDITOR environment variable, falling
back to vi.

By default the "default" template is opened. Use -t to specify a different
template name.`,
		Example: `  # Edit the default template for pods
  kubectl cwide template edit -r pod

  # Edit a specific named template
  kubectl cwide template edit -r deployment -t minimal

  # Edit from a specific directory
  kubectl cwide template edit -r pod --template-path ~/my-templates`,
		RunE: func(cmd *cobra.Command, args []string) error {
			absPath, err := utils.ResolveTemplatePath(cmd)
			if err != nil {
				return fmt.Errorf("failed to resolve template path: %w", err)
			}

			resourceType := cmd.Flag("resource").Value.String()
			templateName := cmd.Flag("template").Value.String()

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

			// Try .yaml first, then .tpl
			yamlPath := filepath.Join(files[0], templateName+".yaml")
			tplPath := filepath.Join(files[0], templateName+".tpl")

			var targetPath string
			switch {
			case fileExists(yamlPath):
				targetPath = yamlPath
			case fileExists(tplPath):
				targetPath = tplPath
			default:
				return fmt.Errorf("template %q not found (tried %s and %s)", templateName, yamlPath, tplPath)
			}

			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}

			editorCmd := exec.Command(editor, targetPath)
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr

			if err := editorCmd.Run(); err != nil {
				return fmt.Errorf("editor exited with error: %w", err)
			}
			return nil
		},
	}

	editCMD.Flags().StringP("resource", "r", "", "Resource type to edit the template for (e.g. pod, deployment)")
	editCMD.Flags().StringP("template", "t", "default", "Name of the template to edit (without extension)")
	editCMD.MarkFlagRequired("resource")

	return editCMD
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
