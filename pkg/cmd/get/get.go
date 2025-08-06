package get

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/utils/ptr"

	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/interrupt"
)

type GetOptions struct {
	Raw       string
	Watch     bool
	WatchOnly bool
	ChunkSize int64

	OutputWatchEvents bool

	LabelSelector     string
	FieldSelector     string
	AllNamespaces     bool
	Namespace         string
	ExplicitNamespace bool
	Subresource       string
	SortBy            string
	resource.FilenameOptions

	ServerPrint bool

	NoHeaders      bool
	IgnoreNotFound bool

	genericiooptions.IOStreams

	Template          string
	Context           string
	TemplateRootPath  string
	EnableCustomTable bool
}

// NewGetOptions returns a GetOptions with default chunk size 500.
func NewGetOptions(streams genericiooptions.IOStreams) *GetOptions {
	return &GetOptions{

		IOStreams:   streams,
		ChunkSize:   cmdutil.DefaultChunkSize,
		ServerPrint: true,
	}
}

func (o *GetOptions) Run(cmd *cobra.Command, args []string) error {
	var rootPath string
	var err error
	if cmd.Flag("template-path").Changed {
		rootPath = cmd.Flag("template-path").Value.String()
	} else {
		// get template path from config.yaml
		rootPath, err = utils.GetTemplatePathFromConfig()
		if err != nil {
			return err
		}
	}

	o.TemplateRootPath = rootPath

	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag().WithDiscoveryBurst(300).WithDiscoveryQPS(50.0)

	if o.Context != "" {
		kubeConfigFlags.Context = &o.Context
	}

	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)

	if o.Watch || o.WatchOnly {
		return o.watch(f, args)
	}

	if o.Namespace == "" {
		o.Namespace, o.ExplicitNamespace, err = f.ToRawKubeConfigLoader().Namespace()
		if err != nil {
			return err
		}
	}

	if o.AllNamespaces {
		o.ExplicitNamespace = false
	}

	o.NoHeaders = cmdutil.GetFlagBool(cmd, "no-headers")

	r := f.NewBuilder().
		Unstructured().
		DefaultNamespace().
		NamespaceParam(o.Namespace).
		AllNamespaces(o.AllNamespaces).
		FilenameParam(o.ExplicitNamespace, &o.FilenameOptions).
		LabelSelectorParam(o.LabelSelector).
		FieldSelectorParam(o.FieldSelector).
		Subresource(o.Subresource).
		RequestChunksOf(o.ChunkSize).
		// TransformRequests(o.transformRequests).
		ResourceTypeOrNameArgs(true, args...).
		ContinueOnError().
		Latest().
		Flatten().
		Do()

	if err := r.Err(); err != nil {
		return err
	}

	infos, err := r.Infos()
	if err != nil {
		return fmt.Errorf("error fetching resources: %v\n", err)
	}

	if len(infos) == 0 {
		fmt.Fprintf(o.ErrOut, "No resources found in %s namespace.\n", o.Namespace)
		return nil
	}

	var crdTemplateDir string
	crdTemplateDir = utils.GenerateDirNameByGVK(infos[0].Object.GetObjectKind().GroupVersionKind())

	templateFile := fmt.Sprintf("%s.tpl", o.Template)

	file, err := os.Open(filepath.Join(rootPath, crdTemplateDir, templateFile))
	if err != nil {
		return fmt.Errorf("error reading template %s, %v\n", filepath.Join(rootPath, crdTemplateDir, templateFile), err)
	}

	decoder := scheme.Codecs.UniversalDecoder(scheme.Scheme.PrioritizedVersionsAllGroups()...)

	restConfig, err := f.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("error getting rest config: %v\n", err)
	}

	printer, err := NewCustomColumnsPrinterFromTemplate(file, decoder, restConfig)
	if err != nil {
		return fmt.Errorf("error creating printer from template: %v\n", err)
	}

	if o.EnableCustomTable {
		printer.WithCustomTable()
	}

	printer.NoHeaders = o.NoHeaders

	w := printers.GetNewTabWriter(os.Stdout)
	for _, info := range infos {
		if err := printer.PrintObj(info.Object, w); err != nil {
			return fmt.Errorf("error printing object: %v\n", err)
		}
	}

	if printer.CustomTable != nil {
		printer.CustomTable.Render()
	} else {
		w.Flush()
	}

	return nil
}

func (o *GetOptions) watch(f cmdutil.Factory, args []string) error {
	r := f.NewBuilder().
		Unstructured().
		DefaultNamespace().
		NamespaceParam(o.Namespace).
		AllNamespaces(o.AllNamespaces).
		FilenameParam(o.ExplicitNamespace, &o.FilenameOptions).
		LabelSelectorParam(o.LabelSelector).
		FieldSelectorParam(o.FieldSelector).
		Subresource(o.Subresource).
		RequestChunksOf(o.ChunkSize).
		// TransformRequests(o.transformRequests).
		ResourceTypeOrNameArgs(true, args...).
		ContinueOnError().
		Latest().
		Flatten().
		Do()
	if err := r.Err(); err != nil {
		return err
	}
	infos, err := r.Infos()
	if err != nil {
		return err
	}
	if multipleGVKsRequested(infos) {
		return i18n.Errorf("watch is only supported on individual resources and resource collections - more than 1 resource was found")
	}

	var crdTemplateDir string
	crdTemplateDir = utils.GenerateDirNameByGVK(infos[0].Object.GetObjectKind().GroupVersionKind())

	templateFile := fmt.Sprintf("%s.tpl", o.Template)

	file, err := os.Open(filepath.Join(o.TemplateRootPath, crdTemplateDir, templateFile))
	if err != nil {
		return fmt.Errorf("error reading template %s, %v\n", filepath.Join(o.TemplateRootPath, crdTemplateDir, templateFile), err)
	}

	decoder := scheme.Codecs.UniversalDecoder(scheme.Scheme.PrioritizedVersionsAllGroups()...)

	restConfig, err := f.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("error getting rest config: %v\n", err)
	}
	outputObjects := ptr.To(!o.WatchOnly)
	printer, err := NewCustomColumnsPrinterFromTemplate(file, decoder, restConfig)
	if err != nil {
		return fmt.Errorf("error creating printer from template: %v\n", err)
	}

	if o.EnableCustomTable {
		printer.WithCustomTable()
	}

	// print the current object
	printer.NoHeaders = o.NoHeaders

	w := printers.GetNewTabWriter(os.Stdout)
	for _, info := range infos {
		if err := printer.PrintObj(info.Object, w); err != nil {
			return fmt.Errorf("error printing object: %v\n", err)
		}
	}

	if printer.CustomTable != nil {
		printer.CustomTable.Render()
	} else {
		w.Flush()
	}

	obj, err := r.Object()
	if err != nil {
		return err
	}
	// watching from resourceVersion 0, starts the watch at ~now and
	// will return an initial watch event.  Starting form ~now, rather
	// the rv of the object will insure that we start the watch from
	// inside the watch window, which the rv of the object might not be.
	rv := "0"
	isList := meta.IsListType(obj)
	if isList {
		// the resourceVersion of list objects is ~now but won't return
		// an initial watch event
		rv, err = meta.NewAccessor().ResourceVersion(obj)
		if err != nil {
			return err
		}
	}

	if isList {
		// we can start outputting objects now, watches started from lists don't emit synthetic added events
		*outputObjects = true
	} else {
		// suppress output, since watches started for individual items emit a synthetic ADDED event first
		*outputObjects = false
	}

	// print watched changes
	watcher, err := r.Watch(rv)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	intr := interrupt.New(nil, cancel)
	intr.Run(func() error {
		_, err := watchtools.UntilWithoutRetry(ctx, watcher, func(e watch.Event) (bool, error) {
			objToPrint := e.Object
			if o.OutputWatchEvents {
				objToPrint = &metav1.WatchEvent{Type: string(e.Type), Object: runtime.RawExtension{Object: objToPrint}}
			}

			if err := printer.PrintObj(objToPrint, w); err != nil {
				return false, err
			}

			if printer.CustomTable != nil {
				printer.CustomTable.Render()
			} else {
				w.Flush()
			}
			// after processing at least one event, start outputting objects
			*outputObjects = true
			return false, nil
		})
		return err
	})
	return nil
}

func NewCmdGet(streams genericiooptions.IOStreams) *cobra.Command {
	o := NewGetOptions(streams)

	cmd := &cobra.Command{
		Use:   "get",
		Short: "get k8s resources in custom wide output format",
		RunE:  o.Run,
	}

	cmd.Flags().BoolVar(&o.NoHeaders, "no-headers", o.NoHeaders, "When using the default or custom-column output format, don't print headers (default print headers).")
	cmd.Flags().StringVar(&o.Raw, "raw", o.Raw, "Raw URI to request from the server.  Uses the transport specified by the kubeconfig file.")
	cmd.Flags().BoolVarP(&o.Watch, "watch", "w", o.Watch, "After listing/getting the requested object, watch for changes.")
	cmd.Flags().BoolVar(&o.WatchOnly, "watch-only", o.WatchOnly, "Watch for changes to the requested object(s), without listing/getting first.")
	cmd.Flags().BoolVar(&o.IgnoreNotFound, "ignore-not-found", o.IgnoreNotFound, "If the requested object does not exist the command will return exit code 0.")
	cmd.Flags().StringVar(&o.FieldSelector, "field-selector", o.FieldSelector, "Selector (field query) to filter on, supports '=', '==', and '!='.(e.g. --field-selector key1=value1,key2=value2). The server only supports a limited number of field queries per type.")
	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespaces", "A", o.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmdutil.AddChunkSizeFlag(cmd, &o.ChunkSize)
	cmdutil.AddLabelSelectorFlagVar(cmd, &o.LabelSelector)
	cmdutil.AddSubresourceFlags(cmd, &o.Subresource, "If specified, gets the subresource of the requested object.")

	cmd.Flags().StringVarP(&o.Template, "template", "t", "default", "Template string to use when printing objects. Use \"\" to disable the template.")
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", "", "If present, the namespace scope for this CLI request. If not set, the current namespace in kubeconfig is used. Use --all-namespaces to ignore this flag.")
	cmd.Flags().StringVar(&o.Context, "context", "", "kubeconfig context to use for this CLI request. If not set, the current context in kubeconfig is used.")
	cmd.Flags().BoolVar(&o.EnableCustomTable, "ctable", false, "Enable custom table output. If set, the output will be formatted as a custom table based on the provided template.")
	return cmd
}

func multipleGVKsRequested(infos []*resource.Info) bool {
	if len(infos) < 2 {
		return false
	}
	gvk := infos[0].Mapping.GroupVersionKind
	for _, info := range infos {
		if info.Mapping.GroupVersionKind != gvk {
			return true
		}
	}
	return false
}
