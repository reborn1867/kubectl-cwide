package get

import (
	"io"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestFormatMetaMap(t *testing.T) {
	// sorted key=value pairs
	got := formatMetaMap(map[string]string{"tier": "front", "app": "web"})
	if got != "app=web,tier=front" {
		t.Errorf("formatMetaMap = %q, want app=web,tier=front", got)
	}
	// empty → <none>
	if formatMetaMap(nil) != "<none>" {
		t.Errorf("empty map should render <none>, got %q", formatMetaMap(nil))
	}
}

func TestWithMetaColumn(t *testing.T) {
	p := &CustomColumnsPrinter{
		Columns: []Column{{Header: "NAME", FieldSpec: "{.metadata.name}"}},
		Headers: []string{"NAME"},
	}
	p.WithMetaColumn("labels")
	p.WithMetaColumn("annotations")

	if len(p.Columns) != 3 || len(p.Headers) != 3 {
		t.Fatalf("expected 3 columns/headers, got %d/%d", len(p.Columns), len(p.Headers))
	}
	if p.Columns[1].Header != "LABELS" || p.Columns[1].MetaKind != "labels" {
		t.Errorf("labels column wrong: %+v", p.Columns[1])
	}
	if p.Columns[2].Header != "ANNOTATIONS" || p.Columns[2].MetaKind != "annotations" {
		t.Errorf("annotations column wrong: %+v", p.Columns[2])
	}
	// idempotent: calling again doesn't duplicate
	p.WithMetaColumn("labels")
	if len(p.Columns) != 3 {
		t.Errorf("WithMetaColumn should be idempotent, got %d columns", len(p.Columns))
	}
}

// TestShowMeta_EndToEnd renders an object with LABELS and ANNOTATIONS columns
// via the RowSink path and checks the produced cells.
func TestShowMeta_EndToEnd(t *testing.T) {
	var rows [][]string
	p := &CustomColumnsPrinter{
		Columns: []Column{{Header: "NAME", FieldSpec: "{.metadata.name}"}},
		Headers: []string{"NAME"},
		RowSink: func(cols []string) { rows = append(rows, append([]string(nil), cols...)) },
	}
	p.WithMetaColumn("labels")
	p.WithMetaColumn("annotations")

	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]interface{}{
			"name":        "web-1",
			"labels":      map[string]interface{}{"app": "web"},
			"annotations": map[string]interface{}{"team": "sre"},
		},
	}}
	if err := p.PrintObj(obj, io.Discard); err != nil {
		t.Fatalf("PrintObj: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	row := rows[0] // [NAME, LABELS, ANNOTATIONS]
	if len(row) != 3 {
		t.Fatalf("row = %v (len %d), want 3 cells", row, len(row))
	}
	if row[0] != "web-1" {
		t.Errorf("NAME = %q, want web-1", row[0])
	}
	if !strings.Contains(row[1], "app=web") {
		t.Errorf("LABELS = %q, want to contain app=web", row[1])
	}
	if !strings.Contains(row[2], "team=sre") {
		t.Errorf("ANNOTATIONS = %q, want to contain team=sre", row[2])
	}
}
