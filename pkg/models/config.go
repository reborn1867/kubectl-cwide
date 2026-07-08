package models

import (
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// Config is the struct for the config.yaml file
type Config struct {
	TemplatePath    string            `json:"templatePath" yaml:"templatePath"`
	TemplateSources []string          `json:"templateSources,omitempty" yaml:"templateSources,omitempty"`
	Aliases         map[string]string `json:"aliases,omitempty" yaml:"aliases,omitempty"`

	// DefaultTemplateContext overrides the "default" template name per
	// kubeconfig context (e.g. {"prod": "compact", "dev": "verbose"}).
	DefaultTemplateContext map[string]string `json:"defaultTemplateContext,omitempty" yaml:"defaultTemplateContext,omitempty"`
	// DefaultTemplateNamespace overrides per namespace. Namespace overrides
	// context if both match.
	DefaultTemplateNamespace map[string]string `json:"defaultTemplateNamespace,omitempty" yaml:"defaultTemplateNamespace,omitempty"`
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
