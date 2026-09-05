package get

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func newPodWithLabels(name string, labels map[string]string) *unstructured.Unstructured {
	m := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name": name,
		},
	}
	if labels != nil {
		lm := make(map[string]interface{}, len(labels))
		for k, v := range labels {
			lm[k] = v
		}
		m["metadata"].(map[string]interface{})["labels"] = lm
	}
	return &unstructured.Unstructured{Object: m}
}

// TestAppendLabelColumns_HeadersAndColumns verifies the printer gains one
// column per label key plus a trailing LABELS column, in order.
func TestAppendLabelColumns_HeadersAndColumns(t *testing.T) {
	p := &CustomColumnsPrinter{
		Columns: []Column{{Header: "NAME", FieldSpec: "{.metadata.name}"}},
		Headers: []string{"NAME"},
	}
	p.AppendLabelColumns([]string{"app", "tier"}, true)

	wantHeaders := []string{"NAME", "app", "tier", "LABELS"}
	if strings.Join(p.Headers, ",") != strings.Join(wantHeaders, ",") {
		t.Fatalf("headers = %v, want %v", p.Headers, wantHeaders)
	}
	if len(p.Columns) != 4 {
		t.Fatalf("columns len = %d, want 4", len(p.Columns))
	}
	if p.Columns[1].LabelKey != "app" || p.Columns[2].LabelKey != "tier" {
		t.Errorf("label keys not set correctly: %+v", p.Columns)
	}
	if !p.Columns[3].ShowLabels {
		t.Errorf("last column should be ShowLabels, got %+v", p.Columns[3])
	}
	if !p.Columns[1].isLabelColumn() || !p.Columns[3].isLabelColumn() {
		t.Errorf("label columns should report isLabelColumn()")
	}
	if p.Columns[0].isLabelColumn() {
		t.Errorf("NAME column must not be a label column")
	}
}

// TestAppendLabelColumns_Noop verifies no columns are added when there's
// nothing to add, and blank keys are skipped.
func TestAppendLabelColumns_Noop(t *testing.T) {
	p := &CustomColumnsPrinter{
		Columns: []Column{{Header: "NAME"}},
		Headers: []string{"NAME"},
	}
	p.AppendLabelColumns(nil, false)
	if len(p.Columns) != 1 || len(p.Headers) != 1 {
		t.Fatalf("expected no-op, got columns=%v headers=%v", p.Columns, p.Headers)
	}
	p.AppendLabelColumns([]string{"", "  "}, false)
	if len(p.Columns) != 1 {
		t.Fatalf("blank keys should be skipped, got %v", p.Columns)
	}
}

// TestObjectLabels verifies label extraction from an unstructured object.
func TestObjectLabels(t *testing.T) {
	obj := newPodWithLabels("web", map[string]string{"app": "web", "tier": "frontend"})
	got := objectLabels(obj)
	if got["app"] != "web" || got["tier"] != "frontend" {
		t.Fatalf("objectLabels = %v", got)
	}

	none := objectLabels(newPodWithLabels("bare", nil))
	if len(none) != 0 {
		t.Fatalf("expected no labels, got %v", none)
	}
}

// TestFormatAllLabels verifies the --show-labels rendering: sorted key=value
// pairs, and <none> for an empty map.
func TestFormatAllLabels(t *testing.T) {
	got := formatAllLabels(map[string]string{"tier": "frontend", "app": "web"})
	if got != "app=web,tier=frontend" {
		t.Fatalf("formatAllLabels = %q, want sorted app=web,tier=frontend", got)
	}
	if formatAllLabels(nil) != "<none>" {
		t.Fatalf("empty labels should render <none>, got %q", formatAllLabels(nil))
	}
}

// TestLabelColumns_EndToEnd renders an object through the printer with label
// columns via the RowSink capture path and checks the produced cells.
func TestLabelColumns_EndToEnd(t *testing.T) {
	var rows [][]string
	p := &CustomColumnsPrinter{
		Columns: []Column{{Header: "NAME", FieldSpec: "{.metadata.name}"}},
		Headers: []string{"NAME"},
		RowSink: func(cols []string) { rows = append(rows, append([]string(nil), cols...)) },
	}
	p.AppendLabelColumns([]string{"app", "missing"}, true)

	obj := newPodWithLabels("web-1", map[string]string{"app": "web"})
	if err := p.PrintObj(obj, nil); err != nil {
		t.Fatalf("PrintObj: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	row := rows[0]
	// [NAME, app, missing, LABELS]
	if len(row) != 4 {
		t.Fatalf("row = %v (len %d), want 4 cells", row, len(row))
	}
	if row[0] != "web-1" {
		t.Errorf("NAME = %q, want web-1", row[0])
	}
	if row[1] != "web" {
		t.Errorf("app label = %q, want web", row[1])
	}
	if row[2] != "" {
		t.Errorf("missing label should be empty, got %q", row[2])
	}
	if row[3] != "app=web" {
		t.Errorf("LABELS = %q, want app=web", row[3])
	}
}
