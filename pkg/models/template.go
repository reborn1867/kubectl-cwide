package models

// YAMLTemplate represents a YAML-based template file for custom column output.
type YAMLTemplate struct {
	Columns []YAMLColumn `yaml:"columns"`
	Helpers string       `yaml:"helpers,omitempty"`
}

// YAMLColumn defines a single column in a YAML template.
// Either FieldSpec (JSONPath) or Template (Go template) should be set, not both.
type YAMLColumn struct {
	Header    string `yaml:"header"`
	FieldSpec string `yaml:"fieldSpec,omitempty"`
	Template  string `yaml:"template,omitempty"`
}
