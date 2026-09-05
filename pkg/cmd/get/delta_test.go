package get

import (
	"os"
	"strings"
	"testing"
)

// forceColorOn clears NO_COLOR for the duration of a test so Colorize emits
// escapes; delta highlighting is a no-op when color is disabled.
func forceColorOn(t *testing.T) {
	t.Helper()
	old, had := os.LookupEnv("NO_COLOR")
	os.Unsetenv("NO_COLOR")
	t.Cleanup(func() {
		if had {
			os.Setenv("NO_COLOR", old)
		}
	})
}

func hasANSI(s string) bool { return strings.Contains(s, "\x1b[") }

// TestDeltaTracker_FirstTickNoHighlight verifies the initial listing is never
// highlighted — there's nothing to diff against yet.
func TestDeltaTracker_FirstTickNoHighlight(t *testing.T) {
	forceColorOn(t)
	dt := newDeltaTracker([]string{"NAME", "STATUS", "AGE"})
	// Before markFirstTickDone, everything is baseline: no color.
	got := dt.decorate([]string{"web-1", "Running", "5m"})
	for _, c := range got {
		if hasANSI(c) {
			t.Fatalf("first-tick cell should not be colored: %q", c)
		}
	}
}

// TestDeltaTracker_ChangedCellHighlighted verifies that after the baseline, a
// cell whose value changed is colored and an unchanged cell is not.
func TestDeltaTracker_ChangedCellHighlighted(t *testing.T) {
	forceColorOn(t)
	dt := newDeltaTracker([]string{"NAME", "STATUS", "AGE"})
	dt.decorate([]string{"web-1", "Running", "5m"}) // baseline
	dt.markFirstTickDone()

	got := dt.decorate([]string{"web-1", "CrashLoopBackOff", "6m"})
	// NAME unchanged → plain
	if hasANSI(got[0]) {
		t.Errorf("unchanged NAME should be plain, got %q", got[0])
	}
	// STATUS changed → colored
	if !hasANSI(got[1]) {
		t.Errorf("changed STATUS should be colored, got %q", got[1])
	}
	if !strings.Contains(got[1], "CrashLoopBackOff") {
		t.Errorf("colored cell must still contain the value, got %q", got[1])
	}
	// AGE changed → colored
	if !hasANSI(got[2]) {
		t.Errorf("changed AGE should be colored, got %q", got[2])
	}
}

// TestDeltaTracker_NewRowAllGreen verifies a row key never seen before is
// highlighted in full (the "added" case), keyed by identity not position.
func TestDeltaTracker_NewRowAllGreen(t *testing.T) {
	forceColorOn(t)
	dt := newDeltaTracker([]string{"NAME", "STATUS"})
	dt.decorate([]string{"web-1", "Running"})
	dt.markFirstTickDone()

	got := dt.decorate([]string{"web-2", "Pending"}) // brand-new row
	for i, c := range got {
		if !hasANSI(c) {
			t.Errorf("new-row cell %d should be colored, got %q", i, c)
		}
	}
}

// TestDeltaTracker_RowIdentityByName verifies rows are correlated by NAME, so a
// row that reorders between ticks is not falsely flagged as changed.
func TestDeltaTracker_RowIdentityByName(t *testing.T) {
	forceColorOn(t)
	dt := newDeltaTracker([]string{"NAME", "STATUS"})
	// baseline: two rows
	dt.decorate([]string{"web-1", "Running"})
	dt.decorate([]string{"web-2", "Running"})
	dt.markFirstTickDone()

	// next tick: same rows, reordered, same values → nothing highlighted
	got2 := dt.decorate([]string{"web-2", "Running"})
	got1 := dt.decorate([]string{"web-1", "Running"})
	for _, c := range append(got1, got2...) {
		if hasANSI(c) {
			t.Errorf("reordered-but-unchanged row should be plain, got %q", c)
		}
	}
}

// TestDeltaTracker_NamespaceKey verifies NAMESPACE participates in the key so
// same-named resources in different namespaces don't collide.
func TestDeltaTracker_NamespaceKey(t *testing.T) {
	forceColorOn(t)
	dt := newDeltaTracker([]string{"NAMESPACE", "NAME", "STATUS"})
	dt.decorate([]string{"default", "web", "Running"})
	dt.markFirstTickDone()

	// Same NAME, different NAMESPACE → treated as a new row (all green), not a
	// change to the default/web row.
	got := dt.decorate([]string{"prod", "web", "Running"})
	if !hasANSI(got[2]) {
		t.Errorf("same name in a new namespace should be a new row (colored), got %q", got[2])
	}
}

// TestDeltaTracker_NilSafe verifies the tracker methods are safe on a nil
// receiver — the non-watch path leaves delta nil.
func TestDeltaTracker_NilSafe(t *testing.T) {
	var dt *deltaTracker
	dt.markFirstTickDone() // must not panic
	got := dt.decorate([]string{"a", "b"})
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("nil tracker must return cells unchanged, got %v", got)
	}
}

// TestDeltaTracker_ColorDisabledNoEscapes verifies NO_COLOR fully disables
// highlighting even after a real change — output is byte-identical to input.
func TestDeltaTracker_ColorDisabledNoEscapes(t *testing.T) {
	os.Setenv("NO_COLOR", "1")
	t.Cleanup(func() { os.Unsetenv("NO_COLOR") })

	dt := newDeltaTracker([]string{"NAME", "STATUS"})
	dt.decorate([]string{"web-1", "Running"})
	dt.markFirstTickDone()
	got := dt.decorate([]string{"web-1", "Failed"})
	for _, c := range got {
		if hasANSI(c) {
			t.Fatalf("NO_COLOR must suppress all escapes, got %q", c)
		}
	}
}
