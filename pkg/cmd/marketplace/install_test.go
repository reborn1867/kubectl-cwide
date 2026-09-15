package marketplace

import (
	"reflect"
	"testing"
)

// TestTemplateNamesFromEntries verifies extraction used by the "template not
// found" error: .yaml/.yml/.tpl basenames, deduped (.yaml + .tpl of the same
// name collapse), sorted; non-template entries ignored.
func TestTemplateNamesFromEntries(t *testing.T) {
	entries := []GitHubEntry{
		{Name: "debug.yaml"},
		{Name: "default.yaml"},
		{Name: "default.tpl"}, // same base as default.yaml → one entry
		{Name: "compact.yml"},
		{Name: "README.md"}, // ignored
		{Name: "manifest.yaml.bak"}, // not a template extension → ignored
	}
	got := templateNamesFromEntries(entries)
	want := []string{"compact", "debug", "default"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("templateNamesFromEntries = %v, want %v", got, want)
	}
}

func TestTemplateNamesFromEntries_Empty(t *testing.T) {
	if got := templateNamesFromEntries(nil); len(got) != 0 {
		t.Errorf("nil entries should yield no names, got %v", got)
	}
	if got := templateNamesFromEntries([]GitHubEntry{{Name: "notes.txt"}}); len(got) != 0 {
		t.Errorf("no template files should yield no names, got %v", got)
	}
}
