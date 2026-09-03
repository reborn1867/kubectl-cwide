package alias

import (
	"fmt"
	"strings"

	"github.com/kubectl-cwide/pkg/clients"
	"github.com/kubectl-cwide/pkg/models"
	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
	"k8s.io/client-go/discovery"
)

func NewCmdAliasSet() *cobra.Command {
	var context string
	var template string
	var perKindTemplates []string
	var force bool

	cmd := &cobra.Command{
		Use:   "set ALIAS RESOURCE",
		Short: "Create or update a resource type alias",
		Long: `Set a custom alias for a Kubernetes resource type.

The RESOURCE argument may be a single resource type or a comma-separated list
(an "alias group") — e.g. "pod,service,configmap". Comma-separated targets pass
through to the resource builder unchanged, so 'kubectl cwide get <alias>' lists
all of them at once.

Bind a template with --template=NAME so 'cwide get <alias>' uses that template
instead of the standard default. For alias groups, use --resource-template
repeatedly to bind different templates per kind: --resource-template pod=debug
--resource-template svc=wide.

The alias is checked for conflicts against:
  - Existing aliases in the config (existing entry is overwritten with a warning)
  - Kubernetes resource names, plurals, and short names via Discovery — this
    covers built-ins AND any registered CRDs. A collision here is rejected
    with an error because 'cwide get <alias>' would silently shadow the real
    resource. Pass --force to override that check.`,
		Example: `  # Single-resource alias
  kubectl cwide alias set pd pods

  # Alias with a bound template
  kubectl cwide alias set pd pods --template debug

  # Alias group: 'core' lists pods, services, and configmaps together
  kubectl cwide alias set core pod,service,configmap

  # Alias group with per-kind templates
  kubectl cwide alias set core pod,service,cm --resource-template pod=debug --resource-template svc=wide

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

			// Check for duplicate against existing aliases (rich or legacy).
			if e, ok := config.AliasEntries[alias]; ok {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: alias %q already exists (points to %q), will be overwritten\n", alias, e.Resource)
			} else if existing, ok := config.Aliases[alias]; ok {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: alias %q already exists (points to %q), will be overwritten\n", alias, existing)
			}

			// Check for duplicate against k8s resource names + short names
			// (walks Discovery, so CRDs are covered along with built-ins).
			// A conflict here would silently shadow the real resource in
			// 'cwide get <name>' — that's a footgun, so we reject the alias.
			// Users who really want to shadow can pass --force.
			if conflicts := checkK8sShortNameConflicts(cmd, context, alias); len(conflicts) > 0 {
				lines := make([]string, 0, len(conflicts))
				for _, c := range conflicts {
					lines = append(lines, fmt.Sprintf("  - %s (%s)", c.resource, c.apiVersion))
				}
				if !force {
					return fmt.Errorf(
						"alias %q collides with an existing cluster resource:\n%s\n"+
							"'cwide get %s' would silently shadow the real resource. "+
							"Pick a different alias name, or pass --force to override.",
						alias, strings.Join(lines, "\n"), alias)
				}
				for _, l := range lines {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: %q conflicts with existing resource:\n%s\n  proceeding because --force was set\n", alias, l)
				}
			}

			// Check for duplicate against existing alias targets pointing to different resources
			for existingAlias, existingResource := range config.Aliases {
				if existingAlias != alias && existingResource == resource {
					fmt.Fprintf(cmd.ErrOrStderr(), "Note: %q is already aliased as %q\n", resource, existingAlias)
				}
			}

			// Parse --resource-template flags into a map.
			perKind := map[string]string{}
			for _, spec := range perKindTemplates {
				k, v, ok := strings.Cut(spec, "=")
				if !ok || k == "" || v == "" {
					return fmt.Errorf("invalid --resource-template %q; expected kind=template", spec)
				}
				perKind[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
			}

			// If any template binding was requested, use the rich AliasEntries
			// path; otherwise write to the legacy Aliases map for backward compat.
			if template != "" || len(perKind) > 0 {
				if config.AliasEntries == nil {
					config.AliasEntries = make(map[string]models.AliasEntry)
				}
				config.AliasEntries[alias] = models.AliasEntry{
					Resource:  resource,
					Template:  template,
					Templates: perKind,
				}
				// Remove legacy entry so lookups stay consistent.
				delete(config.Aliases, alias)
			} else {
				if config.Aliases == nil {
					config.Aliases = make(map[string]string)
				}
				config.Aliases[alias] = resource
				delete(config.AliasEntries, alias)
			}

			if err := utils.SaveConfig(config); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}

			// User-facing confirmation reflects the effective binding.
			msg := fmt.Sprintf("Alias set: %s → %s", alias, resource)
			if template != "" {
				msg += fmt.Sprintf(" (template=%s)", template)
			}
			if len(perKind) > 0 {
				parts := make([]string, 0, len(perKind))
				for k, v := range perKind {
					parts = append(parts, k+"="+v)
				}
				msg += fmt.Sprintf(" (per-kind: %s)", strings.Join(parts, ","))
			}
			fmt.Fprintln(cmd.OutOrStdout(), msg)
			return nil
		},
	}

	cmd.Flags().StringVar(&context, "context", "", "The name of the kubeconfig context to use")
	cmd.Flags().StringVarP(&template, "template", "t", "",
		"Bind a template name to this alias so 'get <alias>' uses it as the default")
	cmd.Flags().StringArrayVar(&perKindTemplates, "resource-template", nil,
		"For alias groups: kind=template to bind different templates per kind (repeatable, e.g. --resource-template pod=debug)")
	cmd.Flags().BoolVar(&force, "force", false,
		"Save the alias even if it collides with an existing cluster resource name or short name (default: reject with an error).")

	return cmd
}

type shortNameConflict struct {
	resource   string
	apiVersion string
}

func checkK8sShortNameConflicts(cmd *cobra.Command, context, alias string) []shortNameConflict {
	factory := clients.FactoryFromCmd(cmd, context)

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
