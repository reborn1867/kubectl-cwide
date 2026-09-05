package template

import (
	"os"
	"strings"
	"testing"
)

func TestUnifiedDiff_IdenticalIsEmpty(t *testing.T) {
	got := unifiedDiff("hello\nworld\n", "hello\nworld\n", "a", "b", 3, false)
	if got != "" {
		t.Fatalf("identical inputs should diff to empty; got:\n%s", got)
	}
}

func TestUnifiedDiff_EmitsUnifiedHeader(t *testing.T) {
	got := unifiedDiff("hello\n", "world\n", "left.yaml", "right.yaml", 3, false)
	if !strings.HasPrefix(got, "--- left.yaml\n+++ right.yaml\n") {
		t.Fatalf("expected unified header; got:\n%s", got)
	}
	if !strings.Contains(got, "-hello") {
		t.Errorf("expected '-hello'; got:\n%s", got)
	}
	if !strings.Contains(got, "+world") {
		t.Errorf("expected '+world'; got:\n%s", got)
	}
}

func TestUnifiedDiff_SingleLineChangeIncludesContext(t *testing.T) {
	a := "one\ntwo\nthree\nfour\nfive\n"
	b := "one\ntwo\nTHREE\nfour\nfive\n"
	got := unifiedDiff(a, b, "a", "b", 3, false)
	if !strings.Contains(got, " one\n") {
		t.Errorf("expected leading context ' one'; got:\n%s", got)
	}
	if !strings.Contains(got, "-three\n") {
		t.Errorf("expected '-three'; got:\n%s", got)
	}
	if !strings.Contains(got, "+THREE\n") {
		t.Errorf("expected '+THREE'; got:\n%s", got)
	}
	if !strings.Contains(got, "@@ ") {
		t.Errorf("expected hunk header; got:\n%s", got)
	}
}

func TestUnifiedDiff_ByteStableAcrossRuns(t *testing.T) {
	// Byte-stability matters because users may diff in CI and grep for
	// changes. No timestamps, no clocks — same inputs → same bytes.
	a := "x\ny\nz\n"
	b := "x\nY\nz\n"
	first := unifiedDiff(a, b, "a", "b", 3, false)
	second := unifiedDiff(a, b, "a", "b", 3, false)
	if first != second {
		t.Fatalf("unifiedDiff must be byte-stable; got:\nfirst:%s\nsecond:%s", first, second)
	}
}

func TestUnifiedDiff_PlainHasNoANSI(t *testing.T) {
	// color=false must never emit escapes, regardless of NO_COLOR state.
	got := unifiedDiff("a\n", "b\n", "l", "r", 3, false)
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("plain diff must contain no ANSI escapes; got:\n%q", got)
	}
}

func TestUnifiedDiff_ColorWrapsAddRemoveAndHeaders(t *testing.T) {
	// color=true wraps -/+/@@ lines in escapes but keeps the leading +/-/space
	// marker intact so the output still reads as a unified diff.
	os.Unsetenv("NO_COLOR") // ensure the shared color gate is open
	got := unifiedDiff("hello\n", "world\n", "l", "r", 3, true)
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("colored diff should contain ANSI escapes; got:\n%q", got)
	}
	// Removed and added markers still present after the escape prefix.
	if !strings.Contains(got, "-hello") || !strings.Contains(got, "+world") {
		t.Errorf("colored diff must retain -/+ line markers; got:\n%q", got)
	}
	if !strings.Contains(got, "@@ ") {
		t.Errorf("colored diff must retain hunk header; got:\n%q", got)
	}
}

func TestUnifiedDiff_ColorRespectsNoColor(t *testing.T) {
	// Even with color=true requested, NO_COLOR forces plain output via the
	// shared funcs gate.
	os.Setenv("NO_COLOR", "1")
	t.Cleanup(func() { os.Unsetenv("NO_COLOR") })
	got := unifiedDiff("hello\n", "world\n", "l", "r", 3, true)
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("NO_COLOR must suppress escapes even when color requested; got:\n%q", got)
	}
}

func TestIsTerminal_NonFileWriterIsFalse(t *testing.T) {
	if isTerminal(&strings.Builder{}) {
		t.Fatal("a non-*os.File writer must be treated as non-terminal")
	}
}

func TestSplitKeepNewline_PreservesTrailingNewlines(t *testing.T) {
	got := splitKeepNewline("a\nb\nc\n")
	want := []string{"a\n", "b\n", "c\n"}
	if len(got) != len(want) {
		t.Fatalf("wrong count: got %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d: got %q; want %q", i, got[i], want[i])
		}
	}
}

func TestSplitKeepNewline_HandlesNoTrailingNewline(t *testing.T) {
	got := splitKeepNewline("a\nb")
	want := []string{"a\n", "b"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v; want %v", got, want)
	}
}

func TestSplitKeepNewline_EmptyInputIsNil(t *testing.T) {
	if got := splitKeepNewline(""); got != nil {
		t.Fatalf("empty input should yield nil; got %v", got)
	}
}

func TestLoadBundledRecipe_KnownRecipe(t *testing.T) {
	data, err := loadBundledRecipe("pod/restart-reason")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(string(data), "columns:") {
		t.Errorf("expected recipe to be a YAML template with columns; got:\n%s", data)
	}
}

func TestLoadBundledRecipe_UnknownRecipeListsAvailable(t *testing.T) {
	_, err := loadBundledRecipe("pod/no-such-recipe")
	if err == nil {
		t.Fatal("expected error for unknown recipe")
	}
	if !strings.Contains(err.Error(), "restart-reason") || !strings.Contains(err.Error(), "available") {
		t.Errorf("error should list available recipes; got: %v", err)
	}
}

func TestLoadBundledRecipe_UnknownKind(t *testing.T) {
	_, err := loadBundledRecipe("no-such-kind/recipe")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no bundled recipes") {
		t.Errorf("unknown-kind error should be specific; got: %v", err)
	}
}

func TestLoadBundledRecipe_MalformedSpec(t *testing.T) {
	cases := []string{"", "bare", "/", "kind/", "/recipe"}
	for _, c := range cases {
		if _, err := loadBundledRecipe(c); err == nil {
			t.Errorf("expected error for malformed spec %q", c)
		}
	}
}
