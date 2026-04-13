package get

import (
	"bytes"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

// testDecoder returns a minimal runtime.Decoder for tests.
func testDecoder() runtime.Decoder {
	s := runtime.NewScheme()
	return serializer.NewCodecFactory(s).UniversalDecoder()
}

// testObj builds an unstructured Kubernetes object from a map.
func testObj(data map[string]interface{}) runtime.Object {
	obj := &unstructured.Unstructured{Object: data}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"})
	return obj
}

func TestCustomFuncs_SingleArg(t *testing.T) {
	yamlTmpl := []byte(`
funcs:
  statusIcon: '{{ if eq . "Running" }}OK{{ else }}WARN{{ end }}'
columns:
  - header: STATUS
    template: '{{ statusIcon (index .status "phase") }}'
`)

	printer, err := NewCustomColumnsPrinterFromYAML(yamlTmpl, testDecoder(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj := testObj(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]interface{}{"name": "test-pod"},
		"status":     map[string]interface{}{"phase": "Running"},
	})

	var buf bytes.Buffer
	if err := printer.PrintObj(obj, &buf); err != nil {
		t.Fatalf("PrintObj error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "OK") {
		t.Errorf("expected output to contain 'OK', got: %s", output)
	}
	if strings.Contains(output, "WARN") {
		t.Errorf("expected output NOT to contain 'WARN', got: %s", output)
	}
}

func TestCustomFuncs_MultipleArgs(t *testing.T) {
	yamlTmpl := []byte(`
funcs:
  pair: '{{ index . 0 }}/{{ index . 1 }}'
columns:
  - header: PAIR
    template: '{{ pair .metadata.name .metadata.namespace }}'
`)

	printer, err := NewCustomColumnsPrinterFromYAML(yamlTmpl, testDecoder(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj := testObj(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]interface{}{"name": "my-pod", "namespace": "default"},
	})

	var buf bytes.Buffer
	if err := printer.PrintObj(obj, &buf); err != nil {
		t.Fatalf("PrintObj error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	// Second line (after header) should contain "my-pod/default"
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got: %s", output)
	}
	if !strings.Contains(lines[1], "my-pod/default") {
		t.Errorf("expected 'my-pod/default', got: %s", lines[1])
	}
}

func TestCustomFuncs_FuncCallingFunc(t *testing.T) {
	yamlTmpl := []byte(`
funcs:
  wrap: '[{{ . }}]'
  doubleWrap: '{{ wrap . | wrap }}'
columns:
  - header: VAL
    template: '{{ doubleWrap .metadata.name }}'
`)

	printer, err := NewCustomColumnsPrinterFromYAML(yamlTmpl, testDecoder(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj := testObj(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]interface{}{"name": "x"},
	})

	var buf bytes.Buffer
	if err := printer.PrintObj(obj, &buf); err != nil {
		t.Fatalf("PrintObj error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got: %s", output)
	}
	if !strings.Contains(lines[1], "[[x]]") {
		t.Errorf("expected '[[x]]', got: %s", lines[1])
	}
}

func TestCustomFuncs_WithSprig(t *testing.T) {
	yamlTmpl := []byte(`
funcs:
  shortImage: '{{ . | splitList "/" | last }}'
columns:
  - header: IMAGE
    template: '{{ shortImage "docker.io/library/nginx" }}'
`)

	printer, err := NewCustomColumnsPrinterFromYAML(yamlTmpl, testDecoder(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj := testObj(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]interface{}{"name": "test"},
	})

	var buf bytes.Buffer
	if err := printer.PrintObj(obj, &buf); err != nil {
		t.Fatalf("PrintObj error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got: %s", output)
	}
	if !strings.Contains(lines[1], "nginx") {
		t.Errorf("expected 'nginx', got: %s", lines[1])
	}
}

func TestCustomFuncs_WithHelpers(t *testing.T) {
	yamlTmpl := []byte(`
helpers: |
  {{ define "suffix" }}-suffix{{ end }}
funcs:
  decorated: '{{ . }}{{ template "suffix" }}'
columns:
  - header: NAME
    template: '{{ decorated .metadata.name }}'
`)

	printer, err := NewCustomColumnsPrinterFromYAML(yamlTmpl, testDecoder(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj := testObj(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]interface{}{"name": "pod1"},
	})

	var buf bytes.Buffer
	if err := printer.PrintObj(obj, &buf); err != nil {
		t.Fatalf("PrintObj error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got: %s", output)
	}
	if !strings.Contains(lines[1], "pod1-suffix") {
		t.Errorf("expected 'pod1-suffix', got: %s", lines[1])
	}
}

func TestCustomFuncs_InvalidBody(t *testing.T) {
	yamlTmpl := []byte(`
funcs:
  bad: '{{ if }}'
columns:
  - header: X
    template: '{{ bad "a" }}'
`)

	_, err := NewCustomColumnsPrinterFromYAML(yamlTmpl, testDecoder(), nil)
	if err == nil {
		t.Fatal("expected error for invalid func body, got nil")
	}
	if !strings.Contains(err.Error(), "bad") {
		t.Errorf("expected error to mention func name 'bad', got: %v", err)
	}
}

func TestCustomFuncs_NoFuncsField(t *testing.T) {
	yamlTmpl := []byte(`
columns:
  - header: NAME
    fieldSpec: .metadata.name
`)

	printer, err := NewCustomColumnsPrinterFromYAML(yamlTmpl, testDecoder(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj := testObj(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]interface{}{"name": "test"},
	})

	var buf bytes.Buffer
	if err := printer.PrintObj(obj, &buf); err != nil {
		t.Fatalf("PrintObj error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if !strings.Contains(output, "test") {
		t.Errorf("expected 'test', got: %s", output)
	}
}
