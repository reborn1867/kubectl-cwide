package get

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/kubectl-cwide/pkg/common"
	"github.com/kubectl-cwide/pkg/models"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"
)

var GetCMD = &cobra.Command{
	Use:   "get",
	Short: "get k8s resources in custom wide output format",
	RunE: func(cmd *cobra.Command, args []string) error {
		resourceKind := args[0]
		// resourceName := args[1]

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

		crdPath := filepath.Join(rootPath, fmt.Sprintf("%s*", resourceKind), "default.yaml")
		cwideTemplatePath := findFileWithWildCard(crdPath)
		if cwideTemplatePath == "" {
			return fmt.Errorf("template not found for resource kind %s\n", resourceKind)
		}
		file, err := os.Open(cwideTemplatePath)
		if err != nil {
			return fmt.Errorf("error reading template %s, %v\n", cwideTemplatePath, err)
		}

		decoder := scheme.Codecs.UniversalDecoder(scheme.Scheme.PrioritizedVersionsAllGroups()...)

		printer, err := get.NewCustomColumnsPrinterFromTemplate(file, decoder)
		if err != nil {
			return fmt.Errorf("error creating printer from template %v\n", err)
		}

		kubeConfigFlags := genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag().WithDiscoveryBurst(300).WithDiscoveryQPS(50.0)
		matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
		f := cmdutil.NewFactory(matchVersionKubeConfigFlags)
		r := f.NewBuilder().
			Unstructured().
			DefaultNamespace().
			// NamespaceParam(o.Namespace).
			// AllNamespaces(o.AllNamespaces).
			// FilenameParam(o.ExplicitNamespace, &o.FilenameOptions).
			// LabelSelectorParam(o.LabelSelector).
			// FieldSelectorParam(o.FieldSelector).
			// Subresource(o.Subresource).
			// RequestChunksOf(chunkSize).
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

		objs := make([]runtime.Object, len(infos))
		for ix := range infos {
			objs[ix] = infos[ix].Object
		}

		for _, obj := range objs {
			printer.PrintObj(obj, os.Stdout)
		}

		return nil
	},
}

// covert resourcekind to complete crd name
func convertResourceKindToCRDName(resourceKind string) string {
	// TODO
	return ""
}

// find file with wild card path
func findFileWithWildCard(path string) string {
	matches, err := filepath.Glob(path)
	if err != nil {
		log.Println("Error finding files:", err)
		return ""
	}
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

// get template path from config.yaml
func getTemplatePathFromConfig() (string, error) {

	// Read the configuration file
	configFile, err := os.ReadFile(common.ConfigPath)
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
