package get

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/kubectl-cwide/pkg/clients"
)

const (
	// defaultAllResourcesWorkers caps concurrent list requests to avoid
	// hammering the apiserver on clusters with hundreds of resource types.
	defaultAllResourcesWorkers = 16
	// defaultAllResourcesTimeout bounds each per-resource LIST call so a
	// single slow/broken aggregated apiserver can't stall the whole run.
	defaultAllResourcesTimeout = 30 * time.Second
)

// AllResourcesOptions holds state for the "get all-resources" command.
type AllResourcesOptions struct {
	AllNamespaces bool
	Namespace     string
	Context       string
	NoHeaders     bool
	Workers       int
	Timeout       time.Duration
	ShowEmpty     bool

	factory cmdutil.Factory

	genericiooptions.IOStreams
}

// resourceHit is one object discovered while listing all resource types.
type resourceHit struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
	Age        string
}

// listError records a per-resource-type failure so we can report it
// without aborting the entire fan-out.
type listError struct {
	GVR schema.GroupVersionResource
	Err error
}

func NewCmdGetAllResources(streams genericiooptions.IOStreams) *cobra.Command {
	o := &AllResourcesOptions{
		IOStreams: streams,
		Workers:   defaultAllResourcesWorkers,
		Timeout:   defaultAllResourcesTimeout,
	}

	cmd := &cobra.Command{
		Use:     "all-resources",
		Aliases: []string{"ac"},
		Short:   "List every resource of every discovered API type",
		Long: `List every object of every namespaced resource type discovered via the
cluster's Discovery API, in parallel.

By default only the current namespace is listed. Use -A to list across all
namespaces (cluster-scoped resources are included when -A is set).

The output is a single table sorted by APIVERSION, KIND, NAMESPACE, NAME.`,
		Example: `  # List every resource in the current namespace
  kubectl cwide get all-resources

  # Short alias
  kubectl cwide get ac

  # Across all namespaces (includes cluster-scoped types)
  kubectl cwide get all-resources -A

  # Tune the parallel worker pool
  kubectl cwide get all-resources -A --workers 32`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(cmd); err != nil {
				return err
			}
			return o.Run(cmd.Context())
		},
	}

	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespaces", "A", false,
		"If present, list resources across all namespaces and include cluster-scoped types")
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", "",
		"If present, the namespace scope for this CLI request")
	cmd.Flags().StringVar(&o.Context, "context", "",
		"The name of the kubeconfig context to use")
	cmd.Flags().BoolVar(&o.NoHeaders, "no-headers", false, "Don't print column headers")
	cmd.Flags().IntVar(&o.Workers, "workers", o.Workers,
		"Maximum concurrent LIST requests")
	cmd.Flags().DurationVar(&o.Timeout, "list-timeout", o.Timeout,
		"Per-resource-type LIST timeout")
	cmd.Flags().BoolVar(&o.ShowEmpty, "show-empty", false,
		"Include resource types that returned zero objects (as an informational row)")

	return cmd
}

func (o *AllResourcesOptions) Complete(cmd *cobra.Command) error {
	o.factory = clients.FactoryFromCmd(cmd, o.Context)

	if o.Namespace == "" && !o.AllNamespaces {
		ns, _, err := o.factory.ToRawKubeConfigLoader().Namespace()
		if err != nil {
			return fmt.Errorf("failed to resolve current namespace: %w", err)
		}
		o.Namespace = ns
	}

	if o.Workers <= 0 {
		o.Workers = defaultAllResourcesWorkers
	}

	return nil
}

func (o *AllResourcesOptions) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	discoveryClient, err := o.factory.ToDiscoveryClient()
	if err != nil {
		return fmt.Errorf("failed to create discovery client: %w", err)
	}
	dynClient, err := o.factory.DynamicClient()
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	_, resourceLists, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		if !discovery.IsGroupDiscoveryFailedError(err) {
			return fmt.Errorf("failed to discover server resources: %w", err)
		}
		fmt.Fprintf(o.ErrOut, "Warning: some API groups could not be discovered: %v\n", err)
	}

	targets := selectListableResources(resourceLists, o.AllNamespaces)
	if len(targets) == 0 {
		fmt.Fprintln(o.ErrOut, "No listable API resources discovered.")
		return nil
	}

	hits, errs := o.fanOutList(ctx, dynClient, targets)

	// Stable, human-friendly ordering.
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].APIVersion != hits[j].APIVersion {
			return hits[i].APIVersion < hits[j].APIVersion
		}
		if hits[i].Kind != hits[j].Kind {
			return hits[i].Kind < hits[j].Kind
		}
		if hits[i].Namespace != hits[j].Namespace {
			return hits[i].Namespace < hits[j].Namespace
		}
		return hits[i].Name < hits[j].Name
	})

	w := printers.GetNewTabWriter(o.Out)
	defer w.Flush()

	if !o.NoHeaders {
		if o.AllNamespaces {
			fmt.Fprintln(w, "APIVERSION\tKIND\tNAMESPACE\tNAME\tAGE")
		} else {
			fmt.Fprintln(w, "APIVERSION\tKIND\tNAME\tAGE")
		}
	}

	for _, h := range hits {
		if o.AllNamespaces {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", h.APIVersion, h.Kind, h.Namespace, h.Name, h.Age)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", h.APIVersion, h.Kind, h.Name, h.Age)
		}
	}

	if len(errs) > 0 {
		fmt.Fprintf(o.ErrOut, "\n%d resource type(s) failed to list:\n", len(errs))
		sort.Slice(errs, func(i, j int) bool { return errs[i].GVR.String() < errs[j].GVR.String() })
		for _, e := range errs {
			fmt.Fprintf(o.ErrOut, "  %s: %v\n", e.GVR.String(), e.Err)
		}
	}

	return nil
}

// listableTarget is one GVR we plan to LIST, plus display metadata.
type listableTarget struct {
	GVR        schema.GroupVersionResource
	Kind       string
	Namespaced bool
	APIVersion string
}

// selectListableResources produces the deduped set of GVRs to LIST.
//
// Rules:
//   - subresources (name contains "/") are dropped
//   - only resources whose verbs include "list" are kept
//   - when allNamespaces is false, cluster-scoped resources are dropped
//     (there is no per-namespace answer for them)
//   - when the same GroupResource appears at multiple versions, we keep
//     one — preferring the version marked as the group-preferred one
//     via the discovery ordering
func selectListableResources(resourceLists []*metav1.APIResourceList, allNamespaces bool) []listableTarget {
	// Deduplicate on GroupResource so we don't list the same objects twice
	// through v1beta1 and v1 of the same CRD.
	seen := make(map[schema.GroupResource]listableTarget)

	for _, rl := range resourceLists {
		gv, err := schema.ParseGroupVersion(rl.GroupVersion)
		if err != nil {
			continue
		}
		for _, r := range rl.APIResources {
			if strings.Contains(r.Name, "/") {
				continue
			}
			if !containsVerb(r.Verbs, "list") {
				continue
			}
			if !allNamespaces && !r.Namespaced {
				continue
			}

			gr := schema.GroupResource{Group: gv.Group, Resource: r.Name}
			if _, ok := seen[gr]; ok {
				// Discovery returns preferred version first for a group;
				// keep the first one we see.
				continue
			}

			seen[gr] = listableTarget{
				GVR:        gv.WithResource(r.Name),
				Kind:       r.Kind,
				Namespaced: r.Namespaced,
				APIVersion: rl.GroupVersion,
			}
		}
	}

	out := make([]listableTarget, 0, len(seen))
	for _, t := range seen {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].GVR.String() < out[j].GVR.String()
	})
	return out
}

func containsVerb(verbs metav1.Verbs, want string) bool {
	for _, v := range verbs {
		if v == want {
			return true
		}
	}
	return false
}

// fanOutList runs LIST for each target concurrently, bounded by o.Workers,
// and returns the merged set of hits plus any per-target errors.
func (o *AllResourcesOptions) fanOutList(
	ctx context.Context,
	dyn dynamic.Interface,
	targets []listableTarget,
) ([]resourceHit, []listError) {
	sem := make(chan struct{}, o.Workers)
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		hits []resourceHit
		errs []listError
	)

	now := time.Now()

	for _, t := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(t listableTarget) {
			defer wg.Done()
			defer func() { <-sem }()

			perCallCtx, cancel := context.WithTimeout(ctx, o.Timeout)
			defer cancel()

			var (
				list *unstructured.UnstructuredList
				err  error
			)
			if t.Namespaced && !o.AllNamespaces {
				list, err = dyn.Resource(t.GVR).
					Namespace(o.Namespace).
					List(perCallCtx, metav1.ListOptions{})
			} else {
				list, err = dyn.Resource(t.GVR).
					List(perCallCtx, metav1.ListOptions{})
			}

			if err != nil {
				mu.Lock()
				errs = append(errs, listError{GVR: t.GVR, Err: err})
				mu.Unlock()
				return
			}

			local := make([]resourceHit, 0, len(list.Items))
			for i := range list.Items {
				item := &list.Items[i]
				local = append(local, resourceHit{
					APIVersion: t.APIVersion,
					Kind:       t.Kind,
					Namespace:  item.GetNamespace(),
					Name:       item.GetName(),
					Age:        translateAge(item.GetCreationTimestamp(), now),
				})
			}

			mu.Lock()
			hits = append(hits, local...)
			if o.ShowEmpty && len(list.Items) == 0 {
				hits = append(hits, resourceHit{
					APIVersion: t.APIVersion,
					Kind:       t.Kind,
					Name:       "<none>",
					Age:        "<none>",
				})
			}
			mu.Unlock()
		}(t)
	}

	wg.Wait()
	return hits, errs
}

// translateAge renders a creationTimestamp as a short duration string
// (e.g. "3d", "5h", "12m"). Mirrors kubectl's default output style.
func translateAge(ts metav1.Time, now time.Time) string {
	if ts.IsZero() {
		return "<unknown>"
	}
	d := now.Sub(ts.Time)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
