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
		Short: "cwide template list",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			pattern := filepath.Join(absPath, fmt.Sprintf("%s-*/*.tpl", resourceType))
			// list all files in the directory
			files, err := filepath.Glob(pattern)
			if err != nil {
				return fmt.Errorf("failed to find template path: %v", err)
			}

			if len(files) == 0 {
				return fmt.Errorf("no template found for resource type: %s", resourceType)
			}

			for _, file := range files {
				fmt.Println(strings.Split(filepath.Base(file), ".")[0])
			}

			return nil
		},
	}

	templateCMD.Flags().StringP("resource", "r", "", "resource type of custom column templates to list")
	templateCMD.MarkFlagRequired("resource")

	return templateCMD
}
