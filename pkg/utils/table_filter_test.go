package utils

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	printersinternal "k8s.io/kubernetes/pkg/printers/internalversion"
)

// hasCol reports whether a column with the given (case-sensitive) name is
// present in the definition list.
func hasCol(cols []metav1.TableColumnDefinition, name string) bool {
	for _, c := range cols {
		if c.Name == name {
			return true
		}
	}
	return false
}

// TestResourceColumnDefinitionFiltered_DropsWideColumns is the regression test
// for "kc cwide get pod == kubectl get pod (not -o wide)". The default template
// is built from the non-wide column set, so wide-only pod columns (IP, Node,
// Nominated Node, Readiness Gates — all Priority != 0) must be excluded from
// the filtered set while the base columns remain.
func TestResourceColumnDefinitionFiltered_DropsWideColumns(t *testing.T) {
	g := NewTableGenerator().With(printersinternal.AddHandlers)

	all := g.ResourceColumnDefinition("pod")
	if len(all) == 0 {
		t.Skip("pod handler not registered in this build; skipping")
	}

	narrow := g.ResourceColumnDefinitionFiltered("pod", false)
	wide := g.ResourceColumnDefinitionFiltered("pod", true)

	// wide == the full set.
	if len(wide) != len(all) {
		t.Errorf("wide filtered set (%d) should equal full set (%d)", len(wide), len(all))
	}
	// narrow must be a strict subset (pods have wide-only columns).
	if len(narrow) >= len(all) {
		t.Errorf("narrow set (%d) should be smaller than full set (%d) — wide columns not dropped", len(narrow), len(all))
	}

	// Base columns kubectl shows without -o wide must be present.
	for _, want := range []string{"Name", "Ready", "Status", "Restarts", "Age"} {
		if !hasCol(narrow, want) {
			t.Errorf("base column %q missing from narrow set", want)
		}
	}
	// Wide-only columns must be absent from narrow but present in the full set.
	for _, wideOnly := range []string{"IP", "Node", "Nominated Node", "Readiness Gates"} {
		if hasCol(narrow, wideOnly) {
			t.Errorf("wide-only column %q must be excluded from the default (narrow) set", wideOnly)
		}
		if !hasCol(all, wideOnly) {
			t.Errorf("sanity: wide-only column %q should exist in the full set", wideOnly)
		}
	}

	// Every narrow column must have Priority 0 (that's the definition of
	// non-wide), and no narrow column should be missing from the full set.
	for _, c := range narrow {
		if c.Priority != 0 {
			t.Errorf("narrow column %q has non-zero priority %d", c.Name, c.Priority)
		}
	}
}

// TestResourceColumnDefinitionFiltered_UnknownKind returns nil for an
// unregistered kind, matching ResourceColumnDefinition.
func TestResourceColumnDefinitionFiltered_UnknownKind(t *testing.T) {
	g := NewTableGenerator().With(printersinternal.AddHandlers)
	if got := g.ResourceColumnDefinitionFiltered("no-such-kind", false); got != nil {
		t.Errorf("unknown kind should yield nil, got %v", got)
	}
}
