package template

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestLintGoodYAML(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.yaml")
	body := `columns:
  - header: NAME
    fieldSpec: .metadata.name
  - header: STATUS
    fieldSpec: .status.phase
`
	if err := os.WriteFile(good, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	if err := lintOne(cmd, good); err != nil {
		t.Fatalf("good template errored: %v", err)
	}
}

func TestLintMissingHeader(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.yaml")
	body := `columns:
  - fieldSpec: .metadata.name
`
	if err := os.WriteFile(bad, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	if err := lintOne(cmd, bad); err == nil {
		t.Fatal("missing header should have failed lint")
	}
}

func TestLintUnsupportedExt(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.md")
	if err := os.WriteFile(bad, []byte("nope"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	if err := lintOne(cmd, bad); err == nil {
		t.Fatal("wrong extension should have failed")
	}
}

// lintTreeSetup builds a tiny two-resource template tree with one bad file,
// mirroring the layout that `cwide init` produces.
func lintTreeSetup(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, sub := range []string{"pod--v1", "service--v1", "_shared"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0755); err != nil {
			t.Fatal(err)
		}
	}
	writes := map[string]string{
		"pod--v1/default.yaml":     "columns:\n  - header: NAME\n    fieldSpec: .metadata.name\n",
		"pod--v1/debug.yaml":       "columns:\n  - header: STATUS\n    fieldSpec: .status.phase\n",
		"service--v1/default.yaml": "columns:\n  - header: NAME\n    fieldSpec: .metadata.name\n",
		// Missing-header case — should fail lint and count toward failCount.
		"pod--v1/broken.yaml": "columns:\n  - fieldSpec: .metadata.name\n",
		// _shared fragment: skipped by the walker (would otherwise be
		// counted as 'no columns' fail).
		"_shared/helpers.tpl": "not a full template",
		// Non-template file next to templates: skipped by extension filter.
		"pod--v1/README.md": "docs",
	}
	for rel, body := range writes {
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestLintTree_CountsOkAndFail(t *testing.T) {
	root := lintTreeSetup(t)
	cmd := &cobra.Command{}
	err := lintTree(cmd, root)
	if err == nil {
		t.Fatal("tree with a broken template should return an error")
	}
	if !contains(err.Error(), "failed lint") {
		t.Errorf("error should mention 'failed lint'; got %v", err)
	}
}

func TestLintTree_SkipsSharedDir(t *testing.T) {
	root := lintTreeSetup(t)
	// Remove the broken file so the walk should succeed if _shared is
	// truly skipped. If the walker DIDN'T skip _shared, helpers.tpl
	// would be linted (with < 2 lines) and fail.
	if err := os.Remove(filepath.Join(root, "pod--v1", "broken.yaml")); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	if err := lintTree(cmd, root); err != nil {
		t.Fatalf("_shared should be skipped, tree should be clean; got %v", err)
	}
}

func TestLintTree_SingleFilePathIsBenignNoop(t *testing.T) {
	// --recursive against a file path should just lint that file.
	root := lintTreeSetup(t)
	one := filepath.Join(root, "pod--v1", "default.yaml")
	cmd := &cobra.Command{}
	if err := lintTree(cmd, one); err != nil {
		t.Fatalf("single-file --recursive: %v", err)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}()))
}
