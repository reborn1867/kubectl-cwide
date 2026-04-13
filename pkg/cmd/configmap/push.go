package configmap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

func NewCmdPush() *cobra.Command {
	var resource string

	pushCMD := &cobra.Command{
		Use:        "push",
		SuggestFor: []string{"upload", "publish"},
		Short:      "Push local templates into a Kubernetes ConfigMap",
		Long: `Upload local template files into a Kubernetes ConfigMap.

Each template file is stored as a data key in the format
"<resource-dir>..<template-name>" (e.g. "pod--v1..debug"). If the ConfigMap
does not exist, it is created.

By default all templates under the template root are pushed. Use -r to push
only templates for a specific resource type.`,
		Example: `  # Push all local templates to the ConfigMap
  kubectl cwide configmap push

  # Push only pod templates
  kubectl cwide configmap push -r pod

  # Push to a specific ConfigMap
  kubectl cwide configmap push --name my-templates --cm-namespace default`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmName := cmd.Flag("name").Value.String()
			cmNamespace := cmd.Flag("cm-namespace").Value.String()

			absPath, err := utils.ResolveTemplatePath(cmd)
			if err != nil {
				return fmt.Errorf("failed to resolve template path: %w", err)
			}

			// Collect local template files
			data := make(map[string]string)

			pattern := "*/*.yaml"
			if resource != "" {
				pattern = strings.ToLower(resource) + "-*/*.yaml"
			}

			matches, err := filepath.Glob(filepath.Join(absPath, pattern))
			if err != nil {
				return fmt.Errorf("failed to search for templates: %w", err)
			}

			for _, match := range matches {
				rel, err := filepath.Rel(absPath, match)
				if err != nil {
					continue
				}
				dir := filepath.Dir(rel)
				name := strings.TrimSuffix(filepath.Base(rel), ".yaml")
				key := dir + ".." + name

				content, err := os.ReadFile(match)
				if err != nil {
					return fmt.Errorf("failed to read %s: %w", match, err)
				}
				data[key] = string(content)
			}

			if len(data) == 0 {
				if resource != "" {
					return fmt.Errorf("no templates found for resource type %q", resource)
				}
				return fmt.Errorf("no templates found in %s", absPath)
			}

			config, err := ctrl.GetConfig()
			if err != nil {
				return fmt.Errorf("failed to load kubeconfig: %w", err)
			}

			clientset, err := kubernetes.NewForConfig(config)
			if err != nil {
				return fmt.Errorf("failed to create Kubernetes client: %w", err)
			}

			cmClient := clientset.CoreV1().ConfigMaps(cmNamespace)

			existing, err := cmClient.Get(context.TODO(), cmName, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				// Create the ConfigMap
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cmName,
						Namespace: cmNamespace,
					},
					Data: data,
				}
				if _, err := cmClient.Create(context.TODO(), cm, metav1.CreateOptions{}); err != nil {
					return fmt.Errorf("failed to create ConfigMap: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Created ConfigMap %s/%s with %d template(s).\n", cmNamespace, cmName, len(data))
				return nil
			} else if err != nil {
				return fmt.Errorf("failed to get ConfigMap %s/%s: %w", cmNamespace, cmName, err)
			}

			// Update the existing ConfigMap
			if existing.Data == nil {
				existing.Data = make(map[string]string)
			}
			for k, v := range data {
				existing.Data[k] = v
			}
			if _, err := cmClient.Update(context.TODO(), existing, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("failed to update ConfigMap: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated ConfigMap %s/%s with %d template(s).\n", cmNamespace, cmName, len(data))
			return nil
		},
	}

	pushCMD.Flags().StringVarP(&resource, "resource", "r", "", "Only push templates for this resource type (e.g. pod, deployment)")

	return pushCMD
}
