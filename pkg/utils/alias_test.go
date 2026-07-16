package utils

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/kubectl-cwide/pkg/common"
	"github.com/kubectl-cwide/pkg/models"
)

// withTempHome points UserHomeDir at a temp dir seeded with the given alias
// map and returns a cleanup func. LoadConfig reads from $HOME/<common.ConfigPath>.
func withTempHome(t *testing.T, aliases map[string]string) func() {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".kubectl-cwide"), 0755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, common.ConfigPath)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0755); err != nil {
		t.Fatal(err)
	}
	data, err := yaml.Marshal(&models.Config{Aliases: aliases})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	return func() { os.Setenv("HOME", oldHome) }
}

func TestResolveAlias_BareType(t *testing.T) {
	defer withTempHome(t, map[string]string{"pd": "pods"})()
	got := ResolveAlias([]string{"pd"})
	if len(got) != 1 || got[0] != "pods" {
		t.Errorf("bare TYPE: got %v, want [pods]", got)
	}
}

func TestResolveAlias_TypeName(t *testing.T) {
	defer withTempHome(t, map[string]string{"pd": "pods"})()
	got := ResolveAlias([]string{"pd", "my-pod"})
	want := []string{"pods", "my-pod"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("TYPE NAME: got %v, want %v", got, want)
	}
}

// Regression: reported bug — 'rr/alicloud-…' failed because the slash form
// was passed through the resource builder verbatim without resolving 'rr'.
func TestResolveAlias_TypeSlashName(t *testing.T) {
	defer withTempHome(t, map[string]string{"rr": "resources.resource.sap.com"})()
	got := ResolveAlias([]string{"rr/alicloud-1336065331941288"})
	want := "resources.resource.sap.com/alicloud-1336065331941288"
	if len(got) != 1 || got[0] != want {
		t.Errorf("TYPE/NAME: got %v, want [%s]", got, want)
	}
}

func TestResolveAlias_TypeSlashName_WithMoreArgs(t *testing.T) {
	defer withTempHome(t, map[string]string{"pd": "pods"})()
	got := ResolveAlias([]string{"pd/my-pod", "-o", "yaml"})
	want := []string{"pods/my-pod", "-o", "yaml"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("TYPE/NAME + flags: got %v, want %v", got, want)
			break
		}
	}
}

func TestResolveAlias_NoMatch(t *testing.T) {
	defer withTempHome(t, map[string]string{"pd": "pods"})()
	got := ResolveAlias([]string{"deployments"})
	if len(got) != 1 || got[0] != "deployments" {
		t.Errorf("no-match: got %v, want [deployments]", got)
	}
}

func TestResolveAliasString_TypeSlashName(t *testing.T) {
	defer withTempHome(t, map[string]string{"rr": "resources.resource.sap.com"})()
	got := ResolveAliasString("rr/foo")
	want := "resources.resource.sap.com/foo"
	if got != want {
		t.Errorf("ResolveAliasString: got %q, want %q", got, want)
	}
}
