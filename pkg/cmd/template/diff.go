package template

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/kubectl-cwide/pkg/cmd/completions"
	"github.com/kubectl-cwide/pkg/parser/funcs"
	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
)

// isTerminal reports whether w is a character device (an interactive TTY).
// Used to gate ANSI coloring so redirected/piped diff output stays plain and
// byte-stable. Anything that isn't an *os.File (e.g. a test buffer) is treated
// as non-terminal.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// NewCmdDiff shows a unified diff between an installed template and another
// source — either a local file (--against-file) or a bundled cookbook recipe
// (--against-recipe pod/restart-reason). Purely local: never contacts the
// cluster and never phones home. Output is a stable unified diff without
// timestamps so the same input pair diffs to the same bytes every time.
func NewCmdDiff() *cobra.Command {
	var againstFile string
	var againstRecipe string

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare an installed template against another source",
		Long: `Print a unified diff between the installed template for a resource
and another template source, so users can see what a template scaffold or a
marketplace bundle would change before overwriting.

Sources:
  --against-file <path>     compare against a local .yaml/.tpl file
  --against-recipe <k/r>    compare against a bundled cookbook recipe
                            (kind/recipe, see 'template scaffold --list')

Diff is emitted in unified format with 3 lines of context. Timestamps are
omitted so the diff is byte-stable for identical inputs — pipe it into
tooling without worrying about spurious changes.

When writing to an interactive terminal the diff is colorized (added lines
green, removed red, hunk headers cyan). Coloring is automatically disabled when
the output is redirected or piped, and honors --no-color / NO_COLOR, so piped
output stays plain and byte-stable.`,
		Example: `  # Compare installed pod default template against a saved candidate
  kubectl cwide template diff -r pod -t default --against-file ./candidate.yaml

  # See what the 'restart-reason' cookbook recipe would change
  kubectl cwide template diff -r pod -t default --against-recipe pod/restart-reason`,
		RunE: func(cmd *cobra.Command, args []string) error {
			resourceType := cmd.Flag("resource").Value.String()
			templateName := cmd.Flag("template").Value.String()

			if resourceType == "" {
				return fmt.Errorf("--resource is required")
			}
			if againstFile == "" && againstRecipe == "" {
				return fmt.Errorf("one of --against-file or --against-recipe is required")
			}
			if againstFile != "" && againstRecipe != "" {
				return fmt.Errorf("--against-file and --against-recipe are mutually exclusive")
			}

			absPath, err := utils.ResolveTemplatePath(cmd)
			if err != nil {
				return fmt.Errorf("failed to resolve template path: %w", err)
			}

			leftPath, leftData, err := loadInstalledTemplate(absPath, resourceType, templateName)
			if err != nil {
				return err
			}

			var rightLabel string
			var rightData []byte
			if againstFile != "" {
				rightLabel = againstFile
				rightData, err = os.ReadFile(againstFile)
				if err != nil {
					return fmt.Errorf("read --against-file %s: %w", againstFile, err)
				}
			} else {
				rightLabel = "recipe:" + againstRecipe
				rightData, err = loadBundledRecipe(againstRecipe)
				if err != nil {
					return err
				}
			}

			// Colorize only for an interactive terminal so piped output stays
			// byte-stable for tooling. funcs.ColorEnabled() folds in
			// --no-color / NO_COLOR.
			color := funcs.ColorEnabled() && isTerminal(cmd.OutOrStdout())

			out := unifiedDiff(string(leftData), string(rightData), leftPath, rightLabel, 3, color)
			if out == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "no differences: %s == %s\n", leftPath, rightLabel)
				return nil
			}
			_, werr := cmd.OutOrStdout().Write([]byte(out))
			return werr
		},
	}

	cmd.Flags().StringP("resource", "r", "", "Resource type of the installed template (e.g. pod, deployment)")
	_ = cmd.RegisterFlagCompletionFunc("resource", completions.ResourceTypes)
	cmd.Flags().StringP("template", "t", "default", "Name of the installed template (without extension)")
	_ = cmd.RegisterFlagCompletionFunc("template", completions.TemplateNames)
	cmd.Flags().StringVar(&againstFile, "against-file", "", "Path to a local template file to diff against")
	cmd.Flags().StringVar(&againstRecipe, "against-recipe", "", "Bundled cookbook recipe in kind/recipe form to diff against")
	_ = cmd.RegisterFlagCompletionFunc("against-recipe", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var out []string
		kinds, _ := listKinds()
		for _, k := range kinds {
			for _, r := range recipesForKind(k) {
				out = append(out, k+"/"+r)
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	})
	_ = cmd.MarkFlagRequired("resource")

	return cmd
}

// loadInstalledTemplate locates the .yaml/.tpl for the given resource and
// template name under the resolved template root. Prefers .yaml (matching
// get.go's resolution order). Returns the path and raw bytes.
func loadInstalledTemplate(rootPath, resourceType, templateName string) (string, []byte, error) {
	pattern := filepath.Join(rootPath, fmt.Sprintf("%s-*", resourceType))
	dirs, err := filepath.Glob(pattern)
	if err != nil {
		return "", nil, fmt.Errorf("glob %s: %w", pattern, err)
	}
	if len(dirs) == 0 {
		return "", nil, fmt.Errorf("no resource directory found for %q under %s; run 'init' first", resourceType, rootPath)
	}
	if len(dirs) > 1 {
		return "", nil, fmt.Errorf("found multiple directories for %q: %v; specify a more precise resource type", resourceType, dirs)
	}

	yamlPath := filepath.Join(dirs[0], templateName+".yaml")
	if data, err := os.ReadFile(yamlPath); err == nil {
		return yamlPath, data, nil
	}
	tplPath := filepath.Join(dirs[0], templateName+".tpl")
	if data, err := os.ReadFile(tplPath); err == nil {
		return tplPath, data, nil
	}
	return "", nil, fmt.Errorf("template %q not found (tried %s and %s)", templateName, yamlPath, tplPath)
}

// loadBundledRecipe reads a recipe from the scaffold embed.FS. The spec is
// "kind/recipe" — same shape as `template scaffold --list` output — so
// users can pipe list output straight into --against-recipe.
func loadBundledRecipe(spec string) ([]byte, error) {
	kind, recipe, ok := strings.Cut(spec, "/")
	if !ok || kind == "" || recipe == "" {
		return nil, fmt.Errorf("--against-recipe must be in kind/recipe form (see 'template scaffold --list')")
	}
	full := path.Join(scaffoldFSRoot, kind, recipe+".yaml")
	data, err := scaffoldFS.ReadFile(full)
	if err != nil {
		available := recipesForKind(kind)
		if len(available) == 0 {
			return nil, fmt.Errorf("no bundled recipes for kind %q; run 'template scaffold --list' to see what's shipped", kind)
		}
		return nil, fmt.Errorf("recipe %q not found for kind %q; available: %s",
			recipe, kind, strings.Join(available, ", "))
	}
	return data, nil
}

// unifiedDiff produces a unified diff in the classic diff -u format, without
// timestamps. Uses a straightforward LCS-based hunk builder; avoids pulling in
// a full diff library for what's a small, human-facing diagnostic. Return is
// empty string when a == b.
//
// When color is false the output is byte-stable — identical inputs diff to
// identical bytes, safe to pipe into tooling. When color is true (interactive
// terminal, color not disabled) header/added/removed lines are wrapped in ANSI
// codes for readability; callers must gate this on a TTY to preserve the
// byte-stable contract for piped consumers.
func unifiedDiff(a, b, aLabel, bLabel string, context int, color bool) string {
	if a == b {
		return ""
	}
	aLines := splitKeepNewline(a)
	bLines := splitKeepNewline(b)

	ops := diffOps(aLines, bLines)
	if len(ops) == 0 {
		return ""
	}

	// Group ops into hunks with `context` lines of surrounding equal context.
	hunks := groupHunks(ops, context)
	if len(hunks) == 0 {
		return ""
	}

	var out bytes.Buffer
	fmt.Fprintf(&out, "%s\n", colorize(color, "cyan", "--- "+aLabel))
	fmt.Fprintf(&out, "%s\n", colorize(color, "cyan", "+++ "+bLabel))
	for _, h := range hunks {
		writeHunk(&out, h, aLines, bLines, color)
	}
	return out.String()
}

// colorize wraps s in the named ANSI color when enabled, delegating to the
// shared funcs.Colorize so --no-color / NO_COLOR are honored consistently.
func colorize(enabled bool, color, s string) string {
	if !enabled {
		return s
	}
	return funcs.Colorize(color, s)
}

// splitKeepNewline splits s on '\n' but preserves the trailing newline on each
// line so diff output prints faithfully. A file that doesn't end in newline
// yields a final line without one, which is what diff -u expects.
func splitKeepNewline(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	i := 0
	for j := 0; j < len(s); j++ {
		if s[j] == '\n' {
			out = append(out, s[i:j+1])
			i = j + 1
		}
	}
	if i < len(s) {
		out = append(out, s[i:])
	}
	return out
}

type diffOp struct {
	kind byte // '=', '-', '+'
	aIdx int  // index into a (for '=' and '-')
	bIdx int  // index into b (for '=' and '+')
}

// diffOps computes a per-line edit script from a to b using LCS. O(n*m) time
// and memory — fine for template files, which are ≤ a few hundred lines.
func diffOps(a, b []string) []diffOp {
	n, m := len(a), len(b)
	// dp[i][j] = LCS length of a[i:] and b[j:]
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	var ops []diffOp
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			ops = append(ops, diffOp{'=', i, j})
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			ops = append(ops, diffOp{'-', i, 0})
			i++
		default:
			ops = append(ops, diffOp{'+', 0, j})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, diffOp{'-', i, 0})
	}
	for ; j < m; j++ {
		ops = append(ops, diffOp{'+', 0, j})
	}
	return ops
}

type hunk struct {
	aStart, aCount int
	bStart, bCount int
	ops            []diffOp
}

// groupHunks turns a flat op stream into unified-diff hunks with `context`
// lines of surrounding equality. Consecutive change regions separated by
// fewer than 2*context equal lines are merged into one hunk.
func groupHunks(ops []diffOp, context int) []hunk {
	var hunks []hunk
	i := 0
	for i < len(ops) {
		// Skip leading equal ops except for the last `context`.
		if ops[i].kind == '=' {
			// Look ahead: is there any change further on?
			hasChange := false
			for k := i; k < len(ops); k++ {
				if ops[k].kind != '=' {
					hasChange = true
					break
				}
			}
			if !hasChange {
				break
			}
			// Find first change index.
			j := i
			for j < len(ops) && ops[j].kind == '=' {
				j++
			}
			// Rewind to include `context` equal lines before the change.
			start := j - context
			if start < i {
				start = i
			}
			i = start
		}
		// Start a hunk at i. Expand while ops are non-equal OR within 2*context
		// equal lines of the next change.
		start := i
		for i < len(ops) {
			if ops[i].kind != '=' {
				i++
				continue
			}
			// Count run of equals.
			runStart := i
			for i < len(ops) && ops[i].kind == '=' {
				i++
			}
			runLen := i - runStart
			// If followed by more changes AND run <= 2*context, keep merging.
			hasMoreChange := false
			for k := i; k < len(ops); k++ {
				if ops[k].kind != '=' {
					hasMoreChange = true
					break
				}
			}
			if !hasMoreChange || runLen > 2*context {
				// Trim trailing equals to `context` and stop.
				keep := runLen
				if keep > context {
					keep = context
				}
				i = runStart + keep
				break
			}
			// else: merged run kept as-is, keep expanding
		}
		hunks = append(hunks, buildHunk(ops[start:i]))
	}
	return hunks
}

func buildHunk(ops []diffOp) hunk {
	h := hunk{ops: ops}
	// Line numbers are 1-based. Find first a-index and first b-index in this
	// slice — including equal ops, which carry both.
	firstA, firstB := -1, -1
	for _, op := range ops {
		switch op.kind {
		case '=':
			if firstA < 0 {
				firstA = op.aIdx
			}
			if firstB < 0 {
				firstB = op.bIdx
			}
			h.aCount++
			h.bCount++
		case '-':
			if firstA < 0 {
				firstA = op.aIdx
			}
			h.aCount++
		case '+':
			if firstB < 0 {
				firstB = op.bIdx
			}
			h.bCount++
		}
	}
	if firstA < 0 {
		firstA = 0
	}
	if firstB < 0 {
		firstB = 0
	}
	h.aStart = firstA + 1
	h.bStart = firstB + 1
	// Unified-diff convention: if count is 0 (pure insertion at start), aStart
	// is one less. For our purposes we always have context so this rarely
	// matters, but preserve it for correctness.
	if h.aCount == 0 {
		h.aStart = firstA
	}
	if h.bCount == 0 {
		h.bStart = firstB
	}
	return h
}

func writeHunk(w *bytes.Buffer, h hunk, aLines, bLines []string, color bool) {
	header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", h.aStart, h.aCount, h.bStart, h.bCount)
	fmt.Fprintf(w, "%s\n", colorize(color, "cyan", header))
	for _, op := range h.ops {
		var prefix, text, clr string
		switch op.kind {
		case '=':
			prefix, text = " ", aLines[op.aIdx]
		case '-':
			prefix, text, clr = "-", aLines[op.aIdx], "red"
		case '+':
			prefix, text, clr = "+", bLines[op.bIdx], "green"
		}
		// The source line may carry a trailing newline; colorize the visible
		// content only, keeping the reset code before the newline so terminals
		// render cleanly and `patch(1)` still sees a plain leading +/-/space.
		body := strings.TrimSuffix(text, "\n")
		line := prefix + body
		if color && clr != "" {
			line = colorize(true, clr, line)
		}
		w.WriteString(line)
		// Ensure hunk lines are newline-terminated even when the source line
		// wasn't — makes the output parseable by `patch(1)`.
		w.WriteByte('\n')
	}
}
