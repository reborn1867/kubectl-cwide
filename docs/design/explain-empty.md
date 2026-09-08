# Design: `get --explain-empty` (roadmap v0.11.0 #10)

Status: **design** (implementation pending). Grounded in the current parser so
the eventual PR is mostly mechanical. No behavior exists yet.

## Goal

When a template column renders blank (`<none>`), users can't tell whether the
field is genuinely empty on the object or the JSONPath has a typo (the classic
`.status.readReplicas` vs `.status.readyReplicas`). `--explain-empty COL`
diagnoses it: for each object whose `COL` cell is empty, print the JSONPath
tried, the deepest path segment that *did* resolve, and the keys that actually
exist at that level — with a nearest-name suggestion.

## Why the current behavior isn't enough

In `pkg/parser/parser.go`, `FieldParser.Parse` runs `jpParser.FindResults(...)`
and, when it gets zero results, emits `<none>` (parser.go ~L67). No signal about
*where* the path diverged from the object — that's exactly what this feature
adds. It's a diagnostic layer over the same JSONPath, not a new render path.

## Command shape

```
kubectl cwide get <kind> [name...] --explain-empty COL [-t TEMPLATE] [-n NS] [-A]
```

- `COL` is a header from the resolved template (case-insensitive, same matching
  as `-c/--columns`).
- Runs the normal get, then for every row whose `COL` cell is empty/`<none>`,
  emits a diagnostic block to stderr (so normal table stdout stays pipeable).
- Only meaningful for `fieldSpec` (JSONPath) columns. For `template:` columns,
  report "column is a Go template; empty result can't be traced to a single
  path" and skip. For `$_defaultPrinterField`, report "rendered by kubectl's
  default printer" and skip.

## Diagnostic algorithm (per empty cell)

Given the column's JSONPath (already normalized to `{.a.b.c}` form) and the
object as `map[string]interface{}` (unstructured):

1. Tokenize the path into plain segments (`a`, `b`, `c`). Bail out with a
   generic message for paths using filters/wildcards (`[?(...)]`, `[*]`) — those
   don't have a single "missing key" story; just report the path and that it
   matched nothing.
2. Walk segments against the object, tracking the deepest map reached:
   - at each level, if the segment key exists, descend;
   - if it's missing, stop: record the path prefix that resolved, the missing
     segment, and `sortedKeys(currentLevel)`.
3. Emit:
   ```
   COL empty for pod/foo:
     tried:   .status.readReplicas
     resolved: .status  (keys: availableReplicas, readyReplicas, replicas, updatedReplicas)
     missing:  readReplicas
     hint:     did you mean "readyReplicas"?  (Levenshtein-nearest key)
   ```
   The hint is emitted only when the nearest key is within a small edit distance
   (say ≤ 2) of the missing segment.

If the full path resolves but the value is genuinely empty/null/empty-array,
report that instead: `path resolves; value is empty/absent on this object`.

## Reuse map

| Need | Existing code |
|------|---------------|
| resolve template + columns | `resolveTemplatePrinter`, `CustomColumnsPrinter` |
| match COL to a header | `SelectColumns`-style case-insensitive match |
| classify column kind | `common.DefaultPrinterField`, `YAMLColumn.Template` |
| JSONPath normalization | `RelaxedJSONPathExpression` (customcolumn.go) |
| object as map | `runtime.DefaultUnstructuredConverter.ToUnstructured` |
| nearest-key suggestion | small local Levenshtein (like #4's field suggestion note) |

## Output & flags

- Diagnostics go to **stderr**; the normal rendered table still goes to stdout,
  so `--explain-empty` composes with a normal run and piping is unaffected.
- Cap the number of diagnostic blocks (e.g. first 20 empty rows) with a
  `... (N more empty rows)` footer, so a fully-empty column on a big list
  doesn't flood the terminal. Log the cap — never silently truncate.

## Non-goals

- No schema/OpenAPI fetch — this diagnoses against the *actual returned object*,
  which is cheaper and needs no extra RBAC. Schema-based validation is #13
  (`template lint --against-crd`).
- No auto-fixing the template.

## Test plan (cluster-free; feed synthetic unstructured objects)

- typo mid-path → resolved prefix + existing keys + nearest-key hint.
- missing top-level segment → resolved is root, keys are top-level.
- full path resolves but value is `null`/`""`/`[]` → "value empty" message.
- filter/wildcard path → generic "matched nothing" message, no key walk.
- template column / default-printer column → skip with the right explanation.
- Levenshtein hint only fires within the distance threshold.
