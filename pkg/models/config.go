package models

import (
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// Config is the struct for the config.yaml file
type Config struct {
	TemplatePath    string            `json:"templatePath" yaml:"templatePath"`
	TemplateSources []string          `json:"templateSources,omitempty" yaml:"templateSources,omitempty"`
	Aliases         map[string]string `json:"aliases,omitempty" yaml:"aliases,omitempty"`
}

type CRDProperty struct {
	Names v1.CustomResourceDefinitionNames `json:"names"`
}
