package alias

import (
	"github.com/spf13/cobra"
)

func NewCmdAlias() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "alias",
		Aliases:    []string{"al"},
		SuggestFor: []string{"aliases", "shortname"},
		Short:      "Manage custom resource type aliases",
		Long: `Create, list, and delete custom short aliases for Kubernetes resource types.

Aliases are saved in ~/.kubectl-cwide/config.yaml and automatically resolved
when used in 'get' and 'tree' commands. For example, after setting 'pd' as an
alias for 'pods', you can run 'kubectl cwide get pd' instead of 'kubectl cwide get pods'.`,
		RunE: func(c *cobra.Command, args []string) error {
			return c.Help()
		},
	}

	cmd.AddCommand(NewCmdAliasSet())
	cmd.AddCommand(NewCmdAliasList())
	cmd.AddCommand(NewCmdAliasDelete())

	return cmd
}
