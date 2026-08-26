package configmap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
)

// aliasesConfigMapKey is the data-map key used to store a YAML-marshalled
// alias map (name → target) so aliases can be shared across a team via the
// same ConfigMap that carries templates.
const aliasesConfigMapKey = "__aliases__"

func NewCmdPush() *cobra.Command {
	var resource string
	var includeAliases bool
	var files []string
	var keyOverride string

	pushCMD := &cobra.Command{
		Use:        "push",
		SuggestFor: []string{"upload", "publish", "import"},
		Short:      "Push local templates into a Kubernetes ConfigMap",
		Long: `Upload local template files into a Kubernetes ConfigMap.

Each template file is stored as a data key in the format
"<resource-dir>..<template-name>" (e.g. "pod--v1..debug"). If the ConfigMap
does not exist, it is created.

Three ways to select what gets pushed:
  1. No flags — every template under the template root
  2. -r <resource> — every template for that resource type
  3. -f <file> [-f <file> ...] — one or more specific files (paths can live
     ANYWHERE on disk, not just under the template root). Use --key to override
     the derived ConfigMap key when the file's path can't be parsed as
     <resource-dir>/<template>.yaml. -f is the "import from local file" flow.`,
		Example: `  # Push everything under the template root
  kubectl cwide configmap push

  # Push only pod templates
  kubectl cwide configmap push -r pod

  # Import a single file that lives outside the template root
  kubectl cwide configmap push -f ~/downloads/pod-debug.yaml --key pod--v1..debug

  # Import multiple files (keys derived from parent-dir/basename)
  kubectl cwide configmap push -f pod--v1/debug.yaml -f service--v1/wide.yaml

  # Push to a specific ConfigMap
  kubectl cwide configmap push --name my-templates --cm-namespace default`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cmName := cmd.Flag("name").Value.String()
			cmNamespace := cmd.Flag("cm-namespace").Value.String()

			// Collect local template files
			data := make(map[string]string)

			// Path A: explicit -f files (import-from-disk flow).
			if len(files) > 0 {
				if len(files) > 1 && keyOverride != "" {
					return fmt.Errorf("--key can only be used with a single -f file")
				}
				for _, f := range files {
					key, err := deriveKeyForFile(f, keyOverride)
					if err != nil {
						return err
					}
					content, err := os.ReadFile(f)
					if err != nil {
						return fmt.Errorf("read %s: %w", f, err)
					}
					data[key] = string(content)
				}
			} else {
				// Path B: template-root scan.
				absPath, err := utils.ResolveTemplatePath(cmd)
				if err != nil {
					return fmt.Errorf("failed to resolve template path: %w", err)
				}

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
			}

			if includeAliases {
				cfg, err := utils.LoadConfig()
				if err == nil && len(cfg.Aliases) > 0 {
					if raw, err := yaml.Marshal(cfg.Aliases); err == nil {
						data[aliasesConfigMapKey] = string(raw)
					}
				}
			}

			if len(data) == 0 {
				if resource != "" {
					return fmt.Errorf("no templates found for resource type %q", resource)
				}
				if len(files) > 0 {
					return fmt.Errorf("no readable files among the -f arguments")
				}
				return fmt.Errorf("no templates found — check --template-path or run 'kubectl cwide init'")
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

			existing, err := cmClient.Get(ctx, cmName, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				// Create the ConfigMap
				cm := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      cmName,
						Namespace: cmNamespace,
					},
					Data: data,
				}
				if _, err := cmClient.Create(ctx, cm, metav1.CreateOptions{}); err != nil {
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
			if _, err := cmClient.Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("failed to update ConfigMap: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated ConfigMap %s/%s with %d template(s).\n", cmNamespace, cmName, len(data))
			return nil
		},
	}

	pushCMD.Flags().StringVarP(&resource, "resource", "r", "", "Only push templates for this resource type (e.g. pod, deployment). Ignored when -f is passed.")
	pushCMD.Flags().StringArrayVarP(&files, "file", "f", nil, "Local YAML file to push. Repeatable. Derives ConfigMap key from parent-dir/basename; use --key to override.")
	pushCMD.Flags().StringVar(&keyOverride, "key", "", "Explicit ConfigMap data key for the single -f file. Format: <resource-dir>..<template-name> (e.g. pod--v1..debug).")
	pushCMD.Flags().BoolVar(&includeAliases, "with-aliases", false, "Also push resource aliases from local config under the reserved key "+aliasesConfigMapKey)

	return pushCMD
}

// deriveKeyForFile turns a local file path into the ConfigMap data-key format
// `<resource-dir>..<template-name>` that cwide's sync expects. An explicit
// override (from --key) short-circuits derivation but is validated against the
// expected format so users don't accidentally break sync compatibility.
//
// Derivation rules:
//   - If the parent directory looks like a cwide resource dir (contains a "-"
//     as in "pod--v1" or "deployment-apps-v1"), use it: <dir>..<basename>
//   - Otherwise, require --key.
func deriveKeyForFile(path, override string) (string, error) {
	if override != "" {
		if !strings.Contains(override, "..") {
			return "", fmt.Errorf("--key %q must contain '..' (format: <resource-dir>..<template-name>, e.g. pod--v1..debug)", override)
		}
		return override, nil
	}

	base := filepath.Base(path)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	if name == base {
		return "", fmt.Errorf("file %q has no extension; use --key to name the entry", path)
	}
	dir := filepath.Base(filepath.Dir(path))
	if dir == "." || dir == "" || !strings.Contains(dir, "-") {
		return "", fmt.Errorf(
			"cannot derive ConfigMap key for %q: parent dir %q isn't a cwide resource dir. "+
				"Pass --key <resource-dir>..<template-name> (e.g. --key pod--v1..%s)",
			path, dir, name)
	}
	return dir + ".." + name, nil
}
