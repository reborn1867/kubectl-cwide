package get

import (
	"strings"
	"testing"
)

func obj() map[string]interface{} {
	return map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "web-1",
		},
		"status": map[string]interface{}{
			"readyReplicas":     int64(3),
			"replicas":          int64(3),
			"availableReplicas": int64(3),
			"phase":             "",           // present but empty
			"conditions":        []interface{}{}, // present but empty slice
		},
		"spec": map[string]interface{}{
			"nodeName": "node-a",
		},
	}
}

func TestDiagnoseEmptyPath_TypoMidPath(t *testing.T) {
	d := diagnoseEmptyPath(".status.readReplicas", obj())
	if d.Untraceable || d.FullyResolved {
		t.Fatalf("expected a traceable missing-key result, got %+v", d)
	}
	if d.ResolvedPrefix != ".status" {
		t.Errorf("resolvedPrefix = %q, want .status", d.ResolvedPrefix)
	}
	if d.MissingSegment != "readReplicas" {
		t.Errorf("missingSegment = %q, want readReplicas", d.MissingSegment)
	}
	if d.Suggestion != "readyReplicas" {
		t.Errorf("suggestion = %q, want readyReplicas", d.Suggestion)
	}
	// available keys should be sorted and include the real fields
	joined := strings.Join(d.AvailableKeys, ",")
	if !strings.Contains(joined, "readyReplicas") || !strings.Contains(joined, "replicas") {
		t.Errorf("availableKeys missing expected entries: %v", d.AvailableKeys)
	}
}

func TestDiagnoseEmptyPath_MissingTopLevel(t *testing.T) {
	d := diagnoseEmptyPath(".spceee.foo", obj()) // typo'd top segment
	if d.ResolvedPrefix != "" {
		t.Errorf("resolvedPrefix should be root (empty), got %q", d.ResolvedPrefix)
	}
	if d.MissingSegment != "spceee" {
		t.Errorf("missingSegment = %q, want spceee", d.MissingSegment)
	}
	// top-level keys listed
	if len(d.AvailableKeys) == 0 {
		t.Errorf("expected top-level keys, got none")
	}
}

func TestDiagnoseEmptyPath_FullyResolvedButEmpty(t *testing.T) {
	// phase exists but is "" — genuinely empty, not a typo.
	d := diagnoseEmptyPath(".status.phase", obj())
	if !d.FullyResolved {
		t.Fatalf("expected FullyResolved, got %+v", d)
	}
	if d.MissingSegment != "" {
		t.Errorf("no segment should be missing, got %q", d.MissingSegment)
	}
	if d.ResolvedPrefix != ".status.phase" {
		t.Errorf("resolvedPrefix = %q, want .status.phase", d.ResolvedPrefix)
	}
}

func TestDiagnoseEmptyPath_FilterUntraceable(t *testing.T) {
	d := diagnoseEmptyPath(`.status.conditions[?(@.type=="Ready")].status`, obj())
	if !d.Untraceable {
		t.Fatalf("filter path should be untraceable, got %+v", d)
	}
	if d.Reason == "" {
		t.Errorf("untraceable result should carry a reason")
	}
}

func TestDiagnoseEmptyPath_DescendIntoScalar(t *testing.T) {
	// spec.nodeName is a string; descending further is impossible.
	d := diagnoseEmptyPath(".spec.nodeName.foo", obj())
	if d.Untraceable || d.FullyResolved {
		t.Fatalf("expected a not-an-object result, got %+v", d)
	}
	if d.ResolvedPrefix != ".spec.nodeName" || d.MissingSegment != "foo" {
		t.Errorf("unexpected resolve point: %+v", d)
	}
	if d.Reason == "" {
		t.Errorf("expected a 'not an object' note")
	}
}

func TestNormalizeDotPath(t *testing.T) {
	cases := map[string]string{
		"{.a.b}": ".a.b",
		"{a.b}":  ".a.b",
		".a.b":   ".a.b",
		"a.b":    ".a.b",
	}
	for in, want := range cases {
		if got := normalizeDotPath(in); got != want {
			t.Errorf("normalizeDotPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"readReplicas", "readyReplicas", 1},
		{"kitten", "sitting", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
	}
	for _, c := range cases {
		if got := levenshtein(c.a, c.b); got != c.want {
			t.Errorf("levenshtein(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestNearestKey_RespectsThreshold(t *testing.T) {
	keys := []string{"readyReplicas", "replicas", "availableReplicas"}
	if got := nearestKey("readReplicas", keys); got != "readyReplicas" {
		t.Errorf("nearestKey = %q, want readyReplicas", got)
	}
	// far-off target → no suggestion
	if got := nearestKey("completelyDifferent", keys); got != "" {
		t.Errorf("nearestKey should be empty for distant target, got %q", got)
	}
}

func TestFormatEmptyDiagnosis_IncludesHint(t *testing.T) {
	d := diagnoseEmptyPath(".status.readReplicas", obj())
	out := FormatEmptyDiagnosis("READY", "deployment/web", d)
	for _, want := range []string{"READY empty for deployment/web", ".status.readReplicas", "missing:  readReplicas", "readyReplicas"} {
		if !strings.Contains(out, want) {
			t.Errorf("formatted output missing %q:\n%s", want, out)
		}
	}
}
