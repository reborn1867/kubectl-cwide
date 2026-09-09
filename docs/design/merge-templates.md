# Design: `get --merge-templates` (roadmap v0.11.0 #11)

Status: **design** (implementation deferred — see "Why implementation waits").
Grounded in the current printer so the eventual PR is well-scoped.

## Goal

Render one resource list with the columns of two (or more) templates layered
together — a base template plus a diagnostic overlay — without hand-authoring a
combined template. Example: everyday `default` columns plus a `debug` template's
`RESTARTS`/`LAST_REASON` columns in a single view.

```
kubectl cwide get pod --merge-templates default,restart-reason
kubectl cwide get pod -t default --merge-templates restart-reason   # base + overlay
```

## Column model (what exists)

A resolved template is a `CustomColumnsPrinter` with parallel `Columns
[]Column` and `Headers []string` (see customcolumn.go). Each `Column` is
`{Header, FieldSpec, Template, IsTemplate}`. `SelectColumns` already
demonstrates rebuilding both slices in lockstep — the merge does the same,
appending instead of filtering.

## Semantics

1. Resolve each named template to its `[]Column` via the existing
   `resolveTemplatePrinter` (honors `.yaml`→`.tpl` and `_shared` helpers).
2. Concatenate columns left-to-right in the order templates are listed. The
   first template is the base; each subsequent one overlays.
3. **Duplicate headers** (case-insensitive, matching `SelectColumns`'s
   normalization): **first occurrence wins**, later duplicates are dropped, so
   the base template's version of a shared column (e.g. `NAME`) is authoritative
   and overlays only contribute *new* columns. Log dropped duplicates to stderr
   so the user knows an overlay column was shadowed.
4. **Helpers/Funcs blocks** (`.yaml` templates carry `Helpers`/`Funcs`): union
   them. On a helper-name collision across templates, the base wins and a
   warning is logged — mirrors how duplicate columns resolve. `.tpl` bodies
   concatenate (helpers are `{{define}}` blocks; Go's template parser errors on
   a redefined name, so dedupe define names before concatenation or surface the
   parse error clearly).
5. The merged column set flows through the *existing* render/`-c`/`--sort-by`/
   `--filter`/`-o` pipeline unchanged — merge only builds a wider `Columns`.

## Interaction with `-t/--template`

- `--merge-templates a,b,c` with no `-t`: the list is the full stack (a is base).
- `-t base --merge-templates x,y`: equivalent to `--merge-templates base,x,y`.
- `-c/--columns` applies *after* the merge, so users can project the merged
  superset down.

## Implementation sketch

- Add `MergeTemplates []string` to `GetOptions` + a `--merge-templates` flag
  (StringSlice), with completion via `completions.TemplateNames`.
- New helper `mergePrinters(base *CustomColumnsPrinter, overlays ...*CustomColumnsPrinter)`
  in the get package: dedupe-append Columns/Headers (reuse the case-insensitive
  key from SelectColumns), union Helpers/Funcs, return a printer whose
  localTemplate is re-parsed from the merged helpers.
- In `createPrinter`, when `MergeTemplates` is set, resolve each and merge
  before applying `SelectColumns`.

## Why implementation waits

This is the roadmap's one **L-effort** item in v0.11.0. The subtle part is the
helper/`localTemplate` re-parse (the merged `{{define}}` set must recompile
cleanly, and `.tpl` vs `.yaml` helper handling differs). That's exactly the kind
of change that needs a compiler + `go test` in the loop to get right, so — per
the working note in the other design docs — the implementation should land when
a Go toolchain is available, not be authored blind. This spec de-risks it so the
build, when it happens, is mechanical rather than exploratory.

## Test plan (cluster-free)

- two templates, disjoint columns → concatenated in order.
- overlapping header → first wins, duplicate dropped, warning emitted.
- `-t base --merge x` equals `--merge base,x`.
- `-c` after merge projects the superset.
- `.yaml` helper union; helper-name collision → base wins + warning.
- unknown template name in the list → clear error listing available templates.
