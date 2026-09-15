package get

import (
	"io"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"
	printersinternal "k8s.io/kubernetes/pkg/printers/internalversion"

	"github.com/kubectl-cwide/pkg/common"
	"github.com/kubectl-cwide/pkg/utils"
)

func nsPod(namespace, name string) *unstructured.Unstructured {
	meta := map[string]interface{}{"name": name}
	if namespace != "" {
		meta["namespace"] = namespace
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   meta,
	}}
}

// TestWithNamespaceColumn_PrependsAndRenders verifies the NAMESPACE column is
// added as the leftmost column and renders metadata.namespace, matching
// `kubectl get -A`.
func TestWithNamespaceColumn_PrependsAndRenders(t *testing.T) {
	var rows [][]string
	p := &CustomColumnsPrinter{
		Columns: []Column{{Header: "NAME", FieldSpec: "{.metadata.name}"}},
		Headers: []string{"NAME"},
		RowSink: func(cols []string) { rows = append(rows, append([]string(nil), cols...)) },
	}
	p.WithNamespaceColumn()

	// NAMESPACE must be first in both Columns and Headers.
	if len(p.Headers) != 2 || p.Headers[0] != "NAMESPACE" || p.Headers[1] != "NAME" {
		t.Fatalf("headers = %v, want [NAMESPACE NAME]", p.Headers)
	}
	if p.Columns[0].Header != "NAMESPACE" || p.Columns[0].FieldSpec != "{.metadata.namespace}" {
		t.Fatalf("first column not the namespace column: %+v", p.Columns[0])
	}

	if err := p.PrintObj(nsPod("kube-system", "coredns"), io.Discard); err != nil {
		t.Fatalf("PrintObj: %v", err)
	}
	if len(rows) != 1 || len(rows[0]) != 2 {
		t.Fatalf("expected one 2-cell row, got %v", rows)
	}
	if rows[0][0] != "kube-system" {
		t.Errorf("NAMESPACE cell = %q, want kube-system", rows[0][0])
	}
	if rows[0][1] != "coredns" {
		t.Errorf("NAME cell = %q, want coredns", rows[0][1])
	}
}

// TestWithNamespaceColumn_Idempotent verifies calling it when a NAMESPACE
// column already exists is a no-op (no duplicate).
func TestWithNamespaceColumn_Idempotent(t *testing.T) {
	p := &CustomColumnsPrinter{
		Columns: []Column{
			{Header: "NAMESPACE", FieldSpec: "{.metadata.namespace}"},
			{Header: "NAME", FieldSpec: "{.metadata.name}"},
		},
		Headers: []string{"NAMESPACE", "NAME"},
	}
	p.WithNamespaceColumn()
	if len(p.Headers) != 2 {
		t.Fatalf("expected no duplicate NAMESPACE, headers = %v", p.Headers)
	}
}

// TestWithNamespaceColumn_MixedWithDefaultPrinter verifies the prepended
// NAMESPACE column composes with a $_defaultPrinterField column — the common
// `kc cwide get <alias> -A` case where the alias resolves to an init-generated
// default template. NAMESPACE (a plain JSONPath column) must stay leftmost and
// render metadata.namespace regardless of the default-printer column beside it.
func TestWithNamespaceColumn_MixedWithDefaultPrinter(t *testing.T) {
	var rows [][]string
	p := &CustomColumnsPrinter{
		DefaultTableGenerator: utils.NewTableGenerator().With(printersinternal.AddHandlers),
		Columns: []Column{
			{Header: "NAME", FieldSpec: common.DefaultPrinterField},
		},
		Headers: []string{"NAME"},
		RowSink: func(cols []string) { rows = append(rows, append([]string(nil), cols...)) },
	}
	p.WithNamespaceColumn()

	if p.Headers[0] != "NAMESPACE" {
		t.Fatalf("NAMESPACE must be leftmost even beside a default-printer column; headers=%v", p.Headers)
	}
	// The NAME default-printer column keeps needsDefaultPrinter true; the plain
	// JSONPath NAMESPACE column doesn't change that.
	if !p.needsDefaultPrinter() {
		t.Errorf("needsDefaultPrinter should stay true (NAME is a default-printer column)")
	}

	if err := p.PrintObj(nsPod("monitoring", "prometheus-0"), io.Discard); err != nil {
		t.Fatalf("PrintObj: %v", err)
	}
	// NAMESPACE cell (leftmost) renders metadata.namespace directly.
	if len(rows) != 1 || rows[0][0] != "monitoring" {
		t.Fatalf("NAMESPACE cell should render metadata.namespace first, got rows=%v", rows)
	}
}

func TestObjectsAreNamespaced(t *testing.T) {
	namespaced := []*resource.Info{{Object: nsPod("default", "web"), Namespace: "default"}}
	if !objectsAreNamespaced(namespaced) {
		t.Errorf("namespaced objects should report true")
	}

	// Cluster-scoped: no namespace on object or info.
	clusterScoped := []*resource.Info{{Object: nsPod("", "node-1")}}
	if objectsAreNamespaced(clusterScoped) {
		t.Errorf("cluster-scoped objects should report false")
	}

	if objectsAreNamespaced(nil) {
		t.Errorf("empty infos should report false")
	}
}
