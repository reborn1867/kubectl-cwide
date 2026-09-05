# v0.10.0 release prep — "power-user get"

Tracking doc for the v0.10.0 milestone. Everything here is additive and
non-breaking. This file coordinates landing the three feature branches that
make up the milestone; delete it (or fold it into the changelog) once the tag
is cut.

## Milestone contents

v0.10.0 delivers the "power-user get" batch from `ROADMAP.md`. Items #4–#6
(`--sort-by`, `--filter`, `-o csv|template-json|template-yaml`) already shipped
in v0.8.0, so the remaining new work is items #7 and #8 plus the local
`template diff` (#1) that was staged during the v0.9.4 cycle.

| Roadmap # | Feature | Branch | Base | Status |
|-----------|---------|--------|------|--------|
| 1 | `template diff --against-file / --against-recipe` | `feat/template-diff` | current main (rebased) | ready; rebased onto `be420f4`, diff is clean; verify in CI |
| 7 | `get -w` change highlighting | `feat/watch-delta-highlight` | current main | ready; verified `go build`/`go test`/`go vet` green |
| 8 | `get -L/--label-columns` + `--show-labels` | `feat/label-columns` | current main | ready; verify in CI |

## Single integration branch (recommended path)

All three feature branches have been pre-merged into **`integration/v0.10.0`**
(off current `main`) — the three merges applied with **zero conflicts**. The
combined diff is purely additive (18 files, ~+1589/−42) across `pkg/cmd/get`,
`pkg/cmd/template`, and `pkg/parser/funcs`, with no phantom deletions.

The get-path features compose correctly: in `printOneObject`, label-column
values are computed before watch delta-decoration, and structured output
(`RowSink`) still emits undecorated cells — verified by inspection.

Easiest path to ship: open **one** PR from `integration/v0.10.0` → `main`, let
CI verify the whole batch, and merge. (The per-branch PRs below remain a valid
alternative if you prefer to review each feature separately.)

## Recommended merge order (if landing branches individually)

1. **`feat/watch-delta-highlight`** — self-contained; touches the watch path and
   adds `funcs.Colorize`/`ColorEnabled`. No overlap with the others.
2. **`feat/label-columns`** — touches `pkg/cmd/get/customcolumn.go` and
   `get.go`. Overlaps the same files as #7 only in disjoint regions
   (watch loop vs. column assembly), so a trivial merge; land after #7 to keep
   conflict resolution one-directional.
3. **`feat/template-diff`** — **already rebased onto current main** (tip
   `f2176af`; `be420f4` is now an ancestor). The rebase applied cleanly with no
   conflicts — the branch only touches `pkg/cmd/template/*`, disjoint from the
   v0.9.4 fix's `pkg/utils/*` changes. Its diff against main is now purely the
   `template diff` + scaffold-examples addition; the earlier phantom deletion of
   `pkg/utils/create_yaml_test.go` is gone. Nothing further needed before the PR.

   ```sh
   # verification already done; to re-confirm:
   git diff --stat main..feat/template-diff   # only pkg/cmd/template/* files
   ```

## Per-branch summary

### #7 — `feat/watch-delta-highlight`

`kubectl cwide get <kind> -w` highlights cells that changed since the previous
tick (changed cell → yellow, new row → green). Rows correlate by identity
(NAMESPACE+NAME), not position; the initial listing is the baseline and is never
highlighted; honors `--no-color`/`NO_COLOR`; structured output is never
decorated. Adds `pkg/cmd/get/delta.go` + tests.

### #8 — `feat/label-columns`

kubectl-parity label surfacing without editing a template:
- `-L/--label-columns=app,tier` (repeatable) — one column per label key; header
  is the key verbatim; empty when absent. Keys with dots/slashes work as-is.
- `--show-labels` — a trailing `LABELS` column of sorted `k=v` pairs (`<none>`
  when the object has no labels).

Appended after `-c/--columns` selection (never filtered out) and composes with
`--sort-by`, `--filter`, and `-o` formats. Adds `pkg/cmd/get/label_columns_test.go`.

### #1 — `feat/template-diff`

`kubectl cwide template diff -r <kind> -t <name> --against-file <path> |
--against-recipe <kind/recipe>` prints a local unified diff. No cluster contact,
byte-stable output. Adds `pkg/cmd/template/diff.go` + tests.

## Cut checklist

- [ ] Merge #7, #8, (rebased) #1 to `main` via PRs; CI green on the Go
      1.23/1.24 × linux/macOS/windows matrix (`go build`/`go test`/`go vet`).
- [ ] Update `ROADMAP.md`: mark #1/#7/#8 `done`; note #4/#5/#6 already shipped in
      v0.8.0; note #23 (krew-index PR from release workflow) already implemented
      via the `krew-release-bot` step in `.github/workflows/release.yml`.
- [ ] Refresh the README "New in vX" section for the get flags and watch
      highlighting (per-branch README edits already land with each branch).
- [ ] Tag `v0.10.0` on `main` — the `release` workflow (GoReleaser +
      krew-release-bot) builds binaries, publishes the release, and opens the
      krew-index PR automatically.

## Notes

- No breaking changes; the `metadata:` block and all v0.9.x template files keep
  parsing.
- `gh` in the maintainer's CI/dev shell must target `github.com` (not an
  internal host) to open these PRs.
