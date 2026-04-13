package list

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
)

func NewCmdList(streams genericiooptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:        "list",
		Aliases:    []string{"ls"},
		SuggestFor: []string{"resources", "api-resources"},
		Short:      "List API resources in the cluster",
		Long: `Discover and display Kubernetes API resources available in the cluster.

Use 'all' to list all API resources, filtered by scope (namespaced or cluster-scoped).`,
		RunE: func(c *cobra.Command, args []string) error {
			return c.Help()
		},
	}

	cmd.AddCommand(NewCmdListAll(streams))

	return cmd
}
