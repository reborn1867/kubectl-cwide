package get

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListInstalledTemplates_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	if got := listInstalledTemplates(dir); len(got) != 0 {
		t.Fatalf("empty dir should yield no templates; got %v", got)
	}
}

func TestListInstalledTemplates_MissingDir(t *testing.T) {
	if got := listInstalledTemplates(filepath.Join(t.TempDir(), "does-not-exist")); got != nil {
		t.Fatalf("missing dir should return nil; got %v", got)
	}
}

func TestListInstalledTemplates_DedupsYAMLAndTPL(t *testing.T) {
	dir := t.TempDir()
	// Same basename, both extensions — resolver prefers .yaml, so one entry.
	if err := os.WriteFile(filepath.Join(dir, "default.yaml"), []byte("columns: []"), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "default.tpl"), []byte("H\n{.n}"), 0o644); err != nil {
		t.Fatalf("write tpl: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "wide.yaml"), []byte("columns: []"), 0o644); err != nil {
		t.Fatalf("write wide: %v", err)
	}
	got := listInstalledTemplates(dir)
	if len(got) != 2 {
		t.Fatalf("expected 2 unique templates; got %v", got)
	}
	if got[0] != "default" || got[1] != "wide" {
		t.Fatalf("expected sorted [default wide]; got %v", got)
	}
}

func TestListInstalledTemplates_IgnoresUnrelatedFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "only.yaml"), []byte("columns: []"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := listInstalledTemplates(dir)
	if len(got) != 1 || got[0] != "only" {
		t.Fatalf("expected [only]; got %v", got)
	}
}

func TestTemplateNotFoundError_ListsAvailable(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"default.yaml", "wide.yaml", "debug.tpl"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	err := templateNotFoundError("abc", dir)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{`"abc"`, "not found", "available:", "default", "wide", "debug"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q; got: %s", want, msg)
		}
	}
}

func TestTemplateNotFoundError_NoTemplatesInstalled(t *testing.T) {
	dir := t.TempDir()
	err := templateNotFoundError("abc", dir)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	// Empty dir → guide the user forward, don't say "available: <empty>".
	if strings.Contains(msg, "available:") {
		t.Errorf("empty-dir error shouldn't say 'available:'; got: %s", msg)
	}
	for _, want := range []string{"no templates installed", "init", "scaffold"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q; got: %s", want, msg)
		}
	}
}

func TestSortStrings_Sorts(t *testing.T) {
	got := []string{"c", "a", "b"}
	sortStrings(got)
	want := []string{"a", "b", "c"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sortStrings: got %v; want %v", got, want)
		}
	}
}
