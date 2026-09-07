package explain

import (
	"strings"
	"testing"

	"github.com/kubectl-cwide/pkg/models"
)

func TestJSONPathRoot(t *testing.T) {
	cases := map[string]string{
		".status.phase":     "status",
		"{.metadata.name}":  "metadata",
		"metadata.name":     "metadata",
		".spec.containers[0].image": "spec",
		"":                  "",
		".status.conditions[?(@.type==\"Ready\")].status": "status",
	}
	for in, want := range cases {
		if got := jsonPathRoot(in); got != want {
			t.Errorf("jsonPathRoot(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestScanFuncs_FlagsLiveCalls(t *testing.T) {
	body := `{{ probeCheck . "readiness" }} {{ age .metadata.creationTimestamp }}`
	got := scanFuncs(body)
	byName := map[string]funcInfo{}
	for _, f := range got {
		byName[f.Name] = f
	}
	if _, ok := byName["age"]; !ok {
		t.Errorf("expected age detected; got %v", got)
	}
	pc, ok := byName["probeCheck"]
	if !ok {
		t.Fatalf("expected probeCheck detected; got %v", got)
	}
	if pc.LiveCall == "" {
		t.Errorf("probeCheck should be flagged as a live call")
	}
	if byName["age"].LiveCall != "" {
		t.Errorf("age must not be flagged as a live call")
	}
}

func TestExplainTemplate_YAMLWithMetadataAndColumns(t *testing.T) {
	data := []byte(`
metadata:
  source: marketplace
  version: 1.2.0
  sourceRepo: reborn1867/kubectl-cwide-templates
columns:
  - header: NAME
    fieldSpec: .metadata.name
  - header: STATUS
    fieldSpec: .status.phase
  - header: AGE
    fieldSpec: $_defaultPrinterField
  - header: READY
    template: '{{ probeCheck . "readiness" }}'
`)
	res, err := explainTemplate("pod--v1/debug.yaml", data)
	if err != nil {
		t.Fatalf("explainTemplate: %v", err)
	}
	if res.Kind != "template" || res.Format != "yaml" {
		t.Fatalf("kind/format wrong: %+v", res)
	}
	if res.Provenance == nil || res.Provenance.Source != "marketplace" || res.Provenance.Version != "1.2.0" {
		t.Fatalf("provenance not parsed: %+v", res.Provenance)
	}
	if len(res.Columns) != 4 {
		t.Fatalf("want 4 columns, got %d", len(res.Columns))
	}
	// column kinds
	kinds := map[string]string{}
	for _, c := range res.Columns {
		kinds[c.Header] = c.Kind
	}
	if kinds["NAME"] != "jsonpath" || kinds["AGE"] != "default-printer" || kinds["READY"] != "template" {
		t.Fatalf("column classification wrong: %v", kinds)
	}
	// fields read from jsonpath columns (not the default-printer or template)
	joined := strings.Join(res.FieldsRead, ",")
	if !strings.Contains(joined, "metadata") || !strings.Contains(joined, "status") {
		t.Errorf("fieldsRead missing roots: %v", res.FieldsRead)
	}
	// has a template column → fields approximate
	if res.FieldsExact {
		t.Errorf("FieldsExact should be false when a template column exists")
	}
	// probeCheck detected + flagged
	var sawProbe bool
	for _, f := range res.Functions {
		if f.Name == "probeCheck" && f.LiveCall != "" {
			sawProbe = true
		}
	}
	if !sawProbe {
		t.Errorf("probeCheck not detected/flagged: %v", res.Functions)
	}
}

func TestExplainTemplate_NoMetadata(t *testing.T) {
	data := []byte("columns:\n  - header: NAME\n    fieldSpec: .metadata.name\n")
	res, err := explainTemplate("pod--v1/default.yaml", data)
	if err != nil {
		t.Fatalf("explainTemplate: %v", err)
	}
	if res.Provenance != nil {
		t.Errorf("expected nil provenance for metadata-less template")
	}
	if !res.FieldsExact {
		t.Errorf("all-jsonpath template should have exact fields")
	}
}

func TestExplainAlias_RichEntryWithBindings(t *testing.T) {
	cfg := &models.Config{
		AliasEntries: map[string]models.AliasEntry{
			"core": {
				Resource:  "pod,service,configmap",
				Template:  "compact",
				Templates: map[string]string{"pod": "debug"},
			},
		},
	}
	res := explainAlias(cfg, "core")
	if res.Kind != "alias" || res.SourceFmt != "aliasEntries" {
		t.Fatalf("wrong kind/source: %+v", res)
	}
	if res.Target != "pod,service,configmap" {
		t.Fatalf("wrong target: %q", res.Target)
	}
	if len(res.Kinds) != 3 {
		t.Fatalf("group kinds not split: %v", res.Kinds)
	}
	if res.Template != "compact" || res.PerKindTmpl["pod"] != "debug" {
		t.Fatalf("template bindings wrong: %+v", res)
	}
	if res.Note == "" {
		t.Errorf("group alias should carry the TYPE/NAME caveat note")
	}
}

func TestExplainAlias_LegacyMap(t *testing.T) {
	cfg := &models.Config{Aliases: map[string]string{"pd": "pods"}}
	res := explainAlias(cfg, "pd")
	if res.SourceFmt != "aliases (legacy)" || res.Target != "pods" {
		t.Fatalf("legacy alias wrong: %+v", res)
	}
	if len(res.Kinds) != 0 || res.Note != "" {
		t.Errorf("single-kind alias should have no group note: %+v", res)
	}
}

func TestIsAlias(t *testing.T) {
	cfg := &models.Config{
		Aliases:      map[string]string{"pd": "pods"},
		AliasEntries: map[string]models.AliasEntry{"core": {Resource: "pod,svc"}},
	}
	if !isAlias(cfg, "pd") || !isAlias(cfg, "core") {
		t.Errorf("configured aliases should be recognized")
	}
	if isAlias(cfg, "nope") {
		t.Errorf("unknown name should not be an alias")
	}
	if isAlias(nil, "pd") {
		t.Errorf("nil config should not report aliases")
	}
}
