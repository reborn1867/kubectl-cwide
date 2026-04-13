package list

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/discovery"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/utils/ptr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type apiResourceEntry struct {
	Name       string
	ShortNames string
	APIVersion string
	Namespaced bool
	Kind       string
}

// ListAllOptions holds state for the "list all" command.
type ListAllOptions struct {
	ClusterScoped bool
	Context       string
	NoHeaders     bool

	factory cmdutil.Factory

	genericiooptions.IOStreams
}

func NewCmdListAll(streams genericiooptions.IOStreams) *cobra.Command {
	o := &ListAllOptions{IOStreams: streams}

	cmd := &cobra.Command{
		Use:        "all [flags]",
		Aliases:    []string{"a"},
		SuggestFor: []string{"resources", "api-resources"},
		Short:      "List all API resources in the cluster",
		Long: `List all Kubernetes API resources discovered via the cluster's Discovery API.

By default, only namespaced resources are shown. Use -A to show cluster-scoped
(non-namespaced) resources instead.

Output columns: NAME, SHORTNAMES, APIVERSION, NAMESPACED, KIND`,
		Example: `  # List namespaced API resources
  kubectl cwide list all

  # List cluster-scoped (non-namespaced) API resources
  kubectl cwide list all -A

  # Use a specific context
  kubectl cwide list all --context my-cluster`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(cmd); err != nil {
				return err
			}
			return o.Run()
		},
	}

	cmd.Flags().BoolVarP(&o.ClusterScoped, "cluster-scoped", "A", false,
		"Show cluster-scoped (non-namespaced) resources instead of namespaced resources")
	cmd.Flags().StringVar(&o.Context, "context", "",
		"The name of the kubeconfig context to use")
	cmd.Flags().BoolVar(&o.NoHeaders, "no-headers", false,
		"Don't print column headers")

	return cmd
}

// Complete sets up the factory from kubeconfig flags.
func (o *ListAllOptions) Complete(cmd *cobra.Command) error {
	kubeConfigFlags := genericclioptions.NewConfigFlags(true).
		WithDeprecatedPasswordFlag().
		WithDiscoveryBurst(300).
		WithDiscoveryQPS(50.0)

	if v := cmd.Flag("kubeconfig"); v != nil && v.Changed {
		kubeConfigFlags.KubeConfig = ptr.To(v.Value.String())
	}
	if o.Context != "" {
		kubeConfigFlags.Context = &o.Context
	}

	matchVersionFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	o.factory = cmdutil.NewFactory(matchVersionFlags)

	return nil
}

// Run discovers API resources and prints a filtered, sorted table.
func (o *ListAllOptions) Run() error {
	discoveryClient, err := o.factory.ToDiscoveryClient()
	if err != nil {
		return fmt.Errorf("failed to create discovery client: %w", err)
	}

	_, resourceLists, err := discoveryClient.ServerGroupsAndResources()
	if err != nil {
		if !discovery.IsGroupDiscoveryFailedError(err) {
			return fmt.Errorf("failed to discover server resources: %w", err)
		}
		fmt.Fprintf(o.ErrOut, "Warning: some API groups could not be discovered: %v\n", err)
	}

	entries := filterResources(resourceLists, o.ClusterScoped)

	w := printers.GetNewTabWriter(o.Out)
	defer w.Flush()

	if !o.NoHeaders {
		fmt.Fprintln(w, "NAME\tSHORTNAMES\tAPIVERSION\tNAMESPACED\tKIND")
	}

	for _, e := range entries {
		namespacedStr := "true"
		if !e.Namespaced {
			namespacedStr = "false"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			e.Name, e.ShortNames, e.APIVersion, namespacedStr, e.Kind)
	}

	return nil
}

// filterResources filters API resources by scope and removes subresources.
// When clusterScoped is false, only namespaced resources are returned.
// When clusterScoped is true, only cluster-scoped resources are returned.
func filterResources(resourceLists []*metav1.APIResourceList, clusterScoped bool) []apiResourceEntry {
	var entries []apiResourceEntry

	for _, resourceList := range resourceLists {
		for _, resource := range resourceList.APIResources {
			// Skip subresources (e.g. pods/status, deployments/scale)
			if strings.Contains(resource.Name, "/") {
				continue
			}

			if !clusterScoped && !resource.Namespaced {
				continue
			}
			if clusterScoped && resource.Namespaced {
				continue
			}

			entries = append(entries, apiResourceEntry{
				Name:       resource.Name,
				ShortNames: strings.Join(resource.ShortNames, ","),
				APIVersion: resourceList.GroupVersion,
				Namespaced: resource.Namespaced,
				Kind:       resource.Kind,
			})
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})

	return entries
}
