package initialization

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kubectl-cwide/pkg/common"
	"github.com/kubectl-cwide/pkg/models"
	"github.com/kubectl-cwide/pkg/utils"
)

type CRDProperty struct {
	Names v1.CustomResourceDefinitionNames `json:"names"`
}

var InitCMD = &cobra.Command{
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

		if err := utils.CreateTempDir(path); err != nil {
			return fmt.Errorf("failed to create temp directory: %v", err)
		}

		b, err := yaml.Marshal(&models.Config{TemplatePath: path})
		if err != nil {
			return fmt.Errorf("failed to marshal config: %v", err)
		}

		if err := utils.CreateTempDir(filepath.Dir(common.ConfigPath)); err != nil {
			return fmt.Errorf("failed to create temp directory: %v", err)
		}

		if err := utils.CreateOrUpdateFile(common.ConfigPath, b); err != nil {
			return fmt.Errorf("failed to create or update file: %v", err)
		}

		for _, crd := range crdList.Items {
			for _, v := range crd.Spec.Versions {
				if err := utils.CreateTempDir(fmt.Sprintf("%s/%s-%s", path, crd.Name, v.Name)); err != nil {
					return fmt.Errorf("failed to create temp directory: %v", err)
				}
				if err := utils.CreateOrUpdateFile(fmt.Sprintf("%s/%s-%s/default.yaml", path, crd.Name, v.Name), utils.BuildColumnTemplate(v.AdditionalPrinterColumns)); err != nil {
					return fmt.Errorf("failed to create or update file: %v", err)
				}

				b, err := yaml.Marshal(CRDProperty{Names: crd.Spec.Names})
				if err != nil {
					return fmt.Errorf("failed to marshal crd property: %v", err)
				}

				if err := utils.CreateOrUpdateFile(fmt.Sprintf("%s/%s-%s/property.yaml", path, crd.Name, v.Name), b); err != nil {
					return fmt.Errorf("failed to create or update file: %v", err)
				}
			}
		}
		return nil
	},
}
