package completions

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/kubectl-cwide/pkg/common"
	"github.com/kubectl-cwide/pkg/models"
)

// seedConfig points os.UserHomeDir at a temp dir holding a config.yaml with the
// given aliases (legacy) and alias entries (rich). Overrides both HOME and
// USERPROFILE so LoadConfig resolves the same temp path on Unix and Windows.
func seedConfig(t *testing.T, aliases map[string]string, entries map[string]models.AliasEntry) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	cfg := models.Config{TemplatePath: filepath.Join(dir, "tpl"), Aliases: aliases, AliasEntries: entries}
	raw, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	path := filepath.Join(dir, common.ConfigPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

// TestAliasNames_UnionOfLegacyAndRich is the regression test for the bug where
// AliasNames only surfaced legacy Aliases and missed rich AliasEntries.
func TestAliasNames_UnionOfLegacyAndRich(t *testing.T) {
	seedConfig(t,
		map[string]string{"pd": "pods"},
		map[string]models.AliasEntry{"core": {Resource: "pod,svc"}},
	)
	got, _ := AliasNames(nil, nil, "")
	set := map[string]bool{}
	for _, n := range got {
		set[n] = true
	}
	if !set["pd"] {
		t.Errorf("legacy alias 'pd' missing from completion: %v", got)
	}
	if !set["core"] {
		t.Errorf("rich alias 'core' missing from completion (the bug): %v", got)
	}
}

// TestAliasNames_DedupesOverlap ensures a name present in both maps appears once.
func TestAliasNames_DedupesOverlap(t *testing.T) {
	seedConfig(t,
		map[string]string{"x": "pods"},
		map[string]models.AliasEntry{"x": {Resource: "pod,svc"}},
	)
	got, _ := AliasNames(nil, nil, "")
	count := 0
	for _, n := range got {
		if n == "x" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("alias 'x' should appear once, appeared %d times: %v", count, got)
	}
}

func TestFilterPrefix(t *testing.T) {
	all := []string{"pod", "pods", "podsecuritypolicy", "service"}
	cases := []struct {
		prefix string
		want   []string
	}{
		{"", all},
		{"pod", []string{"pod", "pods", "podsecuritypolicy"}},
		{"pods", []string{"pods", "podsecuritypolicy"}},
		{"svc", []string{}},
	}
	for _, tc := range cases {
		got := filterPrefix(all, tc.prefix)
		if len(got) == 0 && len(tc.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("filterPrefix(%q) = %v; want %v", tc.prefix, got, tc.want)
		}
	}
}

func TestResourceMatchesDir(t *testing.T) {
	cases := []struct {
		dir      string
		resource string
		want     bool
	}{
		// singular Kind, plural resource, singular resource all match the pod dir
		{"pod--v1", "pod", true},
		{"pod--v1", "pods", true},
		// -es plural
		{"ingress-networking.k8s.io-v1", "ingress", true},
		{"ingress-networking.k8s.io-v1", "ingresses", true},
		// -ies plural
		{"networkpolicy-networking.k8s.io-v1", "networkpolicies", true},
		{"networkpolicy-networking.k8s.io-v1", "networkpolicy", true},
		// unpluralized suffix (plural == singular)
		{"endpoints--v1", "endpoints", true},
		// grouped resource
		{"deployment-apps-v1", "deployments", true},
		{"deployment-apps-v1", "deployment", true},
		// non-matches must not over-match
		{"pod--v1", "poddisruptionbudgets", false},
		{"poddisruptionbudget-policy-v1", "pods", false},
		{"deployment-apps-v1", "pods", false},
		{"pod--v1", "svc", false},
		// empty inputs
		{"", "pods", false},
	}
	for _, tc := range cases {
		if got := resourceMatchesDir(tc.dir, tc.resource); got != tc.want {
			t.Errorf("resourceMatchesDir(%q, %q) = %v; want %v", tc.dir, tc.resource, got, tc.want)
		}
	}
}
