package get

import (
	"strings"

	"github.com/kubectl-cwide/pkg/parser/funcs"
)

// deltaTracker remembers the previously rendered cell values for each logical
// row so that, on the next watch tick, cells whose value changed can be
// highlighted. It's only active in watch mode; a nil tracker means no
// highlighting (the normal, non-watch render path).
//
// Row identity is a stable key derived from the NAME column (and NAMESPACE when
// present), not the row's position — so a row that moves up or down between
// ticks is still correlated with its prior state rather than flagged wholesale.
type deltaTracker struct {
	// headers is the column header list, used to locate the key columns.
	headers []string
	// keyCols are the indices of the columns that form the row key.
	keyCols []int
	// prev maps a row key to its last-rendered cell values.
	prev map[string][]string
	// seenFirstTick is false until the initial listing has been rendered.
	// Cells are never highlighted on the first tick — there's nothing to
	// compare against, and highlighting an entire fresh listing is noise.
	seenFirstTick bool
}

// newDeltaTracker builds a tracker for the given headers. The key columns are
// NAMESPACE (if present) and NAME (if present); if neither exists it falls back
// to the first column, which keeps behavior sane for exotic templates.
func newDeltaTracker(headers []string) *deltaTracker {
	dt := &deltaTracker{
		headers: headers,
		prev:    make(map[string][]string),
	}
	for i, h := range headers {
		switch strings.ToUpper(strings.TrimSpace(h)) {
		case "NAMESPACE", "NAME":
			dt.keyCols = append(dt.keyCols, i)
		}
	}
	if len(dt.keyCols) == 0 && len(headers) > 0 {
		dt.keyCols = []int{0}
	}
	return dt
}

// markFirstTickDone is called once the initial listing has been rendered, so
// that subsequent watch events start highlighting changes.
func (dt *deltaTracker) markFirstTickDone() {
	if dt == nil {
		return
	}
	dt.seenFirstTick = true
}

func (dt *deltaTracker) key(cells []string) string {
	parts := make([]string, 0, len(dt.keyCols))
	for _, i := range dt.keyCols {
		if i < len(cells) {
			parts = append(parts, cells[i])
		}
	}
	return strings.Join(parts, "\x00")
}

// decorate returns a copy of cells with changed cells wrapped in ANSI color and
// records the new values for the next tick. On the first tick (before
// markFirstTickDone) it records baselines and returns cells unchanged. Color is
// gated by funcs.ColorEnabled() (NO_COLOR / --no-color), so when color is off
// the output is byte-identical to the undecorated cells.
//
// Highlighting rules:
//   - a row key never seen before → whole row in green (added)
//   - an existing row with changed cells → only the changed cells in yellow
//   - unchanged cells → left as-is
func (dt *deltaTracker) decorate(cells []string) []string {
	if dt == nil {
		return cells
	}
	k := dt.key(cells)
	prev, existed := dt.prev[k]

	// Always record the latest values for the next comparison.
	stored := make([]string, len(cells))
	copy(stored, cells)
	dt.prev[k] = stored

	if !dt.seenFirstTick || !funcs.ColorEnabled() {
		return cells
	}

	out := make([]string, len(cells))
	if !existed {
		for i, c := range cells {
			out[i] = funcs.Colorize("green", c)
		}
		return out
	}
	for i, c := range cells {
		if i < len(prev) && prev[i] != c {
			out[i] = funcs.Colorize("yellow", c)
		} else {
			out[i] = c
		}
	}
	return out
}
