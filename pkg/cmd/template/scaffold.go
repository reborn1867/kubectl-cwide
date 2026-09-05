package template

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/kubectl-cwide/pkg/cmd/completions"
	"github.com/spf13/cobra"
)

// scaffoldFS bundles curated starter recipes. Layout is
// assets/scaffolds/<kind>/<recipe>.yaml; kind is lowercase singular so
// user-typed 'pod' / 'Pod' both resolve. Each file is a fully-formed
// YAMLTemplate that 'template lint' should accept as-is.
//
//go:embed all:assets/scaffolds
var scaffoldFS embed.FS

const scaffoldFSRoot = "assets/scaffolds"

// NewCmdScaffold creates a stub YAML template with common columns filled in
// and the rest commented out for the user to enable. With --from <recipe>,
// prints a bundled cookbook recipe instead.
func NewCmdScaffold() *cobra.Command {
	var from string
	var list bool

	cmd := &cobra.Command{
		Use:   "scaffold <resource>",
		Short: "Print a starter template for a resource type",
		Long: `Emit a YAML template for the given resource.

Default output is a stub with the common columns (NAME, NAMESPACE, AGE)
filled in and a set of frequently-useful JSONPath expressions commented out
for the user to enable.

With --from <recipe>, emit a bundled cookbook recipe for the resource
instead. Recipes are shipped inside the binary and don't require network
or cluster access. Use --list to see all available bundled recipes.

Output goes to stdout — pipe it to the destination path yourself. This is
intended as a "first draft" — cwide's 'init' auto-generates a more complete
template by inspecting the CRD schema.`,
		Example: `  # Bare stub for a resource
  kubectl cwide template scaffold pod > ~/.kubectl-cwide/templates/pod--v1/starter.yaml

  # Cookbook recipe: pod restart reasons
  kubectl cwide template scaffold pod --from restart-reason > ~/.kubectl-cwide/templates/pod--v1/restart-reason.yaml

  # List every bundled recipe
  kubectl cwide template scaffold --list`,
		Args: cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if list {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return completions.ResourceTypes(cmd, args, toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if list {
				return listScaffolds(cmd)
			}
			if len(args) == 0 {
				return fmt.Errorf("resource argument required (use --list to see bundled recipes)")
			}
			kind := strings.ToLower(args[0])
			if from != "" {
				return emitBundledScaffold(cmd, kind, from)
			}
			_, err := fmt.Fprint(cmd.OutOrStdout(), scaffoldFor(kind))
			return err
		},
	}

	cmd.Flags().StringVar(&from, "from", "",
		"Emit a bundled cookbook recipe by name instead of the default stub (see --list)")
	cmd.Flags().BoolVar(&list, "list", false,
		"List every bundled scaffold recipe and exit")
	_ = cmd.RegisterFlagCompletionFunc("from", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return recipeNamesAcrossAllKinds(), cobra.ShellCompDirectiveNoFileComp
		}
		return recipesForKind(strings.ToLower(args[0])), cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

// emitBundledScaffold writes the given recipe for the given kind to stdout.
// Kind must match a directory under assets/scaffolds/ (lowercase singular);
// recipe is the filename basename without .yaml. On miss, the error lists
// every recipe available for that kind so users don't have to run --list.
func emitBundledScaffold(cmd *cobra.Command, kind, recipe string) error {
	name := recipe + ".yaml"
	full := path.Join(scaffoldFSRoot, kind, name)
	data, err := scaffoldFS.ReadFile(full)
	if err != nil {
		available := recipesForKind(kind)
		if len(available) == 0 {
			return fmt.Errorf("no bundled scaffolds for %q; run 'kubectl cwide template scaffold --list' to see supported kinds", kind)
		}
		return fmt.Errorf("no bundled scaffold %q for kind %q; available: %s",
			recipe, kind, strings.Join(available, ", "))
	}
	_, werr := cmd.OutOrStdout().Write(data)
	return werr
}

// listScaffolds prints every bundled recipe grouped by kind. Output shape is
// deliberately shell-friendly — one recipe per line as "kind/recipe" — so
// users can `... --list | grep pod` and pipe into a for-loop.
func listScaffolds(cmd *cobra.Command) error {
	kinds, err := listKinds()
	if err != nil {
		return err
	}
	for _, kind := range kinds {
		for _, r := range recipesForKind(kind) {
			fmt.Fprintf(cmd.OutOrStdout(), "%s/%s\n", kind, r)
		}
	}
	return nil
}

func listKinds() ([]string, error) {
	entries, err := scaffoldFS.ReadDir(scaffoldFSRoot)
	if err != nil {
		return nil, fmt.Errorf("read bundled scaffolds: %w", err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func recipesForKind(kind string) []string {
	entries, err := fs.ReadDir(scaffoldFS, path.Join(scaffoldFSRoot, kind))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if !strings.HasSuffix(n, ".yaml") {
			continue
		}
		out = append(out, strings.TrimSuffix(n, ".yaml"))
	}
	sort.Strings(out)
	return out
}

func recipeNamesAcrossAllKinds() []string {
	kinds, _ := listKinds()
	seen := map[string]struct{}{}
	for _, k := range kinds {
		for _, r := range recipesForKind(k) {
			seen[r] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for r := range seen {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
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
