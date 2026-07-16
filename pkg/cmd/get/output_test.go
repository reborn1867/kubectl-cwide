package get

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestFilterRowsEquality(t *testing.T) {
	headers := []string{"NAME", "STATUS"}
	rows := [][]string{
		{"a", "Running"},
		{"b", "Pending"},
		{"c", "Running"},
	}
	got, err := filterRows(headers, rows, []string{"STATUS=Running"})
	if err != nil {
		t.Fatalf("filter err: %v", err)
	}
	want := [][]string{{"a", "Running"}, {"c", "Running"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFilterRowsRegex(t *testing.T) {
	headers := []string{"NAME"}
	rows := [][]string{{"pod-abc"}, {"svc-xyz"}, {"pod-def"}}
	got, err := filterRows(headers, rows, []string{"NAME~^pod-"})
	if err != nil {
		t.Fatalf("filter err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 pods, got %d", len(got))
	}
}

func TestFilterRowsNegation(t *testing.T) {
	headers := []string{"AGE"}
	rows := [][]string{{"1"}, {"2"}, {"3"}}
	got, err := filterRows(headers, rows, []string{"AGE!=2"})
	if err != nil {
		t.Fatalf("filter err: %v", err)
	}
	if len(got) != 2 || got[0][0] == "2" || got[1][0] == "2" {
		t.Fatalf("negation broken: %v", got)
	}
}

func TestFilterRowsUnknownColumn(t *testing.T) {
	_, err := filterRows([]string{"NAME"}, [][]string{{"a"}}, []string{"WAT=1"})
	if err == nil {
		t.Fatal("want error for unknown column, got nil")
	}
}

func TestSortRowsNumeric(t *testing.T) {
	headers := []string{"AGE"}
	rows := [][]string{{"10"}, {"2"}, {"100"}}
	if err := sortRows(headers, rows, "AGE"); err != nil {
		t.Fatalf("sort err: %v", err)
	}
	// numeric sort → 2, 10, 100
	want := [][]string{{"2"}, {"10"}, {"100"}}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("got %v, want %v", rows, want)
	}
}

func TestSortRowsLexical(t *testing.T) {
	headers := []string{"NAME"}
	rows := [][]string{{"charlie"}, {"alpha"}, {"bravo"}}
	if err := sortRows(headers, rows, "NAME"); err != nil {
		t.Fatalf("sort err: %v", err)
	}
	if rows[0][0] != "alpha" || rows[1][0] != "bravo" || rows[2][0] != "charlie" {
		t.Fatalf("lexical sort broken: %v", rows)
	}
}

func TestRenderCSV(t *testing.T) {
	var buf bytes.Buffer
	err := renderRows(&buf, "csv", []string{"NAME", "AGE"}, [][]string{{"a", "1"}, {"b,c", "2"}})
	if err != nil {
		t.Fatalf("csv render: %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "NAME,AGE\n") {
		t.Fatalf("no header line: %q", got)
	}
	if !strings.Contains(got, "\"b,c\",2") {
		t.Fatalf("comma value not quoted: %q", got)
	}
}

func TestRenderTemplateJSON(t *testing.T) {
	// Note: bare "json" is intercepted upstream by emitNative for raw
	// resource output. The template-driven json format is "template-json".
	var buf bytes.Buffer
	err := renderRows(&buf, "template-json", []string{"NAME"}, [][]string{{"a"}, {"b"}})
	if err != nil {
		t.Fatalf("template-json render: %v", err)
	}
	if !strings.Contains(buf.String(), `"NAME": "a"`) {
		t.Fatalf("unexpected json: %q", buf.String())
	}
}
