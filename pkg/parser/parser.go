package parser

import (
	"fmt"
	"reflect"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/kubectl-cwide/pkg/parser/funcs"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/jsonpath"
)

type Parser interface {
	Parse(obj runtime.Object) (string, error)
}

type FieldParser struct {
	*jsonpath.JSONPath
	*template.Template
	IsAGE  bool
	Config *rest.Config
}

func NewFieldParser() *FieldParser {
	return &FieldParser{}
}

func (p *FieldParser) Parse(obj runtime.Object) (string, error) {
	var result string
	if p.JSONPath != nil {
		jpParser := p.JSONPath
		var values [][]reflect.Value
		var err error
		if unstructured, ok := obj.(runtime.Unstructured); ok {
			values, err = jpParser.FindResults(unstructured.UnstructuredContent())
		} else {
			values, err = jpParser.FindResults(reflect.ValueOf(obj).Elem().Interface())
		}

		if err != nil {
			return "", err
		}
		valueStrings := []string{}
		if len(values) == 0 || len(values[0]) == 0 {
			valueStrings = append(valueStrings, "<none>")
		}

		for arrIx := range values {
			for valIx := range values[arrIx] {
				valueStrings = append(valueStrings, printers.EscapeTerminal(fmt.Sprint(values[arrIx][valIx].Interface())))
			}
		}

		result = strings.Join(valueStrings, ",")
	}

	if p.Template != nil {
		var buf strings.Builder
		tParser := p.Template

		unstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return "", err
		}
		if err := tParser.Execute(&buf, unstructured); err != nil {
			return "", err
		}

		result = buf.String()
	}

	if p.IsAGE {
		return age(result), nil
	}

	return result, nil
}

func GetFuncMap(cfg *rest.Config) template.FuncMap {
	m := funcs.DefaultMap

	for k, v := range sprig.TxtFuncMap() {
		m[k] = v
	}

	m["lookup"] = funcs.NewLookupFunction(cfg)

	return m
}

func IsTemplate(template string) bool {
	return strings.HasPrefix(template, "{{") && strings.HasSuffix(template, "}}")
}

// age calculates the age of a resource based on its creation time in RFC3393.
func age(creationTime string) string {
	t, err := time.Parse(time.RFC3339, creationTime)
	if err != nil {
		return ""
	}

	return duration.HumanDuration(time.Since(t))
}
