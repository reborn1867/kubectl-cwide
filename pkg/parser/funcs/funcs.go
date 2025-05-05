package funcs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/gosuri/uiprogress"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
	goYaml "sigs.k8s.io/yaml/goyaml.v3"
)

const (
	recursionMaxNums = 1000
)

// copy from helm/pkg/engine/funcs.go, modified to remove helm specific functions
var DefaultMap = template.FuncMap{
	"toToml":        toTOML,
	"fromToml":      fromTOML,
	"toYaml":        toYAML,
	"toYamlPretty":  toYAMLPretty,
	"fromYaml":      fromYAML,
	"fromYamlArray": fromYAMLArray,
	"toJson":        toJSON,
	"fromJson":      fromJSON,
	"fromJsonArray": fromJSONArray,
	"include":       includeFun(nil, map[string]int{}),
	"tpl":           tplFun(nil, map[string]int{}, false),
	"progressBar":   progressBar,
}

// toYAML takes an interface, marshals it to yaml, and returns a string. It will
// always return a string, even on marshal error (empty string).
//
// This is designed to be called from a template.
func toYAML(v interface{}) string {
	data, err := yaml.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}
	return strings.TrimSuffix(string(data), "\n")
}

func toYAMLPretty(v interface{}) string {
	var data bytes.Buffer
	encoder := goYaml.NewEncoder(&data)
	encoder.SetIndent(2)
	err := encoder.Encode(v)

	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}
	return strings.TrimSuffix(data.String(), "\n")
}

// fromYAML converts a YAML document into a map[string]interface{}.
//
// This is not a general-purpose YAML parser, and will not parse all valid
// YAML documents. Additionally, because its intended use is within templates
// it tolerates errors. It will insert the returned error message string into
// m["Error"] in the returned map.
func fromYAML(str string) map[string]interface{} {
	m := map[string]interface{}{}

	if err := yaml.Unmarshal([]byte(str), &m); err != nil {
		m["Error"] = err.Error()
	}
	return m
}

// fromYAMLArray converts a YAML array into a []interface{}.
//
// This is not a general-purpose YAML parser, and will not parse all valid
// YAML documents. Additionally, because its intended use is within templates
// it tolerates errors. It will insert the returned error message string as
// the first and only item in the returned array.
func fromYAMLArray(str string) []interface{} {
	a := []interface{}{}

	if err := yaml.Unmarshal([]byte(str), &a); err != nil {
		a = []interface{}{err.Error()}
	}
	return a
}

// toTOML takes an interface, marshals it to toml, and returns a string. It will
// always return a string, even on marshal error (empty string).
//
// This is designed to be called from a template.
func toTOML(v interface{}) string {
	b := bytes.NewBuffer(nil)
	e := toml.NewEncoder(b)
	err := e.Encode(v)
	if err != nil {
		return err.Error()
	}
	return b.String()
}

// fromTOML converts a TOML document into a map[string]interface{}.
//
// This is not a general-purpose TOML parser, and will not parse all valid
// TOML documents. Additionally, because its intended use is within templates
// it tolerates errors. It will insert the returned error message string into
// m["Error"] in the returned map.
func fromTOML(str string) map[string]interface{} {
	m := make(map[string]interface{})

	if err := toml.Unmarshal([]byte(str), &m); err != nil {
		m["Error"] = err.Error()
	}
	return m
}

// toJSON takes an interface, marshals it to json, and returns a string. It will
// always return a string, even on marshal error (empty string).
//
// This is designed to be called from a template.
func toJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		// Swallow errors inside of a template.
		return ""
	}
	return string(data)
}

// fromJSON converts a JSON document into a map[string]interface{}.
//
// This is not a general-purpose JSON parser, and will not parse all valid
// JSON documents. Additionally, because its intended use is within templates
// it tolerates errors. It will insert the returned error message string into
// m["Error"] in the returned map.
func fromJSON(str string) map[string]interface{} {
	m := make(map[string]interface{})

	if err := json.Unmarshal([]byte(str), &m); err != nil {
		m["Error"] = err.Error()
	}
	return m
}

// fromJSONArray converts a JSON array into a []interface{}.
//
// This is not a general-purpose JSON parser, and will not parse all valid
// JSON documents. Additionally, because its intended use is within templates
// it tolerates errors. It will insert the returned error message string as
// the first and only item in the returned array.
func fromJSONArray(str string) []interface{} {
	a := []interface{}{}

	if err := json.Unmarshal([]byte(str), &a); err != nil {
		a = []interface{}{err.Error()}
	}
	return a
}

// 'include' needs to be defined in the scope of a 'tpl' template as
// well as regular file-loaded templates.
func includeFun(t *template.Template, includedNames map[string]int) func(string, interface{}) (string, error) {
	return func(name string, data interface{}) (string, error) {
		var buf strings.Builder
		if v, ok := includedNames[name]; ok {
			if v > recursionMaxNums {
				return "", errors.Wrapf(fmt.Errorf("unable to execute template"), "rendering template has a nested reference name: %s", name)
			}
			includedNames[name]++
		} else {
			includedNames[name] = 1
		}
		err := t.ExecuteTemplate(&buf, name, data)
		includedNames[name]--
		return buf.String(), err
	}
}

// As does 'tpl', so that nested calls to 'tpl' see the templates
// defined by their enclosing contexts.
func tplFun(parent *template.Template, includedNames map[string]int, strict bool) func(string, interface{}) (string, error) {
	return func(tpl string, vals interface{}) (string, error) {
		t, err := parent.Clone()
		if err != nil {
			return "", errors.Wrapf(err, "cannot clone template")
		}

		// Re-inject the missingkey option, see text/template issue https://github.com/golang/go/issues/43022
		// We have to go by strict from our engine configuration, as the option fields are private in Template.
		// TODO: Remove workaround (and the strict parameter) once we build only with golang versions with a fix.
		if strict {
			t.Option("missingkey=error")
		} else {
			t.Option("missingkey=zero")
		}

		// Re-inject 'include' so that it can close over our clone of t;
		// this lets any 'define's inside tpl be 'include'd.
		t.Funcs(template.FuncMap{
			"include": includeFun(t, includedNames),
			"tpl":     tplFun(t, includedNames, strict),
		})

		// We need a .New template, as template text which is just blanks
		// or comments after parsing out defines just adds new named
		// template definitions without changing the main template.
		// https://pkg.go.dev/text/template#Template.Parse
		// Use the parent's name for lack of a better way to identify the tpl
		// text string. (Maybe we could use a hash appended to the name?)
		t, err = t.New(parent.Name()).Parse(tpl)
		if err != nil {
			return "", errors.Wrapf(err, "cannot parse template %q", tpl)
		}

		var buf strings.Builder
		if err := t.Execute(&buf, vals); err != nil {
			return "", errors.Wrapf(err, "error during tpl function execution for %q", tpl)
		}

		// See comment in renderWithReferences explaining the <no value> hack.
		return strings.ReplaceAll(buf.String(), "<no value>", ""), nil
	}
}

func progressBar(currentRaw, totalRaw interface{}) string {
	current, ok := currentRaw.(int64)
	if !ok {
		return ""
	}

	total, ok := totalRaw.(int64)
	if !ok {
		return ""
	}

	bar := uiprogress.AddBar(int(total))
	bar.Set(int(current))
	bar.Width = 20
	return bar.String()

}
