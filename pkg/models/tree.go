package models

// TreeRuleFile represents a YAML rule file for the tree command.
type TreeRuleFile struct {
	Relations []TreeRelation `yaml:"relations"`
}

// TreeRelation defines a relationship between a parent and child resource type.
type TreeRelation struct {
	Resource string       `yaml:"resource"` // child resource type, e.g. "replicasets"
	Bind     TreeBindSpec `yaml:"bind"`
}

// TreeBindSpec describes how a child resource is bound to its parent.
type TreeBindSpec struct {
	Type   string `yaml:"type"`             // "ownerRef", "labelSelector", or "fieldRef"
	Parent string `yaml:"parent,omitempty"` // parent resource type; empty means root
	Path   string `yaml:"path,omitempty"`   // JSONPath for fieldRef binding
}
