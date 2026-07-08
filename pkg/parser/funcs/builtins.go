package funcs

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/util/duration"
)

// HumanBytes formats an integer or numeric-string byte count using SI-like
// binary units (KiB, MiB, GiB, TiB, PiB). Non-numeric input is returned as-is.
func HumanBytes(v interface{}) string {
	n, ok := toInt64(v)
	if !ok {
		if s, isStr := v.(string); isStr {
			return s
		}
		return ""
	}
	if n < 0 {
		return "-" + HumanBytes(-n)
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	suffix := "KMGTPE"[exp]
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), suffix)
}

// Age formats an RFC3339 timestamp string as a human-readable duration since
// then (e.g. "3d", "5h12m"). Empty or unparseable input returns "".
func Age(v interface{}) string {
	s, ok := v.(string)
	if !ok || s == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return ""
	}
	return duration.HumanDuration(time.Since(t))
}

// Truncate cuts a string at n runes and appends "…" if it was truncated.
// n <= 0 returns the input unchanged.
func Truncate(n int, s string) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

// B64Dec base64-decodes a string. On decode error, returns the input unchanged.
// Handy for viewing Secret data keys in templates.
func B64Dec(s string) string {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return s
	}
	return string(data)
}

// ColorIf wraps `text` in a terminal color code when `cond` is truthy.
// color: one of "red", "green", "yellow", "blue", "cyan", "magenta", "gray".
// Unrecognized colors return the text unwrapped.
func ColorIf(cond bool, color, text string) string {
	if !cond {
		return text
	}
	code, ok := ansiColorCodes[color]
	if !ok {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

var ansiColorCodes = map[string]string{
	"red":     "31",
	"green":   "32",
	"yellow":  "33",
	"blue":    "34",
	"magenta": "35",
	"cyan":    "36",
	"gray":    "90",
}

// SafeIndex walks a nested map/slice structure by keys/indices, returning
// "" (or the zero value) if any level is missing. String keys index maps;
// integer keys or numeric strings index slices.
//
// Example: {{ safeIndex . "status" "containerStatuses" 0 "restartCount" }}
func SafeIndex(root interface{}, path ...interface{}) interface{} {
	cur := root
	for _, p := range path {
		if cur == nil {
			return ""
		}
		switch v := cur.(type) {
		case map[string]interface{}:
			key, ok := p.(string)
			if !ok {
				return ""
			}
			cur = v[key]
		case []interface{}:
			idx, ok := toInt(p)
			if !ok || idx < 0 || idx >= len(v) {
				return ""
			}
			cur = v[idx]
		default:
			return ""
		}
	}
	if cur == nil {
		return ""
	}
	return cur
}

func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case float32:
		return int64(n), true
	case float64:
		return int64(n), true
	case string:
		i, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			return 0, false
		}
		return i, true
	}
	return 0, false
}

func toInt(v interface{}) (int, bool) {
	n, ok := toInt64(v)
	if !ok {
		return 0, false
	}
	return int(n), true
}
