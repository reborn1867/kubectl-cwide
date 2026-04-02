package marketplace

import "github.com/spf13/cobra"

func NewCmdMarketplace() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "marketplace",
		Short: "Browse and install community templates",
		Long: `Discover and install column templates shared by the community.

Templates are fetched from a GitHub repository (default: reborn1867/kubectl-cwide-templates).
Use 'list' to browse available resource types, 'search' to find templates for a
specific resource, and 'install' to download a template into your local template directory.`,
		RunE: func(c *cobra.Command, args []string) error {
			return c.Help()
		},
	}

	cmd.PersistentFlags().String("repo", defaultRepo, "GitHub repository to fetch templates from (owner/repo)")

	cmd.AddCommand(NewCmdList())
	cmd.AddCommand(NewCmdSearch())
	cmd.AddCommand(NewCmdInstall())

	return cmd
}
