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
