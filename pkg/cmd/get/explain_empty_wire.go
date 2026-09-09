package get

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"

	"github.com/kubectl-cwide/pkg/common"
)

// explainEmptyBudget caps how many per-object diagnostic blocks are printed so
// a fully-empty column on a large list doesn't flood the terminal.
const explainEmptyBudget = 20

// explainEmptyColumn implements --explain-empty: for the named column, find
// every object whose rendered value is empty and print a diagnosis of why
// (which path segment is missing / whether the value is genuinely absent).
// Diagnostics go to stderr so the normal table on stdout stays pipeable.
//
// Only meaningful for JSONPath (fieldSpec) columns; template and
// default-printer columns are reported as untraceable and skipped.
func (o *GetOptions) explainEmptyColumn(printer *CustomColumnsPrinter, infos []*resource.Info) error {
	col, ok := findColumn(printer.Columns, o.ExplainEmpty)
	if !ok {
		headers := make([]string, 0, len(printer.Columns))
		for _, c := range printer.Columns {
			headers = append(headers, c.Header)
		}
		return fmt.Errorf("--explain-empty: unknown column %q; available: %s",
			o.ExplainEmpty, strings.Join(headers, ", "))
	}

	out := o.ErrOut

	if col.IsTemplate {
		fmt.Fprintf(out, "%s is a Go-template column; an empty result can't be traced to a single path.\n", col.Header)
		return nil
	}
	if isDefaultPrinterField(col.FieldSpec) {
		fmt.Fprintf(out, "%s is rendered by kubectl's default printer; nothing to trace.\n", col.Header)
		return nil
	}
	if col.FieldSpec == "" {
		fmt.Fprintf(out, "%s has no fieldSpec; nothing to trace.\n", col.Header)
		return nil
	}

	shown, emptyTotal := 0, 0
	for _, info := range infos {
		m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(info.Object)
		if err != nil {
			continue
		}
		if !fieldIsEmpty(col.FieldSpec, m) {
			continue
		}
		emptyTotal++
		if shown >= explainEmptyBudget {
			continue
		}
		d := diagnoseEmptyPath(col.FieldSpec, m)
		fmt.Fprint(out, FormatEmptyDiagnosis(col.Header, objectID(info, m), d))
		shown++
	}

	if emptyTotal == 0 {
		fmt.Fprintf(out, "%s: no empty cells found.\n", col.Header)
	} else if emptyTotal > shown {
		fmt.Fprintf(out, "... (%d more empty rows; showing first %d)\n", emptyTotal-shown, shown)
	}
	return nil
}

// findColumn locates a column by header, case-insensitively.
func findColumn(cols []Column, header string) (Column, bool) {
	want := strings.ToUpper(strings.TrimSpace(header))
	for _, c := range cols {
		if strings.ToUpper(c.Header) == want {
			return c, true
		}
	}
	return Column{}, false
}

// isDefaultPrinterField reports whether a fieldSpec is the default-printer
// sentinel, in either the bare (YAML) or brace-wrapped form.
func isDefaultPrinterField(spec string) bool {
	return spec == common.DefaultPrinterField || spec == "{."+common.DefaultPrinterField+"}"
}

// fieldIsEmpty renders the JSONPath against the object map and reports whether
// it yields no value — reusing the same segment walk the diagnosis uses so
// "empty" here matches what the diagnosis will explain. For plain paths this is
// exact; for untraceable (filter/wildcard) paths it conservatively treats the
// cell as non-empty (we can't cheaply evaluate it here, and diagnosing it would
// just say "untraceable").
func fieldIsEmpty(spec string, m map[string]interface{}) bool {
	d := diagnoseEmptyPath(spec, m)
	if d.Untraceable {
		return false
	}
	if d.FullyResolved {
		// Path resolves; treat as empty only when the resolved value is a
		// nil/empty leaf. diagnoseEmptyPath already set FullyResolved; re-walk
		// the final value to check emptiness.
		return leafValueEmpty(spec, m)
	}
	// A missing segment means the cell rendered empty.
	return true
}

// leafValueEmpty walks the plain path and reports whether the final value is
// nil, "", an empty slice, or an empty map. Assumes the path fully resolves
// (caller checked via diagnoseEmptyPath.FullyResolved).
func leafValueEmpty(spec string, m map[string]interface{}) bool {
	segs, ok := plainSegments(normalizeDotPath(spec))
	if !ok || len(segs) == 0 {
		return false
	}
	var cur interface{} = m
	for _, s := range segs {
		asMap, isMap := cur.(map[string]interface{})
		if !isMap {
			return false
		}
		v, present := asMap[s]
		if !present {
			return true
		}
		cur = v
	}
	switch v := cur.(type) {
	case nil:
		return true
	case string:
		return v == ""
	case []interface{}:
		return len(v) == 0
	case map[string]interface{}:
		return len(v) == 0
	default:
		return false
	}
}

// objectID builds a short "kind/name" (or "ns/name") identifier for diagnostic
// output, from the unstructured object.
func objectID(info *resource.Info, m map[string]interface{}) string {
	kind, _ := m["kind"].(string)
	name := info.Name
	if name == "" {
		if md, ok := m["metadata"].(map[string]interface{}); ok {
			name, _ = md["name"].(string)
		}
	}
	ns := info.Namespace
	switch {
	case kind != "" && name != "":
		if ns != "" {
			return fmt.Sprintf("%s/%s (ns %s)", strings.ToLower(kind), name, ns)
		}
		return fmt.Sprintf("%s/%s", strings.ToLower(kind), name)
	case name != "":
		return name
	default:
		return "<object>"
	}
}
