package marketplace

import (
	"strings"
	"testing"

	"github.com/kubectl-cwide/pkg/models"
	"gopkg.in/yaml.v3"
)

func TestStampMarketplaceMetadata_AddsBlockWhenAbsent(t *testing.T) {
	in := []byte(`columns:
  - header: NAME
    fieldSpec: .metadata.name
`)
	out := stampMarketplaceMetadata(in, "acme/templates", "v1.2.0")
	var tmpl models.YAMLTemplate
	if err := yaml.Unmarshal(out, &tmpl); err != nil {
		t.Fatalf("output not parseable: %v", err)
	}
	if tmpl.Metadata == nil {
		t.Fatal("metadata block not written")
	}
	if tmpl.Metadata.Source != "marketplace" {
		t.Errorf("Source = %q", tmpl.Metadata.Source)
	}
	if tmpl.Metadata.SourceRepo != "acme/templates" {
		t.Errorf("SourceRepo = %q", tmpl.Metadata.SourceRepo)
	}
	if tmpl.Metadata.SourceRef != "v1.2.0" || tmpl.Metadata.Version != "v1.2.0" {
		t.Errorf("Ref/Version = %q/%q", tmpl.Metadata.SourceRef, tmpl.Metadata.Version)
	}
	if tmpl.Metadata.InstalledAt == "" {
		t.Errorf("InstalledAt should be stamped")
	}
	// Columns must be preserved verbatim.
	if len(tmpl.Columns) != 1 || tmpl.Columns[0].Header != "NAME" {
		t.Errorf("columns lost: %+v", tmpl.Columns)
	}
}

func TestStampMarketplaceMetadata_PreservesUpstreamVersion(t *testing.T) {
	// When the upstream author declared their own Version, ref shouldn't
	// clobber it — a mutable branch name like 'main' shouldn't replace a
	// semver the author baked in.
	in := []byte(`metadata:
  version: 3.1.4
columns:
  - header: NAME
    fieldSpec: .metadata.name
`)
	out := stampMarketplaceMetadata(in, "acme/templates", "main")
	var tmpl models.YAMLTemplate
	if err := yaml.Unmarshal(out, &tmpl); err != nil {
		t.Fatalf("output not parseable: %v", err)
	}
	if tmpl.Metadata.Version != "3.1.4" {
		t.Errorf("upstream version overwritten: got %q, want 3.1.4", tmpl.Metadata.Version)
	}
	if tmpl.Metadata.SourceRef != "main" {
		t.Errorf("SourceRef should still record the ref we fetched: %q", tmpl.Metadata.SourceRef)
	}
}

func TestStampMarketplaceMetadata_LeavesNonTemplateAlone(t *testing.T) {
	// Random non-template file — no columns block — should pass through
	// unchanged so we never corrupt something we can't understand.
	in := []byte("not a template at all\n")
	out := stampMarketplaceMetadata(in, "acme/templates", "v1")
	if string(out) != string(in) {
		t.Errorf("non-template file was mutated:\nin:  %q\nout: %q", in, out)
	}
}

func TestStampMarketplaceMetadata_HandlesEmptyRef(t *testing.T) {
	// Install without --ref: SourceRef and Version should both stay empty,
	// not accidentally get "".
	in := []byte("columns:\n  - header: NAME\n    fieldSpec: .metadata.name\n")
	out := stampMarketplaceMetadata(in, "acme/templates", "")
	var tmpl models.YAMLTemplate
	if err := yaml.Unmarshal(out, &tmpl); err != nil {
		t.Fatal(err)
	}
	if tmpl.Metadata.SourceRef != "" || tmpl.Metadata.Version != "" {
		t.Errorf("empty ref should not set Version; got ref=%q ver=%q",
			tmpl.Metadata.SourceRef, tmpl.Metadata.Version)
	}
	if !strings.Contains(string(out), "source: marketplace") {
		t.Errorf("expected 'source: marketplace' in output; got:\n%s", out)
	}
}
