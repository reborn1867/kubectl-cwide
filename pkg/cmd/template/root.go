package template

import "github.com/spf13/cobra"

func NewCmdTemplate() *cobra.Command {
	templateCMD := &cobra.Command{
		Use:   "template",
		Short: "cwide template",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	templateCMD.AddCommand(NewCmdTemplateList())
	templateCMD.AddCommand(NewCmdCreate())

	return templateCMD
}
