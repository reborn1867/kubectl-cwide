package utils

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	printers "k8s.io/kubernetes/pkg/printers"

	// Register internal API types and their versioned<->internal conversions
	// on legacyscheme.Scheme. These are required so that the codec below can
	// decode an unstructured object into the internal API type expected by
	// the kubectl print handlers.
	//
	// Missing an install here manifests as GenerateTable returning
	// 'no kind "X" is registered for version "Y" in scheme ...',
	// which the customcolumn printer silently swallows — the header prints
	// but every data row's $_defaultPrinterField columns come out empty.
	// So this list must cover every API group whose printer is registered
	// upstream in k8s.io/kubernetes/pkg/printers/internalversion.
	_ "k8s.io/kubernetes/pkg/apis/admissionregistration/install"
	_ "k8s.io/kubernetes/pkg/apis/apiserverinternal/install"
	_ "k8s.io/kubernetes/pkg/apis/apps/install"
	_ "k8s.io/kubernetes/pkg/apis/authentication/install"
	_ "k8s.io/kubernetes/pkg/apis/authorization/install"
	_ "k8s.io/kubernetes/pkg/apis/autoscaling/install"
	_ "k8s.io/kubernetes/pkg/apis/batch/install"
	_ "k8s.io/kubernetes/pkg/apis/certificates/install"
	_ "k8s.io/kubernetes/pkg/apis/coordination/install"
	_ "k8s.io/kubernetes/pkg/apis/core/install"
	_ "k8s.io/kubernetes/pkg/apis/discovery/install"
	_ "k8s.io/kubernetes/pkg/apis/events/install"
	_ "k8s.io/kubernetes/pkg/apis/extensions/install"
	_ "k8s.io/kubernetes/pkg/apis/flowcontrol/install"
	_ "k8s.io/kubernetes/pkg/apis/networking/install"
	_ "k8s.io/kubernetes/pkg/apis/node/install"
	_ "k8s.io/kubernetes/pkg/apis/policy/install"
	_ "k8s.io/kubernetes/pkg/apis/rbac/install"
	_ "k8s.io/kubernetes/pkg/apis/resource/install"
	_ "k8s.io/kubernetes/pkg/apis/scheduling/install"
	_ "k8s.io/kubernetes/pkg/apis/storage/install"
	_ "k8s.io/kubernetes/pkg/apis/storagemigration/install"
)

type handlerEntry struct {
	columnDefinitions []metav1.TableColumnDefinition
	printFunc         reflect.Value
	gvk               schema.GroupVersionKind
}

var _ printers.TableGenerator = &DefaultTableGenerator{}
var _ printers.PrintHandler = &DefaultTableGenerator{}

type DefaultTableGenerator struct {
	handlerMap       map[reflect.Type]*handlerEntry
	handlerMapByKind map[string]*handlerEntry
}

// NewTableGenerator creates a HumanReadableGenerator suitable for calling GenerateTable().
func NewTableGenerator() *DefaultTableGenerator {
	return &DefaultTableGenerator{
		handlerMap:       make(map[reflect.Type]*handlerEntry),
		handlerMapByKind: make(map[string]*handlerEntry),
	}
}

func (h *DefaultTableGenerator) ResourceColumnDefinition(kind string) []metav1.TableColumnDefinition {
	handler, ok := h.handlerMapByKind[kind]
	if !ok {
		return nil
	}

	return handler.columnDefinitions
}

// With method - accepts a list of builder functions that modify HumanReadableGenerator
func (h *DefaultTableGenerator) With(fns ...func(printers.PrintHandler)) *DefaultTableGenerator {
	for _, fn := range fns {
		fn(h)
	}

	return h
}

// GenerateTable returns a table for the provided object, using the printer registered for that type. It returns
// a table that includes all of the information requested by options, but will not remove rows or columns. The
// caller is responsible for applying rules related to filtering rows or columns.
func (h *DefaultTableGenerator) GenerateTable(obj runtime.Object, options printers.GenerateOptions) (*metav1.Table, error) {
	var kind string
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		// If the object is unstructured, we need to convert it to a structured object
		// before passing it to the print function.
		// This is a workaround for the fact that the print function expects a specific type.
		return nil, fmt.Errorf("object is not of type unstructured.Unstructured")
	}

	kind = strings.ToLower(unstructuredObj.GetKind())

	handler, ok := h.handlerMapByKind[kind]
	if !ok {
		return nil, fmt.Errorf("no table handler registered for this kind %s", kind)
	}

	// Decode the unstructured object into its internal API type via the
	// legacy scheme's codec.
	//
	// We deliberately avoid runtime.DefaultUnstructuredConverter.FromUnstructured
	// here. That converter maps struct fields by JSON tag, but k8s internal API
	// types (e.g. k8s.io/kubernetes/pkg/apis/core.Pod) carry no JSON tags, so it
	// falls back to lowercasing only the first letter of the Go field name. That
	// corrupts acronym fields such as PodIP.IP ("IP" -> "iP" instead of "ip"),
	// leaving them empty. As a result the built-in printers rendered affected
	// columns as <none> (most visibly a pod's IP). Decoding through the scheme
	// routes JSON -> versioned type (which has correct JSON tags) -> internal
	// type via the generated conversions, preserving every field.
	data, err := json.Marshal(unstructuredObj.UnstructuredContent())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal object: %w", err)
	}

	internalObj, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(), data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode object into internal type: %w", err)
	}

	if expected := handler.printFunc.Type().In(0); reflect.TypeOf(internalObj) != expected {
		return nil, fmt.Errorf("decoded object type %T does not match printer input type %s", internalObj, expected)
	}

	args := []reflect.Value{reflect.ValueOf(internalObj), reflect.ValueOf(options)}
	results := handler.printFunc.Call(args)
	if !results[1].IsNil() {
		return nil, results[1].Interface().(error)
	}

	var columns []metav1.TableColumnDefinition
	if !options.NoHeaders {
		columns = handler.columnDefinitions
		if !options.Wide {
			columns = make([]metav1.TableColumnDefinition, 0, len(handler.columnDefinitions))
			for i := range handler.columnDefinitions {
				if handler.columnDefinitions[i].Priority != 0 {
					continue
				}
				columns = append(columns, handler.columnDefinitions[i])
			}
		}
	}
	table := &metav1.Table{
		ListMeta: metav1.ListMeta{
			ResourceVersion: "",
		},
		ColumnDefinitions: columns,
		Rows:              results[0].Interface().([]metav1.TableRow),
	}
	if m, err := meta.ListAccessor(obj); err == nil {
		table.ResourceVersion = m.GetResourceVersion()
		table.Continue = m.GetContinue()
		table.RemainingItemCount = m.GetRemainingItemCount()
	} else {
		if m, err := meta.CommonAccessor(obj); err == nil {
			table.ResourceVersion = m.GetResourceVersion()
		}
	}
	return table, nil
}

// TableHandler adds a print handler with a given set of columns to HumanReadableGenerator instance.
// See ValidateRowPrintHandlerFunc for required method signature.
func (h *DefaultTableGenerator) TableHandler(columnDefinitions []metav1.TableColumnDefinition, printFunc interface{}) error {
	printFuncValue := reflect.ValueOf(printFunc)
	if err := ValidateRowPrintHandlerFunc(printFuncValue); err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to register print function: %v", err))
		return err
	}

	entry := &handlerEntry{
		columnDefinitions: columnDefinitions,
		printFunc:         printFuncValue,
	}

	objType := printFuncValue.Type().In(0)
	if _, ok := h.handlerMap[objType]; ok {
		err := fmt.Errorf("registered duplicate printer for %v", objType)
		utilruntime.HandleError(err)
		return err
	}

	// This is a hack to get the kind of the object from the type name, considering it's only for default k8s objects, kind should be sufficient as unique identifier
	kind := strings.ToLower(strings.Split(objType.String(), ".")[1])
	h.handlerMapByKind[kind] = entry

	h.handlerMap[objType] = entry
	return nil
}

// ValidateRowPrintHandlerFunc validates print handler signature.
// printFunc is the function that will be called to print an object.
// It must be of the following type:
//
//	func printFunc(object ObjectType, options GenerateOptions) ([]metav1.TableRow, error)
//
// where ObjectType is the type of the object that will be printed, and the first
// return value is an array of rows, with each row containing a number of cells that
// match the number of columns defined for that printer function.
func ValidateRowPrintHandlerFunc(printFunc reflect.Value) error {
	if printFunc.Kind() != reflect.Func {
		return fmt.Errorf("invalid print handler. %#v is not a function", printFunc)
	}
	funcType := printFunc.Type()
	if funcType.NumIn() != 2 || funcType.NumOut() != 2 {
		return fmt.Errorf("invalid print handler." +
			"Must accept 2 parameters and return 2 value")
	}
	if funcType.In(1) != reflect.TypeOf((*printers.GenerateOptions)(nil)).Elem() ||
		funcType.Out(0) != reflect.TypeOf((*[]metav1.TableRow)(nil)).Elem() ||
		funcType.Out(1) != reflect.TypeOf((*error)(nil)).Elem() {
		return fmt.Errorf("invalid print handler. The expected signature is: "+
			"func handler(obj %v, options GenerateOptions) ([]metav1.TableRow, error)", funcType.In(0))
	}
	return nil
}
