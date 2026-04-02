package configmap

import (
	"github.com/kubectl-cwide/pkg/common"
	"github.com/spf13/cobra"
)

func NewCmdConfigMap() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "configmap",
		Short: "Sync templates with a Kubernetes ConfigMap",
		Long: `Manage column templates stored in a Kubernetes ConfigMap.

Use 'sync' to pull templates from a ConfigMap into the local template directory,
and 'push' to upload local templates into the ConfigMap.

The ConfigMap uses a single data key per template in the format
"<resource-dir>/<template-name>" (e.g. "pod--v1/debug"). Template priority
(local vs configmap) is controlled by the 'templateSources' list in the config file.`,
		RunE: func(c *cobra.Command, args []string) error {
			return c.Help()
		},
	}

	cmd.PersistentFlags().String("name", common.DefaultConfigMapName, "Name of the ConfigMap")
	cmd.PersistentFlags().String("cm-namespace", common.DefaultConfigMapNamespace, "Namespace of the ConfigMap")

	cmd.AddCommand(NewCmdSync())
	cmd.AddCommand(NewCmdPush())

	return cmd
}
