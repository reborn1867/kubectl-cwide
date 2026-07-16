package get

import (
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
)

// nativeOutputFormats are the formats that emit the raw resource object
// (a la `kubectl get -o …`) rather than rendered template columns.
// Anything with a "jsonpath" or "go-template" prefix also delegates natively.
var nativeOutputFormats = map[string]bool{
	"yaml":         true,
	"json":         true,
	"name":         true,
	"wide":         true,
	"jsonpath":     true,
	"jsonpath-file": true,
	"go-template":  true,
	"go-template-file": true,
}

// isNativeOutput reports whether the given -o value should be handled by
// kubectl's native printers (raw resource output) rather than cwide's
// template-column pipeline.
func isNativeOutput(o string) bool {
	if o == "" {
		return false
	}
	if nativeOutputFormats[o] {
		return true
	}
	// jsonpath=..., go-template=... etc
	for prefix := range nativeOutputFormats {
		if strings.HasPrefix(o, prefix+"=") {
			return true
		}
	}
	return false
}

// emitNative bypasses the template pipeline and prints each Info's raw
// object using kubectl's PrintFlags for the given output format.
func (o *GetOptions) emitNative(infos []*resource.Info) error {
	format := o.Output
	pf := genericclioptions.NewPrintFlags("").WithTypeSetter(nil).WithDefaultOutput(format)
	pf.OutputFormat = &format

	printer, err := pf.ToPrinter()
	if err != nil {
		return fmt.Errorf("configure %q output: %w", format, err)
	}

	// Print each object. For yaml/json we insert a document separator
	// between objects so multiple results form a valid multi-doc stream.
	out := o.Out
	if out == nil {
		out = os.Stdout
	}

	needsSeparator := len(infos) > 1 && (format == "yaml" || format == "json")
	for i, info := range infos {
		obj := info.Object
		if _, ok := obj.(runtime.Unstructured); !ok {
			// Objects from the builder are already unstructured; guard defensively.
		}
		if needsSeparator && format == "yaml" && i > 0 {
			fmt.Fprintln(out, "---")
		}
		if err := printer.PrintObj(obj, out); err != nil {
			return fmt.Errorf("print object: %w", err)
		}
	}
	return nil
}

// silence unused-import complaint on printers when the file grows
var _ = printers.NewTypeSetter
