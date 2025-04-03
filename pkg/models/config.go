package models

import (
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// Config is the struct for the config.yaml file
type Config struct {
	TemplatePath string `json:"templatePath"`
}

type CRDProperty struct {
	Names v1.CustomResourceDefinitionNames `json:"names"`
}
