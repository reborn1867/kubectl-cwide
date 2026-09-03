package models

// YAMLTemplate represents a YAML-based template file for custom column output.
type YAMLTemplate struct {
	// Metadata carries optional provenance info: where the template came
	// from and what version. All fields are optional; a template with no
	// metadata block continues to load and behave as it always has.
	// Populated automatically by 'marketplace install' and 'configmap sync';
	// users can also hand-edit it. Consumed by 'template list' (VERSION
	// column) and by 'template diff' (comparing on-disk to marketplace).
	Metadata *TemplateMetadata `yaml:"metadata,omitempty"`

	Columns []YAMLColumn      `yaml:"columns"`
	Helpers string            `yaml:"helpers,omitempty"`
	Funcs   map[string]string `yaml:"funcs,omitempty"`
}

// TemplateMetadata records where a template came from and when it landed.
//
// Any field may be empty. Interpretation notes:
//
//   Source        — free-form origin tag: "marketplace", "configmap",
//                   "init", "user". Empty means unknown (usually
//                   hand-authored before this field existed).
//   SourceRepo    — for marketplace-installed templates, the GitHub
//                   "owner/name" the file came from (e.g.
//                   "reborn1867/kubectl-cwide-templates").
//   SourceRef     — the git ref that was fetched: tag, branch, or SHA.
//   Version       — a human-friendly version string. When SourceRef is
//                   a tag, this is typically the same string; when
//                   SourceRef is a SHA, Version can carry a semver from
//                   an adjacent manifest.
//   InstalledAt   — RFC3339 timestamp. Purely informational; not used
//                   for staleness comparisons yet.
type TemplateMetadata struct {
	Source      string `yaml:"source,omitempty"`
	SourceRepo  string `yaml:"sourceRepo,omitempty"`
	SourceRef   string `yaml:"sourceRef,omitempty"`
	Version     string `yaml:"version,omitempty"`
	InstalledAt string `yaml:"installedAt,omitempty"`
}

// YAMLColumn defines a single column in a YAML template.
// Either FieldSpec (JSONPath) or Template (Go template) should be set, not both.
type YAMLColumn struct {
	Header    string `yaml:"header"`
	FieldSpec string `yaml:"fieldSpec,omitempty"`
	Template  string `yaml:"template,omitempty"`
}
