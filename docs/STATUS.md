# Work status & handoff

Single source of truth for work staged on branches but not yet merged. All
branches are pushed to `origin`. Nothing here is merged to `main` yet.

**Environment note:** this batch was authored in a dev environment with **no Go
toolchain** (couldn't `go build`/`test`/`vet`) and **no PR access to
github.com**. Code branches are marked accordingly — CI must build/test them
before merge. Docs/data branches are verified by inspection and safe to merge
without CI.

## Mergeable now — docs/data only, no CI dependency

| Branch | What | Risk |
|--------|------|------|
| `fix/cookbook-job-completions` | fix broken Job COMPLETIONS recipe (`/` isn't a JSONPath op) | none (markdown) |
| `fix/migration-dir-naming` | correct dir-name rule: singular Kind, not plural | none (markdown) |
| `docs/alias-group-typename-note` | document group-alias + TYPE/NAME limitation | none (markdown) |
| `docs/roadmap-status-sync` | sync ROADMAP statuses to reality (avoids re-implementing shipped features) | none (markdown) |
| `docs/explain-design` | design spec for `explain` (#9) | none (markdown) |
| `feat/template-diff` scaffold guard | PVC bound-node nil-guard (also in integration) | none (yaml/template) |

## Needs CI (build/test) before merge — code

| Branch | What | Verification |
|--------|------|--------------|
| `fix/filter-earliest-operator` | `--filter` splits on earliest operator, not by priority (fixes values containing operators) + tests | inspection only |
| `fix/completion-alias-entries` | shell completion includes rich `AliasEntries`, not just legacy `Aliases` + tests | inspection only |
| `feat/explain-command` | new `cwide explain` for templates/aliases (#9) + completion + tests | inspection only |

## Needs CI — the v0.10.0 feature batch

`integration/v0.10.0` pre-merges (zero conflicts) the three v0.10.0 features:
`feat/template-diff` (#1), `feat/watch-delta-highlight` (#7),
`feat/label-columns` (#8), plus the scaffold nil-guard. One PR ships the batch.
See `docs/RELEASE_v0.10.0.md` for detail.

Note: `feat/explain-command` (#9, v0.11.0) is **not** in `integration/v0.10.0` —
land it separately after the v0.10.0 batch.

## Suggested order to land

1. Merge the docs/data branches (no CI needed) — immediate user-facing fixes.
2. Open PRs for `fix/filter-earliest-operator` and `fix/completion-alias-entries`;
   merge once CI is green.
3. Open one PR for `integration/v0.10.0`; on green, tag `v0.10.0` (the release
   workflow builds binaries + opens the krew-index PR automatically).
4. Open a PR for `feat/explain-command`; merge on green. Then implement
   ROADMAP #10 (`get --explain-empty`) next.

## If the Go toolchain returns in this environment

Before anything else, `go build ./... && go test ./... && go vet ./...` on each
code branch to convert "inspection only" → verified, and fix any compile issues
the environment prevented catching (the branches are isolated, so fixes are
contained).
