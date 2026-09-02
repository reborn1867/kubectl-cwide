package template

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadTemplateMetadata_Present(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "default.yaml")
	body := `metadata:
  source: marketplace
  sourceRepo: acme/templates
  sourceRef: v1.2.0
  version: v1.2.0
  installedAt: "2026-01-01T00:00:00Z"
columns:
  - header: NAME
    fieldSpec: .metadata.name
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	m := readTemplateMetadata(path)
	if m == nil {
		t.Fatal("metadata not parsed")
	}
	if m.Source != "marketplace" || m.SourceRepo != "acme/templates" || m.Version != "v1.2.0" {
		t.Errorf("fields wrong: %+v", m)
	}
}

func TestReadTemplateMetadata_Absent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-meta.yaml")
	body := `columns:
  - header: NAME
    fieldSpec: .metadata.name
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	if m := readTemplateMetadata(path); m != nil {
		t.Errorf("expected nil for template without metadata; got %+v", m)
	}
}

func TestReadTemplateMetadata_TplExtensionSkipped(t *testing.T) {
	// .tpl files carry no metadata block by design; the reader
	// short-circuits without even opening the file. This is the contract
	// that keeps 'template list' fast on large template roots.
	dir := t.TempDir()
	path := filepath.Join(dir, "default.tpl")
	if err := os.WriteFile(path, []byte("NAME\n.metadata.name\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if m := readTemplateMetadata(path); m != nil {
		t.Errorf(".tpl files should return nil; got %+v", m)
	}
}

func TestReadTemplateMetadata_UnreadableFileReturnsNil(t *testing.T) {
	// Missing file → nil, not a crash. Keeps 'template list' robust when
	// races or filesystem hiccups intervene.
	if m := readTemplateMetadata("/no/such/path.yaml"); m != nil {
		t.Errorf("missing file: expected nil; got %+v", m)
	}
}
