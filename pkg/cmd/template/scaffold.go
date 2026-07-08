package template

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// NewCmdScaffold creates a stub YAML template with common columns filled in
// and the rest commented out for the user to enable.
func NewCmdScaffold() *cobra.Command {
	return &cobra.Command{
		Use:   "scaffold <resource>",
		Short: "Print a starter template for a resource type",
		Long: `Emit a YAML template body with a handful of common columns (name, namespace,
age) filled in, plus a commented-out set of frequently-useful JSONPath
expressions you can uncomment.

The output is written to stdout — pipe it to the destination path yourself.
This is intended as a "first draft" — cwide's 'init' auto-generates a more
complete template by inspecting the CRD schema.`,
		Example: `  # Pipe to a template file
  kubectl cwide template scaffold pod > ~/.kubectl-cwide/templates/pod--v1/starter.yaml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := scaffoldFor(args[0])
			_, err := fmt.Fprint(cmd.OutOrStdout(), body)
			return err
		},
	}
}

func scaffoldFor(kind string) string {
	kind = strings.ToLower(kind)
	var b strings.Builder
	fmt.Fprintf(&b, "# Starter template for %s. Uncomment or edit columns as needed.\n", kind)
	b.WriteString("columns:\n")
	b.WriteString("  - header: NAMESPACE\n    fieldSpec: .metadata.namespace\n")
	b.WriteString("  - header: NAME\n    fieldSpec: .metadata.name\n")
	b.WriteString("  - header: AGE\n    fieldSpec: .metadata.creationTimestamp\n")
	b.WriteString("\n")
	b.WriteString("  # --- commonly useful fields; uncomment to include ---\n")
	b.WriteString("  # - header: STATUS\n  #   fieldSpec: .status.phase\n")
	b.WriteString("  # - header: LABELS\n  #   template: '{{ range $k, $v := .metadata.labels }}{{$k}}={{$v}} {{ end }}'\n")
	b.WriteString("  # - header: OWNER\n  #   fieldSpec: .metadata.ownerReferences[0].name\n")
	b.WriteString("  # - header: RESOURCE_VERSION\n  #   fieldSpec: .metadata.resourceVersion\n")
	b.WriteString("  # - header: FINALIZERS\n  #   template: '{{ range .metadata.finalizers }}{{ . }} {{ end }}'\n")
	return b.String()
}
