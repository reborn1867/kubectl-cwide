package marketplace

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func NewCmdSearch() *cobra.Command {
	searchCMD := &cobra.Command{
		Use:   "search",
		Short: "Search for templates by resource type",
		Long: `Search the marketplace for templates matching a specific resource type.

The resource flag is matched as a prefix against directory names in the remote
repository. For each matching resource, all available template files are listed.`,
		Example: `  # Search for pod templates
  kubectl cwide marketplace search -r pod

  # Search for deployment templates
  kubectl cwide marketplace search -r deployment`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := cmd.Flag("repo").Value.String()
			resource := cmd.Flag("resource").Value.String()

			entries, err := listContents(repo, basePath)
			if err != nil {
				return fmt.Errorf("failed to list marketplace: %w", err)
			}

			prefix := strings.ToLower(resource) + "-"
			found := false

			for _, e := range entries {
				if e.Type != "dir" || !strings.HasPrefix(e.Name, prefix) {
					continue
				}

				files, err := listContents(repo, basePath+"/"+e.Name)
				if err != nil {
					return fmt.Errorf("failed to list templates in %s: %w", e.Name, err)
				}

				for _, f := range files {
					if f.Type == "file" && strings.HasSuffix(f.Name, ".yaml") {
						name := strings.TrimSuffix(f.Name, filepath.Ext(f.Name))
						fmt.Fprintf(cmd.OutOrStdout(), "%s/%s\n", e.Name, name)
						found = true
					}
				}
			}

			if !found {
				return fmt.Errorf("no templates found for resource type %q", resource)
			}

			return nil
		},
	}

	searchCMD.Flags().StringP("resource", "r", "", "Resource type to search for (e.g. pod, deployment)")
	searchCMD.MarkFlagRequired("resource")

	return searchCMD
}
