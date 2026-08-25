// Package completions holds shared cobra ValidArgsFunction/completion callbacks
// used across cwide subcommands so `kubectl cwide get <TAB>`, alias names, and
// template names all get intelligent completion.
package completions

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/kubectl-cwide/pkg/clients"
	"github.com/kubectl-cwide/pkg/utils"
)

// ResourceTypes returns names + short names of every resource the cluster
// advertises via discovery, plus every user-defined alias.
//
// Errors are swallowed by default so tab completion never disrupts the shell.
// Set CWIDE_COMPLETE_DEBUG=1 to log each failure to stderr so users can
// diagnose "TAB shows nothing" without touching the source.
func ResourceTypes(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// If the user has already typed a resource type, don't complete a second one.
	if len(args) >= 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	seen := map[string]struct{}{}
	var out []string
	add := func(s string) {
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}

	// Aliases first — they're small and free to load.
	if cfg, err := utils.LoadConfig(); err == nil {
		for name := range cfg.Aliases {
			add(name)
		}
	} else {
		debugf("LoadConfig: %v", err)
	}

	// Cluster-served resources.
	factory := clients.FactoryFromCmd(cmd, contextFromFlag(cmd))
	disc, err := factory.ToDiscoveryClient()
	if err != nil {
		debugf("ToDiscoveryClient: %v", err)
	} else {
		_, resourceLists, err := disc.ServerGroupsAndResources()
		if err != nil && !discovery.IsGroupDiscoveryFailedError(err) {
			debugf("ServerGroupsAndResources: %v", err)
		}
		for _, list := range resourceLists {
			for _, r := range list.APIResources {
				if strings.Contains(r.Name, "/") {
					continue // skip subresources
				}
				add(r.Name)
				for _, sn := range r.ShortNames {
					add(sn)
				}
			}
		}
	}

	sort.Strings(out)
	return filterPrefix(out, toComplete), cobra.ShellCompDirectiveNoFileComp
}

// debugf writes to stderr when CWIDE_COMPLETE_DEBUG is set. Completion
// callbacks run inside a shell hook, so this is the only channel the user
// can observe. Cobra will still emit the completion payload separately.
func debugf(format string, args ...interface{}) {
	if os.Getenv("CWIDE_COMPLETE_DEBUG") == "" {
		return
	}
	fmt.Fprintf(os.Stderr, "cwide-complete: "+format+"\n", args...)
}

// AliasNames completes with the names of currently-configured aliases.
// Handy for `alias delete <TAB>`.
func AliasNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) >= 1 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := utils.LoadConfig()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names := make([]string, 0, len(cfg.Aliases))
	for k := range cfg.Aliases {
		names = append(names, k)
	}
	sort.Strings(names)
	return filterPrefix(names, toComplete), cobra.ShellCompDirectiveNoFileComp
}

// TemplateNames completes with the template basenames that exist for the
// resource type passed as the first positional arg. Falls back to scanning
// every resource directory when no resource is known yet.
func TemplateNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	rootPath, err := utils.ResolveTemplatePath(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	resource := ""
	if len(args) >= 1 {
		resource = strings.ToLower(utils.ResolveAliasString(args[0]))
	}

	// Resolve the typed token (plural/singular/short name) to the Kind's directory
	// prefix via the cluster's RESTMapper — the same resolution `kubectl get
	// <resource>` performs. This handles short names (svc, deploy, po) and every
	// irregular plural authoritatively. When the mapper is unavailable (e.g. no
	// cluster reachable but local templates exist) dirPrefix stays empty and we
	// fall back to the offline meta.UnsafeGuessKindToResource heuristic below.
	dirPrefix := ""
	if resource != "" {
		dirPrefix = resolveKindDirPrefix(cmd, resource)
	}

	seen := map[string]struct{}{}
	var out []string

	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if e.Name() == "_shared" {
			continue
		}
		if resource != "" {
			name := strings.ToLower(e.Name())
			if dirPrefix != "" {
				if !strings.HasPrefix(name, dirPrefix) {
					continue
				}
			} else if !resourceMatchesDir(name, resource) {
				continue
			}
		}
		files, err := os.ReadDir(filepath.Join(rootPath, e.Name()))
		if err != nil {
			continue
		}
		for _, f := range files {
			name := f.Name()
			switch {
			case strings.HasSuffix(name, ".yaml"):
				name = strings.TrimSuffix(name, ".yaml")
			case strings.HasSuffix(name, ".tpl"):
				name = strings.TrimSuffix(name, ".tpl")
			default:
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return filterPrefix(out, toComplete), cobra.ShellCompDirectiveNoFileComp
}

// KubeContexts completes with context names from the loading kubeconfig.
func KubeContexts(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	if v := cmd.Flag("kubeconfig"); v != nil && v.Changed {
		rules.ExplicitPath = v.Value.String()
	}
	cfg, err := rules.Load()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	names := make([]string, 0, len(cfg.Contexts))
	for k := range cfg.Contexts {
		names = append(names, k)
	}
	sort.Strings(names)
	return filterPrefix(names, toComplete), cobra.ShellCompDirectiveNoFileComp
}

func contextFromFlag(cmd *cobra.Command) string {
	if f := cmd.Flag("context"); f != nil {
		return f.Value.String()
	}
	return ""
}

// resolveKindDirPrefix resolves a user-typed resource token (plural, singular, or
// short name) to the lowercase "<kind>-<group>-" prefix of its template directory
// (see utils.GenerateDirNameByGVK), using the cluster's RESTMapper the same way
// `kubectl get <resource>` resolves its arguments. The version is intentionally
// omitted from the prefix so templates for every served version of the Kind match.
// Returns "" when the token can't be resolved (no cluster, unknown/ambiguous
// resource); callers should then fall back to resourceMatchesDir.
func resolveKindDirPrefix(cmd *cobra.Command, resource string) string {
	factory := clients.FactoryFromCmd(cmd, contextFromFlag(cmd))
	mapper, err := factory.ToRESTMapper()
	if err != nil {
		debugf("ToRESTMapper: %v", err)
		return ""
	}
	gvk, err := mapper.KindFor(schema.GroupVersionResource{Resource: resource})
	if err != nil {
		debugf("KindFor(%q): %v", resource, err)
		return ""
	}
	return strings.ToLower(gvk.Kind + "-" + gvk.Group + "-")
}

// resourceMatchesDir is the offline fallback for resolveKindDirPrefix. It reports
// whether a template directory corresponds to the resource token the user typed.
// Template dirs are named "<kind>-<group>-<version>" (all lowercase, see
// utils.GenerateDirNameByGVK), and Kinds never contain a dash, so the first
// "-"-delimited segment is the Kind.
//
// The typed token may be the singular Kind ("pod"), the singular resource, or the
// plural resource ("pods"). Plural/singular forms are derived with
// meta.UnsafeGuessKindToResource — the same heuristic Kubernetes' DefaultRESTMapper
// uses — so we correctly handle regular ("pods"), -es ("ingresses"), -ies
// ("networkpolicies") and unpluralized ("endpoints") names without hand-rolling
// suffix rules. It cannot resolve short names (svc, po); those require the mapper.
// dirName is expected to be lowercase.
func resourceMatchesDir(dirName, resource string) bool {
	kind := strings.SplitN(dirName, "-", 2)[0]
	if kind == "" {
		return false
	}
	if resource == kind {
		return true
	}
	plural, singular := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{Kind: kind})
	return resource == plural.Resource || resource == singular.Resource
}

func filterPrefix(all []string, prefix string) []string {
	if prefix == "" {
		return all
	}
	out := make([]string, 0, len(all))
	for _, s := range all {
		if strings.HasPrefix(s, prefix) {
			out = append(out, s)
		}
	}
	return out
}
