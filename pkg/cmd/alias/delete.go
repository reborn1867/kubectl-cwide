package alias

import (
	"fmt"

	"github.com/kubectl-cwide/pkg/cmd/completions"
	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
)

func NewCmdAliasDelete() *cobra.Command {
	return &cobra.Command{
		Use:        "delete ALIAS",
		Aliases:    []string{"rm"},
		SuggestFor: []string{"remove", "unset"},
		Short:      "Delete a resource type alias",
		Example: `  # Delete the 'pd' alias
  kubectl cwide alias delete pd`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completions.AliasNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := args[0]

			config, err := utils.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config (run 'init' first): %w", err)
			}

			if _, ok := config.Aliases[alias]; !ok {
				return fmt.Errorf("alias %q not found", alias)
			}

			delete(config.Aliases, alias)

			if err := utils.SaveConfig(config); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Alias %q deleted\n", alias)
			return nil
		},
	}
}
