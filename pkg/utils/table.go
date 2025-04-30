package utils

import (
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	printers "k8s.io/kubernetes/pkg/printers"
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

	typed := reflect.New(handler.printFunc.Type().In(0).Elem())
	convertResults := reflect.ValueOf(runtime.DefaultUnstructuredConverter.FromUnstructured).Call([]reflect.Value{reflect.ValueOf(unstructuredObj.Object), typed})
	if !convertResults[0].IsNil() {
		return nil, convertResults[0].Interface().(error)
	}

	fmt.Printf("obj: %+v\n\n", unstructuredObj.Object)

	fmt.Printf("Converting object to type %s, obj: %+v\n\n\n", handler.printFunc.Type().In(0).String(), typed.Interface())

	args := []reflect.Value{typed, reflect.ValueOf(options)}
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
