package template

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestListKinds_ReturnsBundledKinds(t *testing.T) {
	kinds, err := listKinds()
	if err != nil {
		t.Fatalf("listKinds: %v", err)
	}
	// At least the four kinds shipped in this commit — assertion is
	// lower-bound to keep the test stable when future recipes are added.
	want := map[string]bool{
		"pod":                   true,
		"deployment":            true,
		"service":               true,
		"persistentvolumeclaim": true,
	}
	got := map[string]bool{}
	for _, k := range kinds {
		got[k] = true
	}
	for k := range want {
		if !got[k] {
			t.Errorf("kind %q missing from bundled scaffolds; got %v", k, kinds)
		}
	}
}

func TestRecipesForKind_PodHasRestartReason(t *testing.T) {
	got := recipesForKind("pod")
	found := false
	for _, r := range got {
		if r == "restart-reason" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("pod recipes missing 'restart-reason'; got %v", got)
	}
}

func TestRecipesForKind_UnknownKindReturnsNil(t *testing.T) {
	if r := recipesForKind("no-such-kind"); len(r) != 0 {
		t.Errorf("unknown kind should yield no recipes; got %v", r)
	}
}

func TestEmitBundledScaffold_KnownRecipe(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	if err := emitBundledScaffold(cmd, "pod", "restart-reason"); err != nil {
		t.Fatalf("emit: %v", err)
	}
	out := buf.String()
	// Sanity checks on the emitted YAML:
	// - has a columns block
	// - references at least one field the recipe is specifically about
	if !strings.Contains(out, "columns:") {
		t.Errorf("scaffold missing 'columns:' block:\n%s", out)
	}
	if !strings.Contains(out, "restartCount") {
		t.Errorf("restart-reason recipe should mention .status.containerStatuses[0].restartCount; got:\n%s", out)
	}
}

func TestEmitBundledScaffold_UnknownRecipeListsAvailable(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	err := emitBundledScaffold(cmd, "pod", "no-such-recipe")
	if err == nil {
		t.Fatal("expected error for unknown recipe")
	}
	msg := err.Error()
	if !strings.Contains(msg, "restart-reason") || !strings.Contains(msg, "available") {
		t.Errorf("error should suggest the available recipes; got: %v", err)
	}
}

func TestEmitBundledScaffold_UnknownKindGuidesToList(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	err := emitBundledScaffold(cmd, "no-such-kind", "anything")
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
	if !strings.Contains(err.Error(), "--list") {
		t.Errorf("error should point users at --list; got: %v", err)
	}
}

func TestListScaffolds_EmitsKindSlashRecipe(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	if err := listScaffolds(cmd); err != nil {
		t.Fatalf("listScaffolds: %v", err)
	}
	out := buf.String()
	// One recipe per line, in kind/recipe form so users can grep + xargs.
	if !strings.Contains(out, "pod/restart-reason\n") {
		t.Errorf("expected 'pod/restart-reason' line in --list output; got:\n%s", out)
	}
	if !strings.Contains(out, "deployment/rollout\n") {
		t.Errorf("expected 'deployment/rollout' line in --list output; got:\n%s", out)
	}
}
