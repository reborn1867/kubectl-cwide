package alias

import (
	"strings"
	"testing"

	"github.com/kubectl-cwide/pkg/models"
	"gopkg.in/yaml.v3"
)

func TestAliasBundleRoundTrip(t *testing.T) {
	original := aliasBundle{
		Version: aliasBundleVersion,
		Aliases: map[string]string{"pd": "pods"},
		AliasEntries: map[string]models.AliasEntry{
			"core": {
				Resource:  "pod,svc,cm",
				Template:  "wide",
				Templates: map[string]string{"pod": "debug"},
			},
		},
	}
	data, err := yaml.Marshal(&original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back aliasBundle
	if err := yaml.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Version != aliasBundleVersion {
		t.Errorf("version lost: %d", back.Version)
	}
	if back.Aliases["pd"] != "pods" {
		t.Errorf("simple alias lost")
	}
	e, ok := back.AliasEntries["core"]
	if !ok {
		t.Fatalf("rich entry lost")
	}
	if e.Resource != "pod,svc,cm" || e.Template != "wide" || e.Templates["pod"] != "debug" {
		t.Errorf("rich entry fields corrupted: %#v", e)
	}
}

func TestAliasBundleVersionStamped(t *testing.T) {
	b := aliasBundle{Version: aliasBundleVersion}
	data, err := yaml.Marshal(&b)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "version:") {
		t.Errorf("expected version stamp in bundle, got:\n%s", data)
	}
}
