package template

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/util/jsonpath"

	"github.com/kubectl-cwide/pkg/models"
)

func NewCmdLint() *cobra.Command {
	return &cobra.Command{
		Use:   "lint <template-file>",
		Short: "Statically validate a column template file",
		Long: `Parse a .yaml or .tpl template and check that:
  - the file is syntactically valid
  - every JSONPath field spec parses cleanly
  - every text/template body parses cleanly (best-effort — no execution)

Does NOT contact the cluster or resolve schema against a live API.`,
		Example: `  # Lint one template
  kubectl cwide template lint ~/.kubectl-cwide/templates/pod--v1/default.yaml

  # Lint every template under a directory
  find ~/.kubectl-cwide/templates -name '*.yaml' -exec kubectl cwide template lint {} \;`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return lintOne(cmd, args[0])
		},
	}
}

func lintOne(cmd *cobra.Command, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	var problems []string
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		var tmpl models.YAMLTemplate
		if err := yaml.Unmarshal(data, &tmpl); err != nil {
			return fmt.Errorf("yaml parse: %w", err)
		}
		if len(tmpl.Columns) == 0 {
			problems = append(problems, "template has no columns")
		}
		for i, c := range tmpl.Columns {
			if c.Header == "" {
				problems = append(problems, fmt.Sprintf("column[%d]: missing header", i))
			}
			if c.FieldSpec == "" && c.Template == "" {
				problems = append(problems, fmt.Sprintf("column[%d] (%s): needs fieldSpec or template", i, c.Header))
			}
			if c.FieldSpec != "" {
				if err := parseJSONPath(c.FieldSpec); err != nil {
					problems = append(problems, fmt.Sprintf("column[%d] (%s): bad fieldSpec: %v", i, c.Header, err))
				}
			}
		}
	case ".tpl":
		lines := strings.Split(string(data), "\n")
		if len(lines) < 2 {
			problems = append(problems, "tpl needs at least a header line and a spec line")
		}
	default:
		return fmt.Errorf("unsupported extension %q (want .yaml, .yml, or .tpl)", ext)
	}

	if len(problems) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "OK  %s\n", path)
		return nil
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "FAIL %s\n", path)
	for _, p := range problems {
		fmt.Fprintf(cmd.ErrOrStderr(), "  - %s\n", p)
	}
	return fmt.Errorf("%d issue(s)", len(problems))
}

func parseJSONPath(expr string) error {
	// Accept either bare `.foo.bar` or `{.foo.bar}` form; normalize to braces.
	e := expr
	if !strings.HasPrefix(e, "{") {
		e = "{" + e + "}"
	}
	jp := jsonpath.New("lint")
	return jp.Parse(e)
}
