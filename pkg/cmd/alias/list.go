package alias

import (
	"fmt"
	"sort"

	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/printers"
)

func NewCmdAliasList() *cobra.Command {
	return &cobra.Command{
		Use:        "list",
		Aliases:    []string{"ls"},
		SuggestFor: []string{"show"},
		Short:      "List all configured resource type aliases",
		Example: `  # List all aliases
  kubectl cwide alias list`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := utils.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config (run 'init' first): %w", err)
			}

			if len(config.Aliases) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No aliases configured.")
				return nil
			}

			// Sort aliases for deterministic output
			aliases := make([]string, 0, len(config.Aliases))
			for alias := range config.Aliases {
				aliases = append(aliases, alias)
			}
			sort.Strings(aliases)

			w := printers.GetNewTabWriter(cmd.OutOrStdout())
			defer w.Flush()

			fmt.Fprintln(w, "ALIAS\tRESOURCE")
			for _, alias := range aliases {
				fmt.Fprintf(w, "%s\t%s\n", alias, config.Aliases[alias])
			}

			return nil
		},
	}
}
