package cmd

import (
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/kubectl-cwide/pkg/cmd/alias"
	"github.com/kubectl-cwide/pkg/cmd/config"
	configmapCmd "github.com/kubectl-cwide/pkg/cmd/configmap"
	"github.com/kubectl-cwide/pkg/cmd/get"
	"github.com/kubectl-cwide/pkg/cmd/initialization"
	"github.com/kubectl-cwide/pkg/cmd/list"
	"github.com/kubectl-cwide/pkg/cmd/marketplace"
	"github.com/kubectl-cwide/pkg/cmd/template"
	"github.com/kubectl-cwide/pkg/cmd/tree"
)

func NewCmdCwide(streams genericiooptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubectl-cwide",
		Short: "Custom wide output format for kubectl",
		Long: `kubectl-cwide is a kubectl plugin that provides customizable wide output formats.

It allows you to define column templates (YAML or .tpl) for any Kubernetes resource
type, persist them on disk, and reuse them across sessions. Templates support JSONPath
expressions, Go text/template functions, and Helm-style helpers.

Use 'init' to auto-generate templates for all resources in a cluster, then 'get' to
display resources using those templates.`,
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			return c.Help()
		},
	}

	cmd.PersistentFlags().String("template-path", "/tmp/cwide", "Root directory for column template files")
	cmd.PersistentFlags().String("kubeconfig", "", "Path to a kubeconfig file. If unset, the KUBECONFIG env var or default path is used")

	cmd.AddCommand(initialization.NewCmdInit())
	cmd.AddCommand(get.NewCmdGet(streams))
	cmd.AddCommand(template.NewCmdTemplate())
	cmd.AddCommand(config.NewCmdConfig())
	cmd.AddCommand(marketplace.NewCmdMarketplace())
	cmd.AddCommand(configmapCmd.NewCmdConfigMap())
	cmd.AddCommand(tree.NewCmdTree(streams))
	cmd.AddCommand(list.NewCmdList(streams))
	cmd.AddCommand(alias.NewCmdAlias())

	return cmd
}
