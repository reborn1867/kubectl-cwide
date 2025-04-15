package template

import (
	"fmt"
	"path/filepath"

	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
)

func NewCmdCreate() *cobra.Command {
	createCMD := &cobra.Command{
		Use:   "create",
		Short: "cwide template create",
		RunE: func(cmd *cobra.Command, args []string) error {
			// create a new template file in given path
			var path string
			if cmd.Flag("template-path").Changed {
				path = cmd.Flag("template-path").Value.String()
			} else {
				var err error
				// get template path from config.yaml
				path, err = utils.GetTemplatePathFromConfig()
				if err != nil {
					return err
				}
			}

			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("failed to get absolute path: %v", err)
			}

			resourceType := cmd.Flag("resource").Value.String()

			// TODO: need a better way to get the template path
			pattern := filepath.Join(absPath, fmt.Sprintf("%s-*", resourceType))
			// list all files in the directory
			files, err := filepath.Glob(pattern)
			if err != nil {
				return fmt.Errorf("failed to find template path: %v", err)
			}

			if len(files) == 0 {
				return fmt.Errorf("no template found for resource type: %s", resourceType)
			}

			if len(files) != 1 {
				return fmt.Errorf("found multiple templates for resource type: %s", resourceType)
			}

			// get the template name from the command line
			name := cmd.Flag("name").Value.String()

			newFilePath := filepath.Join(files[0], fmt.Sprintf("%s.tpl", name))

			// create a new template file
			if err := utils.CreateFileIfNotExits(newFilePath, []byte("")); err != nil {
				return fmt.Errorf("failed to create template file: %v", err)
			}
			fmt.Printf("created template file: %s\n", newFilePath)

			return nil
		},
	}

	createCMD.Flags().StringP("name", "n", "", "name of the template to create")
	createCMD.Flags().StringP("resource", "r", "", "resource type of custom column templates to create")
	createCMD.MarkFlagRequired("resource")
	createCMD.MarkFlagRequired("name")

	return createCMD
}
