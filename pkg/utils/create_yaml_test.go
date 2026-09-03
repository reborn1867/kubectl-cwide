package utils

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCreateOrFormatYAMLFile_WritesWhenAbsent verifies a fresh directory gets
// the generated content written as-is.
func TestCreateOrFormatYAMLFile_WritesWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pod--v1", "default.yaml")
	content := []byte("columns:\n  - header: NAME\n    fieldSpec: .metadata.name\n")

	if err := CreateOrFormatYAMLFile(path, content); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file to be written: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("content mismatch:\n got: %q\nwant: %q", got, content)
	}
}

// TestCreateOrFormatYAMLFile_PreservesExistingYAML verifies that re-running
// init over an existing default.yaml does not clobber user edits.
func TestCreateOrFormatYAMLFile_PreservesExistingYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "pod--v1", "default.yaml")
	userEdited := []byte("columns:\n  - header: CUSTOM\n    fieldSpec: .spec.custom\n")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, userEdited, 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// init would try to write generated content here — it must be a no-op.
	if err := CreateOrFormatYAMLFile(path, []byte("columns: []\n")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != string(userEdited) {
		t.Fatalf("existing YAML was overwritten:\n got: %q\nwant: %q", got, userEdited)
	}
}

// TestCreateOrFormatYAMLFile_DoesNotShadowExistingTpl is the regression test for
// the reported bug: init must not drop a default.yaml next to a user's
// hand-edited default.tpl, because `get` resolves .yaml before .tpl and would
// silently shadow (effectively erase) the .tpl template.
func TestCreateOrFormatYAMLFile_DoesNotShadowExistingTpl(t *testing.T) {
	dir := t.TempDir()
	resourceDir := filepath.Join(dir, "pod--v1")
	if err := os.MkdirAll(resourceDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	tplPath := filepath.Join(resourceDir, "default.tpl")
	tplContent := []byte("NAME\n.metadata.name\n")
	if err := os.WriteFile(tplPath, tplContent, 0644); err != nil {
		t.Fatalf("seed tpl: %v", err)
	}

	yamlPath := filepath.Join(resourceDir, "default.yaml")
	if err := CreateOrFormatYAMLFile(yamlPath, []byte("columns: []\n")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The .yaml must NOT have been created — otherwise it shadows the .tpl.
	if _, err := os.Stat(yamlPath); !os.IsNotExist(err) {
		t.Fatalf("default.yaml was created and now shadows the user's default.tpl (err=%v)", err)
	}
	// The user's .tpl must be untouched.
	got, err := os.ReadFile(tplPath)
	if err != nil {
		t.Fatalf("read tpl: %v", err)
	}
	if string(got) != string(tplContent) {
		t.Fatalf("default.tpl was modified:\n got: %q\nwant: %q", got, tplContent)
	}
}
