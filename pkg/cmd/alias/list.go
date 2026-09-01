package alias

import (
	"fmt"
	"sort"
	"strings"

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

			// Merge legacy Aliases + rich AliasEntries for display. If the
			// same key appears in both, AliasEntries wins (matches
			// ResolveAliasTarget semantics).
			names := map[string]bool{}
			for n := range config.Aliases {
				names[n] = true
			}
			for n := range config.AliasEntries {
				names[n] = true
			}
			if len(names) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No aliases configured.")
				return nil
			}
			sorted := make([]string, 0, len(names))
			for n := range names {
				sorted = append(sorted, n)
			}
			sort.Strings(sorted)

			w := printers.GetNewTabWriter(cmd.OutOrStdout())
			defer w.Flush()

			fmt.Fprintln(w, "ALIAS\tRESOURCE\tTEMPLATE")
			for _, alias := range sorted {
				resource := config.ResolveAliasTarget(alias)
				tplDesc := ""
				if e, ok := config.AliasEntries[alias]; ok {
					switch {
					case len(e.Templates) > 0:
						parts := make([]string, 0, len(e.Templates))
						for k, v := range e.Templates {
							parts = append(parts, k+"="+v)
						}
						sort.Strings(parts)
						tplDesc = strings.Join(parts, ",")
					case e.Template != "":
						tplDesc = e.Template
					}
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", alias, resource, tplDesc)
			}

			return nil
		},
	}
}
