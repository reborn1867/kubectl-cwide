package get

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kubectl-cwide/pkg/common"
	"github.com/kubectl-cwide/pkg/models"
	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/printers"

	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"
)

type GetOptions struct {
	Raw       string
	Watch     bool
	WatchOnly bool
	ChunkSize int64

	LabelSelector     string
	FieldSelector     string
	AllNamespaces     bool
	Namespace         string
	ExplicitNamespace bool
	Subresource       string
	SortBy            string

	ServerPrint bool

	NoHeaders      bool
	IgnoreNotFound bool

	genericiooptions.IOStreams
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
		rootPath, err = getTemplatePathFromConfig()
		if err != nil {
			return err
		}
	}

	if o.Watch || o.WatchOnly {
		// TODO : add watch support
	}

	kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag().WithDiscoveryBurst(300).WithDiscoveryQPS(50.0)
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)

	o.Namespace, o.ExplicitNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
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
		// FilenameParam(o.ExplicitNamespace, &o.FilenameOptions).
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
	crdTemplateDir = utils.GetCRDDirName(infos[0].Object.GetObjectKind().GroupVersionKind())

	file, err := os.Open(filepath.Join(rootPath, crdTemplateDir, "default.yaml"))
	if err != nil {
		return fmt.Errorf("error reading template %s, %v\n", filepath.Join(rootPath, crdTemplateDir, "default.yaml"), err)
	}

	decoder := scheme.Codecs.UniversalDecoder(scheme.Scheme.PrioritizedVersionsAllGroups()...)

	printer, err := NewCustomColumnsPrinterFromTemplate(file, decoder)
	if err != nil {
		return fmt.Errorf("error creating printer from template %v\n", err)
	}

	printer.NoHeaders = o.NoHeaders

	w := printers.GetNewTabWriter(os.Stdout)
	for _, info := range infos {
		printer.PrintObj(info.Object, w)
	}
	w.Flush()

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

	return cmd
}

// get template path from config.yaml
func getTemplatePathFromConfig() (string, error) {

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %v", err)
	}

	// Read the configuration file
	configFile, err := os.ReadFile(filepath.Join(homeDir, common.ConfigPath))
	if err != nil {
		return "", err
	}

	// Parse the configuration file
	var config models.Config
	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		return "", err
	}

	// Check if the template path is set
	if config.TemplatePath == "" {
		return "", errors.New("template path not found in configuration")
	}

	return config.TemplatePath, nil
}
