package template

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestInstalledTemplateNames verifies enumeration used by the "template not
// found" error: basenames of .yaml/.yml/.tpl, deduped (.yaml and .tpl sharing a
// basename collapse), sorted; directories and other files ignored.
func TestInstalledTemplateNames(t *testing.T) {
	dir := t.TempDir()
	// default exists as both .yaml and .tpl → one entry.
	mustWrite(t, filepath.Join(dir, "default.yaml"), "columns: []\n")
	mustWrite(t, filepath.Join(dir, "default.tpl"), "NAME\n.metadata.name\n")
	mustWrite(t, filepath.Join(dir, "debug.yaml"), "columns: []\n")
	mustWrite(t, filepath.Join(dir, "wide.yml"), "columns: []\n")
	mustWrite(t, filepath.Join(dir, "notes.txt"), "ignore me\n")
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got := installedTemplateNames(dir)
	want := []string{"debug", "default", "wide"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("installedTemplateNames = %v, want %v", got, want)
	}
}

// TestInstalledTemplateNames_EmptyOrMissing returns nil for an empty or
// unreadable directory (so the caller can switch to the "nothing installed"
// message).
func TestInstalledTemplateNames_EmptyOrMissing(t *testing.T) {
	empty := t.TempDir()
	if got := installedTemplateNames(empty); len(got) != 0 {
		t.Errorf("empty dir should yield no names, got %v", got)
	}
	if got := installedTemplateNames(filepath.Join(empty, "does-not-exist")); got != nil {
		t.Errorf("missing dir should yield nil, got %v", got)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
