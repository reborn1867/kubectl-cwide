package get

import (
	"testing"

	"github.com/kubectl-cwide/pkg/common"
	"github.com/kubectl-cwide/pkg/utils"
	printersinternal "k8s.io/kubernetes/pkg/printers/internalversion"
)

func dpCol(header string) Column {
	return Column{Header: header, FieldSpec: common.DefaultPrinterField}
}

// TestWantsWide covers the render-width decision behind get-parity: the
// default-printer table is generated Wide=true only when a template references
// a column that lives solely in the wide set.
func TestWantsWide(t *testing.T) {
	gen := utils.NewTableGenerator().With(printersinternal.AddHandlers)
	if len(gen.ResourceColumnDefinition("pod")) == 0 {
		t.Skip("pod handler not registered in this build")
	}

	// Non-wide-only template (the generated default): must NOT request wide,
	// so value-shaping like EXTERNAL-IP truncation matches `kubectl get`.
	narrow := &CustomColumnsPrinter{
		DefaultTableGenerator: gen,
		Columns: []Column{
			dpCol("NAME"), dpCol("READY"), dpCol("STATUS"), dpCol("RESTARTS"), dpCol("AGE"),
		},
	}
	if narrow.wantsWide("pod") {
		t.Errorf("non-wide pod template should not request wide rendering")
	}

	// References IP (a wide-only pod column) → must request wide.
	withWide := &CustomColumnsPrinter{
		DefaultTableGenerator: gen,
		Columns:               []Column{dpCol("NAME"), dpCol("IP")},
	}
	if !withWide.wantsWide("pod") {
		t.Errorf("template referencing wide-only column IP should request wide")
	}

	// No default-printer columns (pure JSONPath) → never needs wide.
	pureJSON := &CustomColumnsPrinter{
		DefaultTableGenerator: gen,
		Columns:               []Column{{Header: "NAME", FieldSpec: ".metadata.name"}},
	}
	if pureJSON.wantsWide("pod") {
		t.Errorf("pure-JSONPath template should not request wide")
	}

	// Unknown kind (e.g. a CRD with no registered handler) → false.
	if narrow.wantsWide("no-such-kind") {
		t.Errorf("unknown kind should not request wide")
	}

	// Nil generator → false (defensive).
	nilGen := &CustomColumnsPrinter{Columns: []Column{dpCol("IP")}}
	if nilGen.wantsWide("pod") {
		t.Errorf("nil generator should yield false")
	}
}

func TestNormalizeDefaultHeader(t *testing.T) {
	cases := map[string]string{
		"Nominated Node":  "NOMINATED_NODE",
		"IP":              "IP",
		"readiness gates": "READINESS_GATES",
	}
	for in, want := range cases {
		if got := normalizeDefaultHeader(in); got != want {
			t.Errorf("normalizeDefaultHeader(%q) = %q, want %q", in, got, want)
		}
	}
}
