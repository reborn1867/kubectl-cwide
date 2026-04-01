package initialization

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	printersinternal "k8s.io/kubernetes/pkg/printers/internalversion"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubectl-cwide/pkg/common"
	"github.com/kubectl-cwide/pkg/models"
	"github.com/kubectl-cwide/pkg/utils"
)

func NewCmdInit() *cobra.Command {
	initCMD := &cobra.Command{
		Use:   "init",
		Short: "Initialize column templates for all cluster resources",
		Long: `Generate column templates for every CRD and built-in resource discovered in
the target cluster. Templates are written as YAML files under the directory
specified by --template-path. A config file is saved at ~/.kubectl-cwide/config.yaml
so that subsequent commands can find the template directory automatically.

Existing template files are preserved and will not be overwritten.`,
		Example: `  # Initialize templates in the default directory
  kubectl cwide init

  # Initialize templates in a custom directory
  kubectl cwide init --template-path ~/my-templates

  # Use a specific kubeconfig
  kubectl cwide init --kubeconfig /path/to/kubeconfig`,
		RunE: runInit,
	}
	return initCMD
}

func runInit(cmd *cobra.Command, args []string) error {
	config, err := ctrl.GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	k8sClient, err := client.New(config, client.Options{})
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	clientSet, err := apiextensionsclientset.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create apiextensions client: %w", err)
	}

	if err := apiextensions.AddToScheme(k8sClient.Scheme()); err != nil {
		return fmt.Errorf("failed to add apiextensions to scheme: %w", err)
	}
	if err := v1.AddToScheme(k8sClient.Scheme()); err != nil {
		return fmt.Errorf("failed to add apiextensions v1 to scheme: %w", err)
	}

	crdList, err := clientSet.ApiextensionsV1().CustomResourceDefinitions().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list CRDs: %w", err)
	}

	path, err := utils.ResolveTemplatePath(cmd)
	if err != nil {
		// flag wasn't changed and no config exists — use the default
		path = cmd.Flag("template-path").Value.String()
		path, err = filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}
	}

	configRaw, err := yaml.Marshal(&models.Config{TemplatePath: path})
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	if err := utils.CreateFileIfNotExists(filepath.Join(homeDir, common.ConfigPath), configRaw); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Initializing template directory at: %s\n", path)

	for _, crd := range crdList.Items {
		for _, v := range crd.Spec.Versions {
			crdTemplateDir := filepath.Join(path, utils.GenerateDirNameByGVK(schema.GroupVersionKind{
				Group:   crd.Spec.Group,
				Version: v.Name,
				Kind:    crd.Spec.Names.Kind,
			}))

			columns := []v1.CustomResourceColumnDefinition{
				{
					Name:     "Name",
					JSONPath: ".metadata.name",
				},
			}
			columns = append(columns, v.AdditionalPrinterColumns...)

			yamlContent, err := utils.BuildYAMLColumnTemplate(columns)
			if err != nil {
				return fmt.Errorf("failed to build YAML template for %s: %w", crd.Spec.Names.Kind, err)
			}

			if err := utils.CreateOrFormatYAMLFile(filepath.Join(crdTemplateDir, "default.yaml"), yamlContent); err != nil {
				return fmt.Errorf("failed to write template for %s: %w", crd.Spec.Names.Kind, err)
			}
		}
	}

	_, resourceLists, err := clientSet.Discovery().ServerGroupsAndResources()
	if err != nil {
		return fmt.Errorf("failed to discover server resources: %w", err)
	}

	tableGenerator := utils.NewTableGenerator().With(printersinternal.AddHandlers)

	for _, resourceList := range resourceLists {
		var group, version string
		groupVersion := strings.Split(resourceList.GroupVersion, "/")
		if len(groupVersion) == 2 {
			group = groupVersion[0]
			version = groupVersion[1]
		} else {
			version = groupVersion[0]
		}
		for _, resource := range resourceList.APIResources {
			colDefinition := tableGenerator.ResourceColumnDefinition(strings.ToLower(resource.Kind))
			if len(colDefinition) != 0 {
				defaultResourceTemplateDir := filepath.Join(path, utils.GenerateDirNameByGVK(schema.GroupVersionKind{
					Group:   group,
					Version: version,
					Kind:    resource.Kind,
				}))

				yamlContent, err := utils.BuildYAMLTableColumnTemplate(colDefinition)
				if err != nil {
					return fmt.Errorf("failed to build YAML template for %s: %w", resource.Kind, err)
				}

				if err := utils.CreateOrFormatYAMLFile(filepath.Join(defaultResourceTemplateDir, "default.yaml"), yamlContent); err != nil {
					return fmt.Errorf("failed to write template for %s: %w", resource.Kind, err)
				}
			}
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Initialization complete.\n")
	return nil
}
