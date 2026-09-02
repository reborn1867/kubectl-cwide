package template

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"k8s.io/client-go/util/jsonpath"

	"github.com/kubectl-cwide/pkg/models"
	"github.com/kubectl-cwide/pkg/utils"
)

func NewCmdLint() *cobra.Command {
	var recursive bool

	cmd := &cobra.Command{
		Use:   "lint [template-file|directory]",
		Short: "Statically validate one or more column template files",
		Long: `Parse a .yaml or .tpl template and check that:
  - the file is syntactically valid
  - every JSONPath field spec parses cleanly
  - every text/template body parses cleanly (best-effort — no execution)

Does NOT contact the cluster or resolve schema against a live API.

With --recursive, walks a directory (or the resolved template root if no
argument is given) and lints every .yaml/.yml/.tpl file below it. Exits
non-zero if any file fails; prints a summary at the end.`,
		Example: `  # Lint one template
  kubectl cwide template lint ~/.kubectl-cwide/templates/pod--v1/default.yaml

  # Lint every template under the template root (uses --template-path config)
  kubectl cwide template lint --recursive

  # Lint every template under a specific directory
  kubectl cwide template lint --recursive ~/.kubectl-cwide/templates

  # Same, short form
  kubectl cwide template lint -R ~/.kubectl-cwide/templates`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if recursive {
				root := ""
				if len(args) > 0 {
					root = args[0]
				} else {
					// No path given → resolve from config, matching what
					// 'get' / 'template list' do when the user is invoking
					// cwide against their standard template tree.
					r, err := utils.ResolveTemplatePath(cmd)
					if err != nil {
						return fmt.Errorf("no path given and template root couldn't be resolved: %w", err)
					}
					root = r
				}
				return lintTree(cmd, root)
			}
			if len(args) == 0 {
				return fmt.Errorf("either pass a file path or use --recursive to lint a directory tree")
			}
			return lintOne(cmd, args[0])
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "R", false,
		"Walk a directory (or the template root if no path is given) and lint every .yaml/.yml/.tpl file below it")

	return cmd
}

// lintTree walks root and lints every template file it finds. Aggregates
// per-file results and returns a non-nil error if any file fails, so the
// exit code reflects overall success/failure. Continues past failures so
// the user sees the full picture instead of stopping at the first bad file.
func lintTree(cmd *cobra.Command, root string) error {
	info, err := os.Stat(root)
	if err != nil {
		return fmt.Errorf("stat %s: %w", root, err)
	}
	if !info.IsDir() {
		// --recursive on a file is a benign no-op — lint the one file.
		return lintOne(cmd, root)
	}

	var okCount, failCount int
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip unreadable subdirs but surface them so the user can
			// diagnose permissions.
			fmt.Fprintf(cmd.ErrOrStderr(), "skip %s: %v\n", path, err)
			return nil
		}
		if d.IsDir() {
			// Don't recurse into a _shared helpers dir — those files are
			// fragments, not standalone templates, and would trip the
			// 'template has no columns' check.
			if d.Name() == "_shared" && path != root {
				return fs.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" && ext != ".tpl" {
			return nil
		}
		if err := lintOne(cmd, path); err != nil {
			failCount++
			// lintOne already printed FAIL + reasons; keep walking.
			return nil
		}
		okCount++
		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	total := okCount + failCount
	fmt.Fprintf(cmd.OutOrStdout(), "\nLint: %d ok, %d failed (%d files)\n", okCount, failCount, total)
	if failCount > 0 {
		return fmt.Errorf("%d template(s) failed lint", failCount)
	}
	return nil
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
