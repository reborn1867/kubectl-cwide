package template

import "github.com/spf13/cobra"

func NewCmdTemplate() *cobra.Command {
	templateCMD := &cobra.Command{
		Use:   "template",
		Short: "Manage column templates",
		Long: `Create and list column templates used by 'get'.

Templates are stored in the template root directory (set via --template-path or
the config file created by 'init').`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	templateCMD.AddCommand(NewCmdTemplateList())
	templateCMD.AddCommand(NewCmdCreate())

	return templateCMD
}
