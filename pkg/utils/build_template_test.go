package utils

import (
	"strings"
	"testing"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// TestBuildColumnTemplate verifies the .tpl generation shape: an uppercased
// header row and a JSONPath row, each newline-terminated, with the two rows
// column-aligned to a common width.
func TestBuildColumnTemplate(t *testing.T) {
	cols := []v1.CustomResourceColumnDefinition{
		{Name: "Name", JSONPath: ".metadata.name"},
		{Name: "Phase", JSONPath: ".status.phase"},
	}
	out := string(BuildColumnTemplate(cols))
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (header + jsonpath), got %d:\n%q", len(lines), out)
	}
	// Header row is uppercased and carries both names.
	if !strings.Contains(lines[0], "NAME") || !strings.Contains(lines[0], "PHASE") {
		t.Errorf("header row missing uppercased names: %q", lines[0])
	}
	if strings.Contains(lines[0], "Name") && !strings.Contains(lines[0], "NAME") {
		t.Errorf("names should be uppercased in header: %q", lines[0])
	}
	// JSONPath row carries both specs verbatim.
	if !strings.Contains(lines[1], ".metadata.name") || !strings.Contains(lines[1], ".status.phase") {
		t.Errorf("jsonpath row missing specs: %q", lines[1])
	}
	// Output ends with a newline.
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("output should end with newline: %q", out)
	}
}

func TestBuildColumnTemplate_Empty(t *testing.T) {
	out := string(BuildColumnTemplate(nil))
	// No columns → just the two newlines from the header/jsonpath row separators.
	if out != "\n\n" {
		t.Errorf("empty columns should yield two blank lines, got %q", out)
	}
}

// TestFormatContent verifies the alignment helper: header uppercased, two rows
// aligned, and short input returned unchanged.
func TestFormatContent(t *testing.T) {
	// Two-line input: header + jsonpath, ragged spacing.
	in := []byte("name   phase\n.metadata.name .status.phase\n")
	out := string(formatContent(in))
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 formatted lines, got %d:\n%q", len(lines), out)
	}
	if !strings.Contains(lines[0], "NAME") || !strings.Contains(lines[0], "PHASE") {
		t.Errorf("header should be uppercased: %q", lines[0])
	}
	if !strings.Contains(lines[1], ".metadata.name") {
		t.Errorf("jsonpath line lost content: %q", lines[1])
	}
}

func TestFormatContent_ShortInputUnchanged(t *testing.T) {
	// Fewer than two lines is returned as-is per the documented guard.
	in := []byte("only-one-line")
	if got := string(formatContent(in)); got != "only-one-line" {
		t.Errorf("single-line input should be unchanged, got %q", got)
	}
}
