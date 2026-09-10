package get

import (
	"io"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"
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
