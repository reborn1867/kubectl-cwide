package get

import (
	"fmt"
	"sort"
	"strings"
)

// EmptyDiagnosis is the result of tracing why a JSONPath produced no value on a
// given object. It is produced by diagnoseEmptyPath and rendered for the user
// by --explain-empty. Pure data — no cluster or template dependency.
type EmptyDiagnosis struct {
	// Path is the JSONPath that was traced (normalized ".a.b.c" form).
	Path string
	// Untraceable is set when the path can't be walked segment-by-segment
	// (filters, wildcards); Reason explains why. The remaining fields are then
	// zero.
	Untraceable bool
	Reason      string
	// ResolvedPrefix is the deepest dotted prefix that existed on the object
	// (e.g. ".status"). Empty means even the first segment was missing (root).
	ResolvedPrefix string
	// MissingSegment is the first path segment that did not exist at
	// ResolvedPrefix. Empty when the whole path resolved.
	MissingSegment string
	// AvailableKeys are the map keys present at ResolvedPrefix, sorted.
	AvailableKeys []string
	// Suggestion is the nearest available key to MissingSegment when within a
	// small edit distance; empty otherwise.
	Suggestion string
	// FullyResolved is true when every segment existed but the value itself was
	// empty/absent (nil, "", or empty slice/map) — i.e. not a typo.
	FullyResolved bool
}

// suggestionMaxDistance bounds how far a nearest key can be and still be
// suggested, so we don't propose wildly different keys.
const suggestionMaxDistance = 2

// diagnoseEmptyPath walks a dotted JSONPath (accepts ".a.b", "a.b", "{.a.b}")
// against obj (an unstructured object map) and explains why it yielded nothing.
// It only handles plain key paths; paths with filters/wildcards/indexing are
// reported as Untraceable.
func diagnoseEmptyPath(path string, obj map[string]interface{}) EmptyDiagnosis {
	d := EmptyDiagnosis{Path: normalizeDotPath(path)}

	segs, ok := plainSegments(d.Path)
	if !ok {
		d.Untraceable = true
		d.Reason = "path uses filters, wildcards, or indexing; can't trace a single missing key"
		return d
	}
	if len(segs) == 0 {
		d.Untraceable = true
		d.Reason = "empty path"
		return d
	}

	cur := obj
	resolved := make([]string, 0, len(segs))
	for i, seg := range segs {
		val, present := cur[seg]
		if !present {
			d.MissingSegment = seg
			d.ResolvedPrefix = dotPrefix(resolved)
			d.AvailableKeys = sortedMapKeys(cur)
			d.Suggestion = nearestKey(seg, d.AvailableKeys)
			return d
		}
		resolved = append(resolved, seg)
		// Last segment resolved — the value exists; decide if it's "empty".
		if i == len(segs)-1 {
			d.FullyResolved = true
			d.ResolvedPrefix = dotPrefix(resolved)
			return d
		}
		// Descend; if the next level isn't a map we can't continue keying in.
		next, isMap := val.(map[string]interface{})
		if !isMap {
			// The path expects to descend but the value is a scalar/slice.
			d.MissingSegment = segs[i+1]
			d.ResolvedPrefix = dotPrefix(resolved)
			d.AvailableKeys = nil // not a map: no keys to list
			d.Reason = fmt.Sprintf("%s is not an object; can't descend to %q", d.ResolvedPrefix, segs[i+1])
			return d
		}
		cur = next
	}
	return d
}

// normalizeDotPath turns "{.a.b}", "{a.b}", ".a.b", "a.b" into ".a.b".
func normalizeDotPath(path string) string {
	s := strings.TrimSpace(path)
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if !strings.HasPrefix(s, ".") {
		s = "." + s
	}
	return s
}

// plainSegments splits a normalized ".a.b.c" path into ["a","b","c"]. Returns
// ok=false if any segment contains characters implying a filter/wildcard/index
// ([ ] * ? ( ) @), which this tracer intentionally doesn't handle.
func plainSegments(dotPath string) (segs []string, ok bool) {
	trimmed := strings.TrimPrefix(dotPath, ".")
	if trimmed == "" {
		return nil, true
	}
	if strings.ContainsAny(trimmed, "[]*?()@") {
		return nil, false
	}
	for _, s := range strings.Split(trimmed, ".") {
		if s == "" {
			return nil, false
		}
		segs = append(segs, s)
	}
	return segs, true
}

func dotPrefix(resolved []string) string {
	if len(resolved) == 0 {
		return ""
	}
	return "." + strings.Join(resolved, ".")
}

func sortedMapKeys(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// nearestKey returns the closest key to target by Levenshtein distance, but
// only when that distance is within suggestionMaxDistance. Empty otherwise.
func nearestKey(target string, keys []string) string {
	best := ""
	bestDist := suggestionMaxDistance + 1
	for _, k := range keys {
		if d := levenshtein(target, k); d < bestDist {
			bestDist = d
			best = k
		}
	}
	if bestDist <= suggestionMaxDistance {
		return best
	}
	return ""
}

// levenshtein computes the edit distance between a and b (iterative, two-row).
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = min3(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

// FormatEmptyDiagnosis renders a diagnosis as a short human-readable block,
// prefixed with the column header and an object identifier.
func FormatEmptyDiagnosis(col, objID string, d EmptyDiagnosis) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s empty for %s:\n", col, objID)
	fmt.Fprintf(&b, "  tried:    %s\n", d.Path)
	switch {
	case d.Untraceable:
		fmt.Fprintf(&b, "  note:     %s\n", d.Reason)
	case d.FullyResolved:
		fmt.Fprintf(&b, "  resolved: %s (path resolves; value is empty/absent on this object)\n", d.ResolvedPrefix)
	default:
		if d.ResolvedPrefix == "" {
			fmt.Fprintf(&b, "  resolved: <root>")
		} else {
			fmt.Fprintf(&b, "  resolved: %s", d.ResolvedPrefix)
		}
		if len(d.AvailableKeys) > 0 {
			fmt.Fprintf(&b, " (keys: %s)", strings.Join(d.AvailableKeys, ", "))
		}
		fmt.Fprintln(&b)
		fmt.Fprintf(&b, "  missing:  %s\n", d.MissingSegment)
		if d.Reason != "" {
			fmt.Fprintf(&b, "  note:     %s\n", d.Reason)
		}
		if d.Suggestion != "" {
			fmt.Fprintf(&b, "  hint:     did you mean %q?\n", d.Suggestion)
		}
	}
	return b.String()
}
