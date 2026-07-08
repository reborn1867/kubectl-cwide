package tree

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kubectl-cwide/pkg/models"
	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/utils/ptr"
)

// TreeNode represents one resource in the tree.
type TreeNode struct {
	GVK       schema.GroupVersionKind
	Name      string
	Namespace string
	UID       types.UID
	Object    *unstructured.Unstructured
	Children  []*TreeNode
}

// TreeOptions holds all state for the tree command.
type TreeOptions struct {
	Namespace         string
	ExplicitNamespace bool
	AllNamespaces     bool
	Context           string

	RulesFile    string
	RelatedFlags []string
	MaxDepth     int

	rootResource string
	rootName     string
	relations    []models.TreeRelation

	factory    cmdutil.Factory
	restMapper meta.RESTMapper
	dynClient  dynamic.Interface

	genericiooptions.IOStreams
}

// NewCmdTree creates the cobra command for kubectl cwide tree.
func NewCmdTree(streams genericiooptions.IOStreams) *cobra.Command {
	o := &TreeOptions{IOStreams: streams}

	cmd := &cobra.Command{
		Use:        "tree TYPE/NAME [flags]",
		Aliases:    []string{"t"},
		SuggestFor: []string{"hierarchy", "graph", "related"},
		Short:      "Show a tree of related Kubernetes resources",
		Long: `Display a tree of Kubernetes resources linked by ownership, labels, or field references.

Relationships are defined in a YAML rules file (--rules) or inline via --related flags.
The tree starts from the specified root resource and discovers related resources
according to the configured bindings.

Binding types:
  ownerRef       — child resources whose ownerReferences point to the parent
  labelSelector  — resources matched by the parent's label selector (bidirectional)
  fieldRef       — resources referenced by name in a parent's field (via JSONPath)`,
		Example: `  # Show a deployment tree using a rules file
  kubectl cwide tree deployment/nginx -f deploy-stack.yaml

  # Inline relationships
  kubectl cwide tree deployment/nginx --related=replicasets:ownerRef --related=pods:ownerRef:replicasets

  # Mixed: rules file + extra inline relation
  kubectl cwide tree deployment/nginx -f deploy-stack.yaml --related=hpa:ownerRef`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Complete(cmd, args); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			return o.Run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&o.RulesFile, "rules", "f", "", "Path to a YAML rules file defining resource relationships")
	cmd.Flags().StringArrayVar(&o.RelatedFlags, "related", nil, "Inline relationship: <resource>:<bindType>[:<parent>] (repeatable)")
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", "", "Namespace scope for this request")
	cmd.Flags().StringVar(&o.Context, "context", "", "The name of the kubeconfig context to use")
	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespaces", "A", false, "List across all namespaces")
	cmd.Flags().IntVar(&o.MaxDepth, "max-depth", 0, "Maximum tree depth to render; 0 means unbounded. Cycles are always broken with a (cycle) marker.")

	return cmd
}

// Complete parses arguments and sets up the Kubernetes client.
func (o *TreeOptions) Complete(cmd *cobra.Command, args []string) error {
	// Parse type/name
	parts := strings.SplitN(args[0], "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("argument must be in TYPE/NAME format (e.g. deployment/nginx)")
	}
	o.rootResource = parts[0]
	o.rootName = parts[1]

	// Resolve alias on the resource type
	o.rootResource = utils.ResolveAliasString(o.rootResource)

	// Set up factory
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

	// Resolve namespace
	var err error
	if o.Namespace == "" {
		o.Namespace, o.ExplicitNamespace, err = o.factory.ToRawKubeConfigLoader().Namespace()
		if err != nil {
			return fmt.Errorf("failed to resolve namespace: %w", err)
		}
	}
	if o.AllNamespaces {
		o.Namespace = ""
	}

	// Clients
	o.restMapper, err = o.factory.ToRESTMapper()
	if err != nil {
		return fmt.Errorf("failed to get REST mapper: %w", err)
	}
	o.dynClient, err = o.factory.DynamicClient()
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Parse rules
	if o.RulesFile != "" {
		data, err := os.ReadFile(o.RulesFile)
		if err != nil {
			return fmt.Errorf("failed to read rules file: %w", err)
		}
		var rules models.TreeRuleFile
		if err := yaml.Unmarshal(data, &rules); err != nil {
			return fmt.Errorf("failed to parse rules file: %w", err)
		}
		o.relations = append(o.relations, rules.Relations...)
	}

	for _, flag := range o.RelatedFlags {
		rel, err := parseRelatedFlag(flag)
		if err != nil {
			return err
		}
		o.relations = append(o.relations, rel)
	}

	return nil
}

// Validate checks that options are consistent.
func (o *TreeOptions) Validate() error {
	if o.rootResource == "" || o.rootName == "" {
		return fmt.Errorf("root resource and name are required")
	}
	if len(o.relations) == 0 {
		return fmt.Errorf("at least one relation is required (use --rules or --related)")
	}
	for _, rel := range o.relations {
		switch rel.Bind.Type {
		case "ownerRef", "labelSelector", "fieldRef":
		default:
			return fmt.Errorf("invalid bind type %q for resource %q (must be ownerRef, labelSelector, or fieldRef)",
				rel.Bind.Type, rel.Resource)
		}
		if rel.Bind.Type == "fieldRef" && rel.Bind.Path == "" {
			return fmt.Errorf("fieldRef binding for %q requires a path", rel.Resource)
		}
	}
	return nil
}

// Run fetches the root resource, builds the tree, and renders it.
func (o *TreeOptions) Run(ctx context.Context) error {
	// Resolve root GVR
	rootGVR, err := resolveGVR(o.restMapper, o.rootResource)
	if err != nil {
		return fmt.Errorf("failed to resolve resource %q: %w", o.rootResource, err)
	}

	// Fetch root object
	rootObj, err := o.dynClient.Resource(rootGVR).Namespace(o.Namespace).Get(ctx, o.rootName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get %s/%s: %w", o.rootResource, o.rootName, err)
	}

	rootNode := nodeFromUnstructured(rootObj)

	// Build tree: topological resolution by parent dependency
	nodesByResource := map[string][]*TreeNode{
		o.rootResource: {rootNode},
	}
	nodesByUID := map[types.UID]*TreeNode{
		rootNode.UID: rootNode,
	}

	resolved := make(map[int]bool)
	for range o.relations {
		progress := false
		for i, rel := range o.relations {
			if resolved[i] {
				continue
			}
			parentKey := rel.Bind.Parent
			if parentKey == "" {
				parentKey = o.rootResource
			}
			parentNodes, ok := nodesByResource[parentKey]
			if !ok {
				continue // parent not yet resolved, try next pass
			}

			childrenByParent, err := ResolveChildren(ctx, o.dynClient, o.restMapper, rel, parentNodes, o.Namespace)
			if err != nil {
				return fmt.Errorf("failed to resolve %s: %w", rel.Resource, err)
			}

			var allChildren []*TreeNode
			for parentUID, children := range childrenByParent {
				if parent, ok := nodesByUID[parentUID]; ok {
					parent.Children = append(parent.Children, children...)
				}
				for _, child := range children {
					nodesByUID[child.UID] = child
				}
				allChildren = append(allChildren, children...)
			}
			nodesByResource[rel.Resource] = append(nodesByResource[rel.Resource], allChildren...)
			resolved[i] = true
			progress = true
		}
		if !progress {
			break
		}
	}

	// Check for unresolved relations
	for i, rel := range o.relations {
		if !resolved[i] {
			parentKey := rel.Bind.Parent
			if parentKey == "" {
				parentKey = o.rootResource
			}
			return fmt.Errorf("could not resolve relation for %q: parent %q was not found in the tree", rel.Resource, parentKey)
		}
	}

	RenderTree(rootNode, o.Out, o.MaxDepth)
	return nil
}

// parseRelatedFlag parses a --related flag value: <resource>:<bindType>[:<parent>]
func parseRelatedFlag(flag string) (models.TreeRelation, error) {
	parts := strings.SplitN(flag, ":", 3)
	if len(parts) < 2 {
		return models.TreeRelation{}, fmt.Errorf(
			"invalid --related format %q, expected <resource>:<bindType>[:<parent>]", flag)
	}
	rel := models.TreeRelation{
		Resource: parts[0],
		Bind: models.TreeBindSpec{
			Type: parts[1],
		},
	}
	if len(parts) == 3 {
		rel.Bind.Parent = parts[2]
	}
	return rel, nil
}
