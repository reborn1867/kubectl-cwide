// Package explain implements `kubectl cwide explain`, a read-only
// introspection command that describes a template (its provenance, columns,
// fields read, and functions used) or an alias (its target, source format,
// and template bindings). It never contacts the cluster.
package explain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/kubectl-cwide/pkg/cmd/completions"
	"github.com/kubectl-cwide/pkg/common"
	"github.com/kubectl-cwide/pkg/models"
	"github.com/kubectl-cwide/pkg/utils"
)

// liveCallFuncs are template functions that hit the API server per row; explain
// flags them so users understand the cost of a template before running it.
var liveCallFuncs = map[string]string{
	"probeCheck": "pings pod probe endpoints via the API server proxy (one request per row)",
	"lookup":     "fetches related objects from the API server (one or more requests per row)",
}

// knownFuncs is the set of built-in template function names explain recognizes
// when scanning template/helper bodies. Kept in sync with pkg/parser/funcs.
var knownFuncs = []string{
	"probeCheck", "lookup", "age", "humanBytes", "colorIf", "truncate", "b64dec", "safeIndex",
}

func NewCmdExplain() *cobra.Command {
	var resource string
	var output string

	cmd := &cobra.Command{
		Use:        "explain NAME",
		Aliases:    []string{"exp"},
		SuggestFor: []string{"describe", "info"},
		Short:      "Explain a template or alias: where it came from and what it does",
		Long: `Describe a configured alias or an installed template without opening the
file or config by hand. Read-only; never contacts the cluster.

If NAME matches a configured alias it is explained as an alias; otherwise it is
treated as a template name (use -r/--resource to locate the template dir). If
both exist, the alias wins and a hint notes the same-named template.

For a template, explain reports:
  - provenance (source/version/repo/ref) from the metadata block, if any
  - each column and whether it is a JSONPath, a Go template, or a default-printer field
  - the top-level fields the JSONPath columns read
  - which built-in functions the template uses, flagging any that make live API calls

For an alias, explain reports its target kind(s), whether it uses the rich or
legacy config format, and any bound templates.`,
		Example: `  # Explain an alias
  kubectl cwide explain core

  # Explain the 'debug' pod template
  kubectl cwide explain debug -r pod

  # Structured output
  kubectl cwide explain core -o yaml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, cfgErr := utils.LoadConfig()

			// Alias takes precedence when the name is a configured alias.
			if cfgErr == nil && isAlias(cfg, name) {
				res := explainAlias(cfg, name)
				// Note a same-named template if one is resolvable.
				if resource != "" {
					if _, _, err := locateTemplate(cmd, resource, name); err == nil {
						res.Note = fmt.Sprintf("a template named %q also exists for %q; pass a non-alias name to explain it", name, resource)
					}
				}
				return emit(cmd, output, res)
			}

			// Otherwise treat as a template.
			if resource == "" {
				return fmt.Errorf("%q is not a configured alias; to explain a template pass -r/--resource to locate it", name)
			}
			path, data, err := locateTemplate(cmd, resource, name)
			if err != nil {
				return err
			}
			res, err := explainTemplate(path, data)
			if err != nil {
				return err
			}
			return emit(cmd, output, res)
		},
	}

	cmd.Flags().StringVarP(&resource, "resource", "r", "", "Resource type to locate the template under (e.g. pod, deployment)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output format: empty (human) or one of yaml, json")

	_ = cmd.RegisterFlagCompletionFunc("resource", completions.ResourceTypes)
	_ = cmd.RegisterFlagCompletionFunc("output", cobra.FixedCompletions(
		[]string{"yaml", "json"}, cobra.ShellCompDirectiveNoFileComp))
	// The positional arg may be an alias or a template name; alias names are the
	// cheap, cluster-free completion source (template names need -r context).
	cmd.ValidArgsFunction = func(c *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) >= 1 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completions.AliasNames(c, args, toComplete)
	}

	return cmd
}

func isAlias(cfg *models.Config, name string) bool {
	if cfg == nil {
		return false
	}
	if _, ok := cfg.AliasEntries[name]; ok {
		return true
	}
	_, ok := cfg.Aliases[name]
	return ok
}

// locateTemplate finds the .yaml (preferred) or .tpl file for (resource, name)
// under the resolved template root. Mirrors get.go's resolution order.
func locateTemplate(cmd *cobra.Command, resource, name string) (string, []byte, error) {
	root, err := utils.ResolveTemplatePath(cmd)
	if err != nil {
		return "", nil, fmt.Errorf("resolve template path: %w", err)
	}
	pattern := filepath.Join(root, fmt.Sprintf("%s-*", resource))
	dirs, err := filepath.Glob(pattern)
	if err != nil {
		return "", nil, fmt.Errorf("glob %s: %w", pattern, err)
	}
	if len(dirs) == 0 {
		return "", nil, fmt.Errorf("no template directory for %q under %s (run 'init' first)", resource, root)
	}
	if len(dirs) > 1 {
		return "", nil, fmt.Errorf("found multiple directories for %q: %v; use a more specific resource", resource, dirs)
	}
	yamlPath := filepath.Join(dirs[0], name+".yaml")
	if data, err := os.ReadFile(yamlPath); err == nil {
		return yamlPath, data, nil
	}
	tplPath := filepath.Join(dirs[0], name+".tpl")
	if data, err := os.ReadFile(tplPath); err == nil {
		return tplPath, data, nil
	}
	return "", nil, fmt.Errorf("template %q not found (tried %s and %s)", name, yamlPath, tplPath)
}

// ---- result model ----

type columnInfo struct {
	Header string `yaml:"header" json:"header"`
	Kind   string `yaml:"kind" json:"kind"` // jsonpath | template | default-printer
	Spec   string `yaml:"spec,omitempty" json:"spec,omitempty"`
}

type funcInfo struct {
	Name     string `yaml:"name" json:"name"`
	LiveCall string `yaml:"liveCall,omitempty" json:"liveCall,omitempty"`
}

type explainResult struct {
	Kind        string                   `yaml:"kind" json:"kind"` // "template" | "alias"
	Name        string                   `yaml:"name" json:"name"`
	Path        string                   `yaml:"path,omitempty" json:"path,omitempty"`
	Format      string                   `yaml:"format,omitempty" json:"format,omitempty"` // yaml | tpl
	Provenance  *models.TemplateMetadata `yaml:"provenance,omitempty" json:"provenance,omitempty"`
	Columns     []columnInfo             `yaml:"columns,omitempty" json:"columns,omitempty"`
	FieldsRead  []string                 `yaml:"fieldsRead,omitempty" json:"fieldsRead,omitempty"`
	Functions   []funcInfo               `yaml:"functions,omitempty" json:"functions,omitempty"`
	FieldsExact bool                     `yaml:"fieldsExact" json:"fieldsExact"`

	// alias fields
	Target      string            `yaml:"target,omitempty" json:"target,omitempty"`
	Kinds       []string          `yaml:"kinds,omitempty" json:"kinds,omitempty"`
	SourceFmt   string            `yaml:"sourceFormat,omitempty" json:"sourceFormat,omitempty"`
	Template    string            `yaml:"template,omitempty" json:"template,omitempty"`
	PerKindTmpl map[string]string `yaml:"perKindTemplates,omitempty" json:"perKindTemplates,omitempty"`

	Note string `yaml:"note,omitempty" json:"note,omitempty"`
}

// ---- template explanation ----

func explainTemplate(path string, data []byte) (*explainResult, error) {
	res := &explainResult{Kind: "template", Name: baseName(path), Path: path, FieldsExact: true}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".tpl" {
		res.Format = "tpl"
		// .tpl has no structured metadata; report helper defines + a note.
		res.Functions = scanFuncs(string(data))
		res.Note = "classic .tpl template: no structured metadata; column/field detail is limited"
		res.FieldsExact = false
		return res, nil
	}

	res.Format = "yaml"
	var tmpl models.YAMLTemplate
	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("yaml parse %s: %w", path, err)
	}
	res.Provenance = tmpl.Metadata

	fieldSet := map[string]struct{}{}
	for _, c := range tmpl.Columns {
		ci := columnInfo{Header: c.Header}
		switch {
		case c.FieldSpec == common.DefaultPrinterField:
			ci.Kind = "default-printer"
		case c.FieldSpec != "":
			ci.Kind = "jsonpath"
			ci.Spec = c.FieldSpec
			if root := jsonPathRoot(c.FieldSpec); root != "" {
				fieldSet[root] = struct{}{}
			}
		case c.Template != "":
			ci.Kind = "template"
			ci.Spec = c.Template
			// Go-template field access can't be fully enumerated statically.
			res.FieldsExact = false
		default:
			ci.Kind = "empty"
		}
		res.Columns = append(res.Columns, ci)
	}

	res.FieldsRead = sortedSetKeys(fieldSet)
	// Scan template columns + helpers/funcs blocks for known functions.
	var b strings.Builder
	for _, c := range tmpl.Columns {
		b.WriteString(c.Template)
		b.WriteString("\n")
	}
	b.WriteString(tmpl.Helpers)
	for _, v := range tmpl.Funcs {
		b.WriteString("\n")
		b.WriteString(v)
	}
	res.Functions = scanFuncs(b.String())
	return res, nil
}

// jsonPathRoot returns the first path segment of a JSONPath fieldSpec, e.g.
// ".status.phase" -> "status", "{.metadata.name}" -> "metadata". Returns "" if
// it can't determine a plain root (filters, wildcards at the head, etc.).
func jsonPathRoot(spec string) string {
	s := strings.TrimSpace(spec)
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	s = strings.TrimPrefix(s, ".")
	if s == "" {
		return ""
	}
	// Cut at the next path separator or bracket.
	for i, r := range s {
		if r == '.' || r == '[' {
			s = s[:i]
			break
		}
	}
	if s == "" || strings.ContainsAny(s, "*?()@") {
		return ""
	}
	return s
}

// scanFuncs reports which known functions appear in body, flagging live-call
// ones. Substring match is sufficient for a human-facing hint.
func scanFuncs(body string) []funcInfo {
	var out []funcInfo
	for _, name := range knownFuncs {
		if strings.Contains(body, name) {
			out = append(out, funcInfo{Name: name, LiveCall: liveCallFuncs[name]})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ---- alias explanation ----

func explainAlias(cfg *models.Config, name string) *explainResult {
	res := &explainResult{Kind: "alias", Name: name}
	if e, ok := cfg.AliasEntries[name]; ok {
		res.SourceFmt = "aliasEntries"
		res.Target = e.Resource
		res.Template = e.Template
		if len(e.Templates) > 0 {
			res.PerKindTmpl = e.Templates
		}
	} else {
		res.SourceFmt = "aliases (legacy)"
		res.Target = cfg.Aliases[name]
	}
	if strings.Contains(res.Target, ",") {
		for _, k := range strings.Split(res.Target, ",") {
			if k = strings.TrimSpace(k); k != "" {
				res.Kinds = append(res.Kinds, k)
			}
		}
		res.Note = "group alias: use the bare TYPE form; a group alias does not combine with a TYPE/NAME selector"
	}
	return res
}

// ---- output ----

func emit(cmd *cobra.Command, format string, res *explainResult) error {
	switch strings.ToLower(format) {
	case "", "text":
		return emitText(cmd, res)
	case "yaml":
		data, err := yaml.Marshal(res)
		if err != nil {
			return err
		}
		_, err = cmd.OutOrStdout().Write(data)
		return err
	case "json":
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	default:
		return fmt.Errorf("unsupported output %q (want yaml or json)", format)
	}
}

func emitText(cmd *cobra.Command, res *explainResult) error {
	w := cmd.OutOrStdout()
	if res.Kind == "alias" {
		fmt.Fprintf(w, "Alias: %s\n", res.Name)
		fmt.Fprintf(w, "  target:  %s\n", res.Target)
		if len(res.Kinds) > 0 {
			fmt.Fprintf(w, "  kinds:   %s\n", strings.Join(res.Kinds, ", "))
		}
		fmt.Fprintf(w, "  format:  %s\n", res.SourceFmt)
		if res.Template != "" {
			fmt.Fprintf(w, "  template (all kinds): %s\n", res.Template)
		}
		if len(res.PerKindTmpl) > 0 {
			fmt.Fprintf(w, "  per-kind templates:\n")
			for _, k := range sortedKeys(mapKeys(res.PerKindTmpl)) {
				fmt.Fprintf(w, "    %s -> %s\n", k, res.PerKindTmpl[k])
			}
		}
		if res.Note != "" {
			fmt.Fprintf(w, "  note:    %s\n", res.Note)
		}
		return nil
	}

	fmt.Fprintf(w, "Template: %s\n", res.Name)
	fmt.Fprintf(w, "  path:    %s (%s)\n", res.Path, res.Format)
	if res.Provenance != nil {
		p := res.Provenance
		fmt.Fprintf(w, "  source:  %s", orNone(p.Source))
		if p.Version != "" {
			fmt.Fprintf(w, "  version: %s", p.Version)
		}
		if p.SourceRepo != "" {
			fmt.Fprintf(w, "  repo: %s", p.SourceRepo)
		}
		if p.SourceRef != "" {
			fmt.Fprintf(w, "  ref: %s", p.SourceRef)
		}
		fmt.Fprintln(w)
		if p.InstalledAt != "" {
			fmt.Fprintf(w, "  installed: %s\n", p.InstalledAt)
		}
	} else if res.Format == "yaml" {
		fmt.Fprintf(w, "  source:  user-authored / legacy (no metadata block)\n")
	}
	if len(res.Columns) > 0 {
		fmt.Fprintf(w, "  columns:\n")
		for _, c := range res.Columns {
			if c.Spec != "" {
				fmt.Fprintf(w, "    %-20s %-15s %s\n", c.Header, c.Kind, c.Spec)
			} else {
				fmt.Fprintf(w, "    %-20s %s\n", c.Header, c.Kind)
			}
		}
	}
	if len(res.FieldsRead) > 0 {
		exact := ""
		if !res.FieldsExact {
			exact = " (approximate — has template columns)"
		}
		fmt.Fprintf(w, "  fields read%s: %s\n", exact, strings.Join(res.FieldsRead, ", "))
	}
	if len(res.Functions) > 0 {
		fmt.Fprintf(w, "  functions:\n")
		for _, f := range res.Functions {
			if f.LiveCall != "" {
				fmt.Fprintf(w, "    %-12s ⚠ %s\n", f.Name, f.LiveCall)
			} else {
				fmt.Fprintf(w, "    %s\n", f.Name)
			}
		}
	}
	if res.Note != "" {
		fmt.Fprintf(w, "  note:    %s\n", res.Note)
	}
	return nil
}

func orNone(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

func baseName(path string) string {
	b := filepath.Base(path)
	return strings.TrimSuffix(b, filepath.Ext(b))
}

func sortedKeys(m []string) []string {
	out := append([]string(nil), m...)
	sort.Strings(out)
	return out
}

func sortedSetKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func mapKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
