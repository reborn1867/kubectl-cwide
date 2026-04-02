package marketplace

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewCmdList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available resource types in the marketplace",
		Long: `List all resource types that have community templates available.

Each entry corresponds to a resource directory (e.g. pod--v1, deployment-apps-v1)
in the remote template repository.`,
		Example: `  # List all available resource types
  kubectl cwide marketplace list

  # List from a custom repository
  kubectl cwide marketplace list --repo myorg/my-templates`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := cmd.Flag("repo").Value.String()

			entries, err := listContents(repo, basePath)
			if err != nil {
				return fmt.Errorf("failed to list marketplace: %w", err)
			}

			count := 0
			for _, e := range entries {
				if e.Type == "dir" {
					fmt.Fprintln(cmd.OutOrStdout(), e.Name)
					count++
				}
			}

			if count == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No resource types found in the marketplace.")
			}

			return nil
		},
	}
}
