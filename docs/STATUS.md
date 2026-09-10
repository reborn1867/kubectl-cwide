# Work status & handoff

Single source of truth for work staged on branches but not yet merged. All
branches are pushed to `origin`. Nothing here is merged to `main` yet.

**Verification:** the Go toolchain became available and the code branches below
were compiled and tested (`go build ./... && go vet ./... && go test ./...`).
Entries marked **verified green** built and passed all tests; docs/data branches
are inspection-verified. (The toolchain access is intermittent across sessions,
so re-run the suite in CI before merge regardless.)

## Mergeable now — docs/data only, no CI dependency

| Branch | What |
|--------|------|
| `fix/cookbook-job-completions` | fix broken Job COMPLETIONS recipe (`/` isn't a JSONPath op) |
| `fix/migration-dir-naming` | correct dir-name rule: singular Kind, not plural |
| `docs/alias-group-typename-note` | document group-alias + TYPE/NAME limitation |
| `docs/roadmap-status-sync` | sync ROADMAP statuses to reality |
| `docs/explain-design` | design spec for `explain` (#9) |
| `docs/explain-empty-design` | design spec for `get --explain-empty` (#10) |
| `docs/merge-templates-design` | design spec for `get --merge-templates` (#11) |

## Code / features — verified green locally; confirm in CI

| Branch | What | Status |
|--------|------|--------|
| `fix/default-template-matches-kubectl-get` | **`kc cwide get <resource>` matches `kubectl get` (not `-o wide`)** — init builds default template from the non-wide column set | verified green |
| `fix/filter-earliest-operator` | `--filter` splits on earliest operator (fixes values containing operators) + tests | verified green |
| `fix/completion-alias-entries` | shell completion includes rich `AliasEntries`, not just legacy `Aliases` + tests | verified green |
| `feat/explain-command` | new `cwide explain` for templates/aliases (#9) + completion + tests | verified green |
| `feat/explain-empty-core` | path-tracing diagnostic core for `--explain-empty` (#10) + tests | verified green |
| `feat/explain-empty-wire` | `--explain-empty` flag wired onto the core (#10) | verified green (based on -core) |

## The v0.10.0 feature batch

`integration/v0.10.0` pre-merges (zero conflicts) the three v0.10.0 features:
`feat/template-diff` (#1), `feat/watch-delta-highlight` (#7),
`feat/label-columns` (#8), plus the scaffold nil-guard and the
`needsDefaultPrinter` fix (a nil-panic caught by compiling — see below). One PR
ships the batch. **Verified green** (build + vet + full test suite). See
`docs/RELEASE_v0.10.0.md`.

Note: `feat/explain-command`/`-empty-*` (#9/#10) and
`fix/default-template-matches-kubectl-get` are **not** in `integration/v0.10.0`
— land them separately.

## Fix found by compiling (regression the earlier no-toolchain window hid)

`feat/label-columns`' end-to-end test passed a nil writer / lacked a
DefaultTableGenerator and panicked once compiled. Fixed by guarding the
default-printer table build behind `needsDefaultPrinter()` (also an efficiency
win — no per-object table work for pure JSONPath/template/label templates) and
using `io.Discard` in the test. Fix is on `feat/label-columns` and cherry-picked
into `integration/v0.10.0`.

## Suggested order to land

1. Merge the docs/data branches (no CI needed).
2. Merge `fix/default-template-matches-kubectl-get` — the get-parity fix.
3. Open PRs for `fix/filter-earliest-operator`, `fix/completion-alias-entries`.
4. Open one PR for `integration/v0.10.0`; on green, tag `v0.10.0` (release
   workflow builds binaries + opens the krew-index PR automatically).
5. Open PRs for `feat/explain-command` and the `feat/explain-empty-*` pair
   (land -core before -wire).

## Deferred (needs a compiler in the loop)

ROADMAP #11 `get --merge-templates` is designed (`docs/merge-templates-design`)
but intentionally **not implemented** — its merged-helper/localTemplate re-parse
(`.yaml` vs `.tpl` handling) is easy to get subtly wrong and should be built
with `go test` running, not authored blind.
