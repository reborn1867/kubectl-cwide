package clients

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/utils/ptr"
)

// FactoryFromCmd builds a cmdutil.Factory using the standard kubeconfig
// discovery, honoring --kubeconfig and --context flags from the given cobra
// command (both optional).
//
// This is the single place cwide constructs Kubernetes client factories so all
// subcommands share the same discovery cache and TLS/QPS settings.
func FactoryFromCmd(cmd *cobra.Command, kubeContext string) cmdutil.Factory {
	kubeConfigFlags := genericclioptions.NewConfigFlags(true).
		WithDeprecatedPasswordFlag().
		WithDiscoveryBurst(300).
		WithDiscoveryQPS(50.0)

	if cmd != nil {
		if v := cmd.Flag("kubeconfig"); v != nil && v.Changed {
			kubeConfigFlags.KubeConfig = ptr.To(v.Value.String())
		}
	}
	if kubeContext != "" {
		kubeConfigFlags.Context = &kubeContext
	}

	matchVersionFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	return cmdutil.NewFactory(matchVersionFlags)
}
