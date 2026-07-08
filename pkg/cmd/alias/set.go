package alias

import (
	"fmt"
	"strings"

	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/utils/ptr"
)

func NewCmdAliasSet() *cobra.Command {
	var context string

	cmd := &cobra.Command{
		Use:   "set ALIAS RESOURCE",
		Short: "Create or update a resource type alias",
		Long: `Set a custom alias for a Kubernetes resource type.

The RESOURCE argument may be a single resource type or a comma-separated list
(an "alias group") — e.g. "pod,service,configmap". Comma-separated targets pass
through to the resource builder unchanged, so 'kubectl cwide get <alias>' lists
all of them at once.

The alias is checked for conflicts against:
  - Existing aliases in the config
  - Built-in Kubernetes resource short names (via Discovery API)

A warning is printed if the alias conflicts with an existing name, but the
alias is still saved.`,
		Example: `  # Single-resource alias
  kubectl cwide alias set pd pods

  # Alias group: 'core' lists pods, services, and configmaps together
  kubectl cwide alias set core pod,service,configmap

  # Long name → short alias
  kubectl cwide alias set vw validatingwebhookconfigurations`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := strings.ToLower(args[0])
			resource := strings.ToLower(args[1])

			config, err := utils.LoadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config (run 'init' first): %w", err)
			}

			// Check for duplicate against existing aliases
			if existing, ok := config.Aliases[alias]; ok {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: alias %q already exists (points to %q), will be overwritten\n", alias, existing)
			}

			// Check for duplicate against built-in k8s short names
			if conflicts := checkK8sShortNameConflicts(cmd, context, alias); len(conflicts) > 0 {
				for _, c := range conflicts {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %q conflicts with built-in short name for %q (%s)\n", alias, c.resource, c.apiVersion)
				}
			}

			// Check for duplicate against existing alias targets pointing to different resources
			for existingAlias, existingResource := range config.Aliases {
				if existingAlias != alias && existingResource == resource {
					fmt.Fprintf(cmd.ErrOrStderr(), "Note: %q is already aliased as %q\n", resource, existingAlias)
				}
			}

			if config.Aliases == nil {
				config.Aliases = make(map[string]string)
			}
			config.Aliases[alias] = resource

			if err := utils.SaveConfig(config); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Alias set: %s → %s\n", alias, resource)
			return nil
		},
	}

	cmd.Flags().StringVar(&context, "context", "", "The name of the kubeconfig context to use")

	return cmd
}

type shortNameConflict struct {
	resource   string
	apiVersion string
}

func checkK8sShortNameConflicts(cmd *cobra.Command, context, alias string) []shortNameConflict {
	kubeConfigFlags := genericclioptions.NewConfigFlags(true).
		WithDeprecatedPasswordFlag().
		WithDiscoveryBurst(300).
		WithDiscoveryQPS(50.0)

	if v := cmd.Flag("kubeconfig"); v != nil && v.Changed {
		kubeConfigFlags.KubeConfig = ptr.To(v.Value.String())
	}
	if context != "" {
		kubeConfigFlags.Context = &context
	}

	matchVersionFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	factory := cmdutil.NewFactory(matchVersionFlags)

	discoveryClient, err := factory.ToDiscoveryClient()
	if err != nil {
		return nil
	}

	_, resourceLists, err := discoveryClient.ServerGroupsAndResources()
	if err != nil && !discovery.IsGroupDiscoveryFailedError(err) {
		return nil
	}

	var conflicts []shortNameConflict
	for _, resourceList := range resourceLists {
		for _, resource := range resourceList.APIResources {
			if strings.Contains(resource.Name, "/") {
				continue
			}
			for _, shortName := range resource.ShortNames {
				if strings.EqualFold(shortName, alias) {
					conflicts = append(conflicts, shortNameConflict{
						resource:   resource.Name,
						apiVersion: resourceList.GroupVersion,
					})
				}
			}
			// Also check if alias matches a resource name itself
			if strings.EqualFold(resource.Name, alias) {
				conflicts = append(conflicts, shortNameConflict{
					resource:   resource.Name,
					apiVersion: resourceList.GroupVersion,
				})
			}
		}
	}

	return conflicts
}
