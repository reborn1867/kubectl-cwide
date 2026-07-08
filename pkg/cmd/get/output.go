package get

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"sigs.k8s.io/yaml"
)

// renderRows emits the collected rows in the requested format.
func renderRows(out io.Writer, format string, headers []string, rows [][]string) error {
	switch strings.ToLower(format) {
	case "json":
		objs := rowsAsObjects(headers, rows)
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(objs)
	case "yaml":
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
	default:
		return fmt.Errorf("unsupported output format %q (expected json, yaml, or csv)", format)
	}
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
