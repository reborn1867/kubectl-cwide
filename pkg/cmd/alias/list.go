package alias

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/yaml"
)

func NewCmdAliasList() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:        "list",
		Aliases:    []string{"ls"},
		SuggestFor: []string{"show"},
		Short:      "List all configured resource type aliases",
		Example: `  # Table output (default)
  kubectl cwide alias list

  # YAML output
  kubectl cwide alias list -o yaml

  # JSON output
  kubectl cwide alias list -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := utils.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config (run 'init' first): %w", err)
			}

			if len(config.Aliases) == 0 && output == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "No aliases configured.")
				return nil
			}

			// Sort aliases for deterministic output.
			aliases := make([]string, 0, len(config.Aliases))
			for alias := range config.Aliases {
				aliases = append(aliases, alias)
			}
			sort.Strings(aliases)

			// Rebuild a sorted map for structured formats — Go's yaml/json
			// marshalers sort keys alphabetically, so passing config.Aliases
			// directly is fine, but we accept both for clarity.
			out := cmd.OutOrStdout()
			switch output {
			case "yaml":
				data, err := yaml.Marshal(config.Aliases)
				if err != nil {
					return fmt.Errorf("marshal aliases as yaml: %w", err)
				}
				_, err = out.Write(data)
				return err
			case "json":
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(config.Aliases)
			case "", "table":
				w := printers.GetNewTabWriter(out)
				defer w.Flush()
				fmt.Fprintln(w, "ALIAS\tRESOURCE")
				for _, alias := range aliases {
					fmt.Fprintf(w, "%s\t%s\n", alias, config.Aliases[alias])
				}
				return nil
			default:
				return fmt.Errorf("unsupported output format %q (expected yaml, json, or table)", output)
			}
		},
	}

	cmd.Flags().StringVarP(&output, "output", "o", "", "Output format. One of: yaml, json, table. Defaults to table.")
	_ = cmd.RegisterFlagCompletionFunc("output", cobra.FixedCompletions([]string{"yaml", "json", "table"}, cobra.ShellCompDirectiveNoFileComp))

	return cmd
}
