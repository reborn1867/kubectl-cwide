package get

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"sigs.k8s.io/yaml"

	"k8s.io/cli-runtime/pkg/printers"
)

// renderRows emits the collected rows in the requested format. Note that
// "yaml" and "json" are intercepted upstream by emitNative and never reach
// this function; the "template-yaml"/"template-json" values below dump the
// rendered template columns as records rather than the raw resource.
func renderRows(out io.Writer, format string, headers []string, rows [][]string) error {
	switch strings.ToLower(format) {
	case "template-json":
		objs := rowsAsObjects(headers, rows)
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(objs)
	case "template-yaml":
		objs := rowsAsObjects(headers, rows)
		data, err := yaml.Marshal(objs)
		if err != nil {
			return err
		}
		_, err = out.Write(data)
		return err
	case "csv":
		w := csv.NewWriter(out)
		defer w.Flush()
		if len(headers) > 0 {
			if err := w.Write(headers); err != nil {
				return err
			}
		}
		for _, r := range rows {
			// pad short rows to header width so csv shape stays regular
			if len(r) < len(headers) {
				padded := make([]string, len(headers))
				copy(padded, r)
				r = padded
			}
			if err := w.Write(r); err != nil {
				return err
			}
		}
		return nil
	case "table", "":
		w := printers.GetNewTabWriter(out)
		defer w.Flush()
		if len(headers) > 0 {
			fmt.Fprintln(w, strings.Join(headers, "\t"))
		}
		for _, r := range rows {
			fmt.Fprintln(w, strings.Join(r, "\t"))
		}
		return nil
	default:
		return fmt.Errorf("unsupported output format %q (expected csv, template-json, template-yaml, or table)", format)
	}
}

// filterRows keeps rows where every expression evaluates true.
// Supported operators: =, ==, !=, ~ (regex match), !~ (regex non-match).
// Column names are case-insensitive.
func filterRows(headers []string, rows [][]string, exprs []string) ([][]string, error) {
	preds := make([]filterPredicate, 0, len(exprs))
	headerIdx := headerIndexMap(headers)
	for _, e := range exprs {
		p, err := parseFilterExpr(e, headerIdx)
		if err != nil {
			return nil, err
		}
		preds = append(preds, p)
	}

	out := make([][]string, 0, len(rows))
	for _, r := range rows {
		keep := true
		for _, p := range preds {
			var cell string
			if p.colIdx < len(r) {
				cell = r[p.colIdx]
			}
			switch p.op {
			case "=", "==":
				if cell != p.value {
					keep = false
				}
			case "!=":
				if cell == p.value {
					keep = false
				}
			case "~":
				if !p.re.MatchString(cell) {
					keep = false
				}
			case "!~":
				if p.re.MatchString(cell) {
					keep = false
				}
			}
			if !keep {
				break
			}
		}
		if keep {
			out = append(out, r)
		}
	}
	return out, nil
}

func headerIndexMap(headers []string) map[string]int {
	m := make(map[string]int, len(headers))
	for i, h := range headers {
		m[strings.ToUpper(h)] = i
	}
	return m
}

type filterPredicate struct {
	colIdx int
	op     string
	value  string
	re     *regexp.Regexp
}

func parseFilterExpr(expr string, headerIdx map[string]int) (filterPredicate, error) {
	// Order matters: match longer operators first.
	for _, op := range []string{"!~", "!=", "==", "~", "="} {
		if i := strings.Index(expr, op); i > 0 {
			col := strings.TrimSpace(expr[:i])
			val := expr[i+len(op):]
			idx, ok := headerIdx[strings.ToUpper(col)]
			if !ok {
				return filterPredicate{}, fmt.Errorf("unknown column %q in filter %q", col, expr)
			}
			p := filterPredicate{colIdx: idx, op: op, value: val}
			if op == "~" || op == "!~" {
				re, err := regexp.Compile(val)
				if err != nil {
					return filterPredicate{}, fmt.Errorf("bad regex in filter %q: %w", expr, err)
				}
				p.re = re
			}
			return p, nil
		}
	}
	return filterPredicate{}, fmt.Errorf("filter %q must contain =, ==, !=, ~, or !~", expr)
}

// sortRows sorts rows in place by the named column. Numeric strings sort
// numerically; otherwise, lexicographically.
func sortRows(headers []string, rows [][]string, colName string) error {
	idx, ok := headerIndexMap(headers)[strings.ToUpper(colName)]
	if !ok {
		return fmt.Errorf("unknown sort-by column %q", colName)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		var a, b string
		if idx < len(rows[i]) {
			a = rows[i][idx]
		}
		if idx < len(rows[j]) {
			b = rows[j][idx]
		}
		af, aErr := strconv.ParseFloat(a, 64)
		bf, bErr := strconv.ParseFloat(b, 64)
		if aErr == nil && bErr == nil {
			return af < bf
		}
		return a < b
	})
	return nil
}

func rowsAsObjects(headers []string, rows [][]string) []map[string]string {
	objs := make([]map[string]string, 0, len(rows))
	for _, r := range rows {
		m := make(map[string]string, len(headers))
		for i, h := range headers {
			if i < len(r) {
				m[h] = r[i]
			} else {
				m[h] = ""
			}
		}
		objs = append(objs, m)
	}
	return objs
}
