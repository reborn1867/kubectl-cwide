package models

import (
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// Config is the struct for the config.yaml file
type Config struct {
	TemplatePath    string            `json:"templatePath" yaml:"templatePath"`
	TemplateSources []string          `json:"templateSources,omitempty" yaml:"templateSources,omitempty"`
	Aliases         map[string]string `json:"aliases,omitempty" yaml:"aliases,omitempty"`

	// AliasEntries is the richer alias format that allows binding a template
	// (or per-resource templates for alias groups) to an alias. When an alias
	// name is present in both Aliases and AliasEntries, AliasEntries wins.
	// Existing v0.8.x configs only use Aliases; new writes go here.
	AliasEntries map[string]AliasEntry `json:"aliasEntries,omitempty" yaml:"aliasEntries,omitempty"`

	// DefaultTemplateContext overrides the "default" template name per
	// kubeconfig context (e.g. {"prod": "compact", "dev": "verbose"}).
	DefaultTemplateContext map[string]string `json:"defaultTemplateContext,omitempty" yaml:"defaultTemplateContext,omitempty"`
	// DefaultTemplateNamespace overrides per namespace. Namespace overrides
	// context if both match.
	DefaultTemplateNamespace map[string]string `json:"defaultTemplateNamespace,omitempty" yaml:"defaultTemplateNamespace,omitempty"`
}

// AliasEntry describes one alias's target resource(s) plus optional template
// bindings. Resource is a single kind or comma-separated list matching what
// `Aliases[name]` holds. Template applies to every kind in Resource when the
// alias resolves; Templates (per-kind) overrides Template for specific kinds
// in an alias group.
type AliasEntry struct {
	Resource  string            `json:"resource" yaml:"resource"`
	Template  string            `json:"template,omitempty" yaml:"template,omitempty"`
	Templates map[string]string `json:"templates,omitempty" yaml:"templates,omitempty"`
}

// ResolveAliasTarget returns the resource string for an alias. Checks the
// rich AliasEntries first, then falls back to the legacy Aliases map.
// Returns "" if the alias isn't found.
func (c *Config) ResolveAliasTarget(name string) string {
	if e, ok := c.AliasEntries[name]; ok {
		return e.Resource
	}
	return c.Aliases[name]
}

// ResolveAliasTemplate returns the template name to use for a given
// (alias, resource-kind) pair. Precedence: AliasEntries[name].Templates[kind],
// AliasEntries[name].Template, then "" meaning "use the standard default".
// The kind argument matches the singular/plural token from the alias target
// after group-splitting (e.g. "pod", "svc", "cm"). Callers may pass "" for
// single-kind aliases where the kind is redundant.
func (c *Config) ResolveAliasTemplate(name, kind string) string {
	e, ok := c.AliasEntries[name]
	if !ok {
		return ""
	}
	if kind != "" {
		if t, ok := e.Templates[kind]; ok && t != "" {
			return t
		}
	}
	return e.Template
}

// ResolveDefaultTemplate picks the effective default template name for the
// given (context, namespace) pair. Precedence: namespace > context > "default".
func (c *Config) ResolveDefaultTemplate(kubeCtx, namespace string) string {
	if namespace != "" {
		if t, ok := c.DefaultTemplateNamespace[namespace]; ok && t != "" {
			return t
		}
	}
	if kubeCtx != "" {
		if t, ok := c.DefaultTemplateContext[kubeCtx]; ok && t != "" {
			return t
		}
	}
	return "default"
}

type CRDProperty struct {
	Names v1.CustomResourceDefinitionNames `json:"names"`
}
