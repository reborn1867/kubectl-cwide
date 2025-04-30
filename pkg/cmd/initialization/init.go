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
		Short: "init cwide template",
		RunE: func(cmd *cobra.Command, args []string) error {
			config := ctrl.GetConfigOrDie()

			k8sClient, err := client.New(config, client.Options{})
			if err != nil {
				return err
			}

			clientSet := apiextensionsclientset.NewForConfigOrDie(config)

			apiextensions.AddToScheme(k8sClient.Scheme())
			v1.AddToScheme(k8sClient.Scheme())

			crdList, err := clientSet.ApiextensionsV1().CustomResourceDefinitions().List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				return err
			}

			path := cmd.Flag("template-path").Value.String()

			absPath, err := filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("failed to get absolute path: %v", err)
			}

			configRaw, err := yaml.Marshal(&models.Config{TemplatePath: absPath})
			if err != nil {
				return fmt.Errorf("failed to marshal config: %v", err)
			}

			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %v", err)
			}

			if err := utils.CreateFileIfNotExits(filepath.Join(homeDir, common.ConfigPath), configRaw); err != nil {
				return fmt.Errorf("failed to create or update config file: %v", err)
			}

			fmt.Printf("Initializing template directory at : %s\n", path)

			for _, crd := range crdList.Items {
				for _, v := range crd.Spec.Versions {
					crdTemplateDir := filepath.Join(path, utils.GenerateDirNameByGVK(schema.GroupVersionKind{
						Group:   crd.Spec.Group,
						Version: v.Name,
						Kind:    crd.Spec.Names.Kind,
					}))

					// add resource name as first column
					columns := []v1.CustomResourceColumnDefinition{
						{
							Name:     "Name",
							JSONPath: ".metadata.name",
						},
					}
					columns = append(columns, v.AdditionalPrinterColumns...)

					if err := utils.CreateOrFormatFile(filepath.Join(crdTemplateDir, "default.tpl"), utils.BuildColumnTemplate(columns)); err != nil {
						return fmt.Errorf("failed to create or update template file: %v", err)
					}

				}
			}

			_, resourceLists, err := clientSet.Discovery().ServerGroupsAndResources()
			if err != nil {
				return fmt.Errorf("failed to get server groups and resources: %v", err)
			}

			tableGenerator := utils.NewTableGenerator().With(printersinternal.AddHandlers)

			for _, resourceList := range resourceLists {
				var group, version string
				groupVersion := strings.Split(resourceList.GroupVersion, "/")
				group = strings.Split(resourceList.GroupVersion, "/")[0]
				if len(groupVersion) == 2 {
					version = strings.Split(resourceList.GroupVersion, "/")[1]
				}
				for _, resource := range resourceList.APIResources {
					colDefinition := tableGenerator.ResourceColumnDefinition(strings.ToLower(resource.Kind))
					if len(colDefinition) != 0 {
						fmt.Printf("Found resource: %s-%s-%s\n", resource.Kind, group, version)
						defaultResourceTemplateDir := filepath.Join(path, utils.GenerateDirNameByGVK(schema.GroupVersionKind{
							Group:   group,
							Version: version,
							Kind:    resource.Kind,
						}))

						if err := utils.CreateOrFormatFile(filepath.Join(defaultResourceTemplateDir, "default.tpl"), utils.BuildTableColumnTemplate(colDefinition)); err != nil {
							return fmt.Errorf("failed to create or update template file: %v", err)
						}
					}
				}
			}
			return nil
		},
	}
	return initCMD
}
