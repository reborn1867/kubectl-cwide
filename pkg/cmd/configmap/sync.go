package configmap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

func NewCmdSync() *cobra.Command {
	var force bool

	syncCMD := &cobra.Command{
		Use:        "sync",
		Aliases:    []string{"pull"},
		SuggestFor: []string{"download", "fetch"},
		Short:      "Pull templates from a ConfigMap into the local directory",
		Long: `Sync templates stored in a Kubernetes ConfigMap to the local template directory.

Each ConfigMap data key should be in the format "<resource-dir>..<template-name>"
(e.g. "pod--v1..debug"). The value is the YAML template content.

Whether existing local files are overwritten depends on the 'templateSources'
order in the config file:
  - ["local", "configmap"]  — local files take priority, existing files are skipped
  - ["configmap", "local"]  — configmap takes priority, existing files are overwritten

Use --force to always overwrite regardless of priority.`,
		Example: `  # Sync templates from the default ConfigMap
  kubectl cwide configmap sync

  # Sync from a specific ConfigMap
  kubectl cwide configmap sync --name my-templates --cm-namespace default

  # Force overwrite all local templates
  kubectl cwide configmap sync --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cmName := cmd.Flag("name").Value.String()
			cmNamespace := cmd.Flag("cm-namespace").Value.String()

			absPath, err := utils.ResolveTemplatePath(cmd)
			if err != nil {
				return fmt.Errorf("failed to resolve template path: %w", err)
			}

			config, err := ctrl.GetConfig()
			if err != nil {
				return fmt.Errorf("failed to load kubeconfig: %w", err)
			}

			clientset, err := kubernetes.NewForConfig(config)
			if err != nil {
				return fmt.Errorf("failed to create Kubernetes client: %w", err)
			}

			cm, err := clientset.CoreV1().ConfigMaps(cmNamespace).Get(ctx, cmName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get ConfigMap %s/%s: %w", cmNamespace, cmName, err)
			}

			if len(cm.Data) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "ConfigMap has no template data.")
				return nil
			}

			// Load config for priority
			sources := loadTemplateSources()

			synced := 0
			skipped := 0
			aliasesSynced := 0
			for key, value := range cm.Data {
				if key == aliasesConfigMapKey {
					n, err := mergeRemoteAliases([]byte(value), force)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Skipping %s: %v\n", key, err)
						continue
					}
					aliasesSynced = n
					continue
				}
				parts := strings.SplitN(key, "..", 2)
				if len(parts) != 2 {
					fmt.Fprintf(cmd.ErrOrStderr(), "Skipping invalid key %q (expected <resource-dir>..<template-name>)\n", key)
					continue
				}

				resourceDir := parts[0]
				templateName := parts[1]
				localDir := filepath.Join(absPath, resourceDir)
				localPath := filepath.Join(localDir, templateName+".yaml")

				localExists := utils.CheckFileExists(localPath)
				if !shouldOverwrite(sources, localExists, force) {
					skipped++
					continue
				}

				if err := os.MkdirAll(localDir, 0755); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", localDir, err)
				}
				if err := os.WriteFile(localPath, []byte(value), 0644); err != nil {
					return fmt.Errorf("failed to write %s: %w", localPath, err)
				}
				synced++
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Synced %d template(s), skipped %d.\n", synced, skipped)
			if aliasesSynced > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Merged %d alias(es) from ConfigMap.\n", aliasesSynced)
			}
			return nil
		},
	}

	syncCMD.Flags().BoolVar(&force, "force", false, "Overwrite existing local files regardless of priority")

	return syncCMD
}

// shouldOverwrite determines whether a configmap template should overwrite a local file.
func shouldOverwrite(sources []string, localExists bool, force bool) bool {
	if force {
		return true
	}
	if !localExists {
		return true
	}
	for _, s := range sources {
		switch s {
		case "configmap":
			return true
		case "local":
			return false
		}
	}
	return false
}

// mergeRemoteAliases merges a YAML alias map (name → target) from the
// ConfigMap into the local config. Existing local aliases are preserved
// unless `force` is true, in which case remote wins.
// Returns the number of aliases added or updated.
func mergeRemoteAliases(raw []byte, force bool) (int, error) {
	remote := map[string]string{}
	if err := yaml.Unmarshal(raw, &remote); err != nil {
		return 0, fmt.Errorf("parse aliases yaml: %w", err)
	}
	if len(remote) == 0 {
		return 0, nil
	}
	cfg, err := utils.LoadConfig()
	if err != nil {
		return 0, fmt.Errorf("load config: %w", err)
	}
	if cfg.Aliases == nil {
		cfg.Aliases = map[string]string{}
	}
	n := 0
	for name, target := range remote {
		if _, exists := cfg.Aliases[name]; exists && !force {
			continue
		}
		cfg.Aliases[name] = target
		n++
	}
	if n == 0 {
		return 0, nil
	}
	if err := utils.SaveConfig(cfg); err != nil {
		return 0, fmt.Errorf("save config: %w", err)
	}
	return n, nil
}

// loadTemplateSources reads the templateSources from the config file.
// Returns the default ["local", "configmap"] if not configured.
func loadTemplateSources() []string {
	cfg, err := utils.LoadConfig()
	if err != nil {
		return []string{"local", "configmap"}
	}
	if len(cfg.TemplateSources) == 0 {
		return []string{"local", "configmap"}
	}
	return cfg.TemplateSources
}
