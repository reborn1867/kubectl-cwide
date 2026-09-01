package get

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListAvailableTemplates_MergesYAMLAndTpl(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"default.yaml", "debug.tpl", "wide.yaml", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Same-basename dedup: shouldn't produce "default" twice when both
	// .yaml and .tpl exist.
	if err := os.WriteFile(filepath.Join(dir, "default.tpl"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	got := listAvailableTemplates(dir)
	want := []string{"debug", "default", "wide"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestListAvailableTemplates_MissingDir(t *testing.T) {
	if got := listAvailableTemplates("/nonexistent/path/that/should/not/exist"); got != nil {
		t.Errorf("expected nil for missing dir, got %v", got)
	}
}

func TestTemplateNotFoundError_ListsAvailable(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"default.yaml", "debug.yaml"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	err := templateNotFoundError(dir, "bogus")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"bogus", "available templates", "debug", "default"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error missing %q; got:\n%s", want, msg)
		}
	}
}

func TestTemplateNotFoundError_EmptyDirGuidesInit(t *testing.T) {
	dir := t.TempDir()
	err := templateNotFoundError(dir, "anything")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"no templates exist", "kubectl cwide init", "template create"} {
		if !strings.Contains(msg, want) {
			t.Errorf("empty-dir error missing %q; got:\n%s", want, msg)
		}
	}
}
