package get

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
	"k8s.io/client-go/rest"
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
	Columns           []string
	Output            string
	SortColumn        string
	FilterExprs       []string

	factory cmdutil.Factory
	args    []string
}

// NewGetOptions returns a GetOptions with default chunk size 500.
func NewGetOptions(streams genericiooptions.IOStreams) *GetOptions {
	return &GetOptions{
		IOStreams:   streams,
		ChunkSize:  cmdutil.DefaultChunkSize,
		ServerPrint: true,
	}
}

// resolveTemplatePrinter finds the template file (.yaml first, then .tpl) and creates the appropriate printer.
// Shared helpers under <rootPath>/_shared/*.tpl are concatenated at the top of .tpl template bodies before parsing,
// and merged into the `helpers` field of .yaml templates.
func resolveTemplatePrinter(rootPath, crdTemplateDir, templateName string, decoder runtime.Decoder, restConfig *rest.Config) (*CustomColumnsPrinter, error) {
	dir := filepath.Join(rootPath, crdTemplateDir)
	sharedHelpers, _ := loadSharedHelpers(rootPath)

	// Try .yaml first
	yamlPath := filepath.Join(dir, templateName+".yaml")
	if data, err := os.ReadFile(yamlPath); err == nil {
		if sharedHelpers != "" {
			data = injectYAMLHelpers(data, sharedHelpers)
		}
		return NewCustomColumnsPrinterFromYAML(data, decoder, restConfig)
	}

	// Fall back to .tpl
	tplPath := filepath.Join(dir, templateName+".tpl")
	data, err := os.ReadFile(tplPath)
	if err != nil {
		return nil, fmt.Errorf("template not found (tried %s.yaml and %s.tpl in %s)", templateName, templateName, dir)
	}

	if sharedHelpers != "" {
		data = append([]byte(sharedHelpers+"\n"), data...)
	}
	return NewCustomColumnsPrinterFromTemplate(strings.NewReader(string(data)), decoder, restConfig)
}

// loadSharedHelpers concatenates every *.tpl file under <rootPath>/_shared/
// so `{{ template "name" . }}` calls from templates can resolve helpers
// defined once and reused everywhere.
func loadSharedHelpers(rootPath string) (string, error) {
	sharedDir := filepath.Join(rootPath, "_shared")
	entries, err := os.ReadDir(sharedDir)
	if err != nil {
		return "", err // no shared/ dir is fine, caller ignores the error
	}
	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".tpl") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sharedDir, e.Name()))
		if err != nil {
			continue
		}
		b.Write(data)
		b.WriteString("\n")
	}
	return b.String(), nil
}

// injectYAMLHelpers appends sharedHelpers to the YAML template's `helpers`
// field. If the field does not exist, it's created.
func injectYAMLHelpers(data []byte, sharedHelpers string) []byte {
	// Cheap textual merge: if the file already has a `helpers: |` block,
	// leave that alone (the YAML parser can only handle one). Otherwise
	// prepend a new one.
	if strings.Contains(string(data), "\nhelpers:") || strings.HasPrefix(string(data), "helpers:") {
		return data
	}
	// indent each helper line by 2 spaces for the block scalar
	indented := "  " + strings.ReplaceAll(sharedHelpers, "\n", "\n  ")
	block := "helpers: |\n" + indented + "\n"
	return append([]byte(block), data...)
}

// Complete resolves flags and sets up the factory.
func (o *GetOptions) Complete(cmd *cobra.Command, args []string) error {
	o.args = utils.ResolveAlias(args)

	rootPath, err := utils.ResolveTemplatePath(cmd)
	if err != nil {
		return fmt.Errorf("failed to resolve template path: %w", err)
	}
	o.TemplateRootPath = rootPath

	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag().WithDiscoveryBurst(300).WithDiscoveryQPS(50.0)

	if v := cmd.Flag("kubeconfig"); v != nil && v.Changed {
		kubeConfigFlags.KubeConfig = ptr.To(v.Value.String())
	}
	if o.Context != "" {
		kubeConfigFlags.Context = &o.Context
	}

	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	o.factory = cmdutil.NewFactory(matchVersionKubeConfigFlags)

	if o.Namespace == "" {
		o.Namespace, o.ExplicitNamespace, err = o.factory.ToRawKubeConfigLoader().Namespace()
		if err != nil {
			return fmt.Errorf("failed to resolve namespace: %w", err)
		}
	}

	if o.AllNamespaces {
		o.ExplicitNamespace = false
	}

	o.NoHeaders = cmdutil.GetFlagBool(cmd, "no-headers")

	return nil
}

// Validate checks that the resolved options are consistent.
func (o *GetOptions) Validate() error {
	if o.TemplateRootPath == "" {
		return fmt.Errorf("template path is required: use --template-path or run 'init' first")
	}
	if o.WatchOnly && o.Watch {
		return fmt.Errorf("--watch and --watch-only are mutually exclusive")
	}
	return nil
}

// Run is the cobra RunE entrypoint. It calls Complete, Validate, then executes.
func (o *GetOptions) Run(cmd *cobra.Command, args []string) error {
	if err := o.Complete(cmd, args); err != nil {
		return err
	}
	if err := o.Validate(); err != nil {
		return err
	}

	if o.Watch || o.WatchOnly {
		return o.watch()
	}

	return o.list()
}

func (o *GetOptions) list() error {
	r := o.buildRequest()

	if err := r.Err(); err != nil {
		return err
	}

	infos, err := r.Infos()
	if err != nil {
		return fmt.Errorf("failed to fetch resources: %w", err)
	}

	if len(infos) == 0 {
		fmt.Fprintf(o.ErrOut, "No resources found in %s namespace.\n", o.Namespace)
		return nil
	}

	printer, err := o.createPrinter(infos)
	if err != nil {
		return err
	}

	if o.Output != "" || o.SortColumn != "" || len(o.FilterExprs) > 0 {
		return o.emitStructured(printer, infos)
	}

	w := printers.GetNewTabWriter(os.Stdout)
	for _, info := range infos {
		if err := printer.PrintObj(info.Object, w); err != nil {
			return fmt.Errorf("failed to print object: %w", err)
		}
	}

	if printer.CustomTable != nil {
		printer.CustomTable.Render()
	} else {
		w.Flush()
	}

	return nil
}

func (o *GetOptions) emitStructured(printer *CustomColumnsPrinter, infos []*resource.Info) error {
	var rows [][]string
	printer.RowSink = func(cols []string) { rows = append(rows, cols) }
	for _, info := range infos {
		if err := printer.PrintObj(info.Object, io.Discard); err != nil {
			return fmt.Errorf("failed to render row: %w", err)
		}
	}

	if len(o.FilterExprs) > 0 {
		filtered, err := filterRows(printer.Headers, rows, o.FilterExprs)
		if err != nil {
			return err
		}
		rows = filtered
	}

	if o.SortColumn != "" {
		if err := sortRows(printer.Headers, rows, o.SortColumn); err != nil {
			return err
		}
	}

	if o.Output != "" {
		return renderRows(o.Out, o.Output, printer.Headers, rows)
	}

	// No explicit -o: render the standard table using our own writer.
	return renderRows(o.Out, "table", printer.Headers, rows)
}

func (o *GetOptions) watch() error {
	r := o.buildRequest()

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

	if len(infos) == 0 {
		fmt.Fprintf(o.ErrOut, "No resources found in %s namespace.\n", o.Namespace)
		return nil
	}

	printer, err := o.createPrinter(infos)
	if err != nil {
		return err
	}

	outputObjects := ptr.To(!o.WatchOnly)

	// print the current objects
	w := printers.GetNewTabWriter(os.Stdout)
	for _, info := range infos {
		if err := printer.PrintObj(info.Object, w); err != nil {
			return fmt.Errorf("failed to print object: %w", err)
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
	rv := "0"
	isList := meta.IsListType(obj)
	if isList {
		rv, err = meta.NewAccessor().ResourceVersion(obj)
		if err != nil {
			return err
		}
	}

	if isList {
		*outputObjects = true
	} else {
		*outputObjects = false
	}

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
			*outputObjects = true
			return false, nil
		})
		return err
	})
	return nil
}

func (o *GetOptions) buildRequest() *resource.Result {
	return o.factory.NewBuilder().
		Unstructured().
		DefaultNamespace().
		NamespaceParam(o.Namespace).
		AllNamespaces(o.AllNamespaces).
		FilenameParam(o.ExplicitNamespace, &o.FilenameOptions).
		LabelSelectorParam(o.LabelSelector).
		FieldSelectorParam(o.FieldSelector).
		Subresource(o.Subresource).
		RequestChunksOf(o.ChunkSize).
		ResourceTypeOrNameArgs(true, o.args...).
		ContinueOnError().
		Latest().
		Flatten().
		Do()
}

func (o *GetOptions) createPrinter(infos []*resource.Info) (*CustomColumnsPrinter, error) {
	crdTemplateDir := utils.GenerateDirNameByGVK(infos[0].Object.GetObjectKind().GroupVersionKind())

	decoder := scheme.Codecs.UniversalDecoder(scheme.Scheme.PrioritizedVersionsAllGroups()...)

	restConfig, err := o.factory.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get REST config: %w", err)
	}

	printer, err := resolveTemplatePrinter(o.TemplateRootPath, crdTemplateDir, o.Template, decoder, restConfig)
	if err != nil {
		return nil, err
	}

	if len(o.Columns) > 0 {
		if err := printer.SelectColumns(o.Columns); err != nil {
			return nil, err
		}
	}

	if o.EnableCustomTable {
		printer.WithCustomTable()
	}
	printer.NoHeaders = o.NoHeaders

	return printer, nil
}

func NewCmdGet(streams genericiooptions.IOStreams) *cobra.Command {
	o := NewGetOptions(streams)

	cmd := &cobra.Command{
		Use:        "get TYPE [NAME ...] [flags]",
		Aliases:    []string{"g"},
		SuggestFor: []string{"list", "show", "fetch", "describe"},
		Short:      "Display resources in custom wide output format",
		Long: `Display one or more resources using custom column templates.

Templates are resolved from the template root directory (set via --template-path or
the config file created by 'init'). For each resource kind, cwide looks for a template
in <root>/<kind>-<group>-<version>/<template>.yaml (falling back to .tpl).`,
		Example: `  # List pods using the default template
  kubectl cwide get pods

  # Get a specific pod in a namespace
  kubectl cwide get pod my-pod -n my-namespace

  # Use a custom template
  kubectl cwide get deployments -t my-template

  # Watch pods
  kubectl cwide get pods -w

  # List across all namespaces
  kubectl cwide get pods -A`,
		Args: cobra.MinimumNArgs(1),
		RunE: o.Run,
	}

	cmd.AddCommand(NewCmdGetAllResources(streams))

	cmd.Flags().BoolVar(&o.NoHeaders, "no-headers", o.NoHeaders, "When using the default or custom-column output format, don't print headers (default print headers).")
	cmd.Flags().StringVar(&o.Raw, "raw", o.Raw, "Raw URI to request from the server. Uses the transport specified by the kubeconfig file.")
	cmd.Flags().BoolVarP(&o.Watch, "watch", "w", o.Watch, "After listing/getting the requested object, watch for changes.")
	cmd.Flags().BoolVar(&o.WatchOnly, "watch-only", o.WatchOnly, "Watch for changes to the requested object(s), without listing/getting first.")
	cmd.Flags().BoolVar(&o.IgnoreNotFound, "ignore-not-found", o.IgnoreNotFound, "If the requested object does not exist the command will return exit code 0.")
	cmd.Flags().StringVar(&o.FieldSelector, "field-selector", o.FieldSelector, "Selector (field query) to filter on, supports '=', '==', and '!='.(e.g. --field-selector key1=value1,key2=value2). The server only supports a limited number of field queries per type.")
	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespaces", "A", o.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmdutil.AddChunkSizeFlag(cmd, &o.ChunkSize)
	cmdutil.AddLabelSelectorFlagVar(cmd, &o.LabelSelector)
	cmdutil.AddSubresourceFlags(cmd, &o.Subresource, "If specified, gets the subresource of the requested object.")

	cmd.Flags().StringVarP(&o.Template, "template", "t", "default", "Name of the column template to use (without extension).")
	cmd.Flags().StringSliceVarP(&o.Columns, "columns", "c", nil, "Comma-separated list of column headers to display (subset of the template's columns, case-insensitive).")
	cmd.Flags().StringVarP(&o.Output, "output", "o", "", "Output format. One of: json, yaml, csv. If empty, prints the standard table.")
	cmd.Flags().StringVar(&o.SortColumn, "sort-by", "", "Column header to sort rows by (case-insensitive). Numeric strings sort numerically.")
	cmd.Flags().StringArrayVar(&o.FilterExprs, "filter", nil, "Filter rows by column values: COL=val, COL!=val, COL~regex, COL!~regex (repeatable, ANDed).")
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", "", "If present, the namespace scope for this CLI request.")
	cmd.Flags().StringVar(&o.Context, "context", "", "The name of the kubeconfig context to use.")
	cmd.Flags().BoolVar(&o.EnableCustomTable, "ctable", false, "Enable custom table output with borders.")
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
