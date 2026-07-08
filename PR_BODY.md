# brainstorm/auto — autonomous loop over brainstorm.md

This PR is the output of a single autonomous run through the task list in `brainstorm.md`. Every implemented item is one commit; every skipped item is called out below with the reason.

## Summary of what shipped

### Features
- **Shell completion** (`kubectl cwide completion bash|zsh|fish|powershell`)
- **`get -c/--columns`** — project a subset of the template's columns without editing it
- **`get -o json|yaml|csv`** — structured output of rendered rows
- **`get --sort-by=COL` and `get --filter=COL=val|~regex`** — post-render row transforms; multiple `--filter`s AND together
- **`tree --max-depth` and cycle detection** — safe rendering when ownerRef graphs loop
- **`tree --reverse`** — walk ancestors via ownerReferences
- **New template functions**: `humanBytes`, `age`, `truncate`, `b64dec`, `colorIf`, `safeIndex`
- **`--no-color`** root flag + `NO_COLOR` env respected
- **`template lint <file>`** — static validation of column templates
- **`template scaffold <resource>`** — starter template for a kind
- **Template inheritance**: `_shared/*.tpl` under template root is prepended to every template
- **Per-context / per-namespace default template** in `~/.kubectl-cwide/config.yaml`
- **Alias groups**: `alias set core pod,svc,cm` now works transparently
- **Cluster-scoped alias sync** via a reserved `__aliases__` key in the templates ConfigMap
- **Marketplace version pinning**: `install --ref <sha|tag>` writes `~/.kubectl-cwide/marketplace.lock`
- **Signal handling** at the root so Ctrl-C cancels mid-request cleanly

### Refactors / infra
- All `context.TODO()` sites now use `cmd.Context()`
- Client factory construction consolidated in `pkg/clients` (removes 4 copies of the same setup)
- Native k8s template set curated under `templates/native/`
- Cookbook (`docs/cookbook.md`) and migration guide (`docs/migration-from-custom-cols.md`)

### CI
- `test.yml` workflow: Go 1.23/1.24 × linux/mac/windows
- `e2e.yml` workflow: runs `hack/e2e-test.sh` against a kind cluster on every PR

### Test coverage
- `pkg/parser/funcs/builtins_test.go` — HumanBytes, Truncate, B64Dec, ColorIf, SafeIndex
- `pkg/cmd/marketplace/lock_test.go` — LockFile upsert semantics
- `pkg/cmd/template/lint_test.go` (added by hook — sanity checks lint output)

### Housekeeping
- README typo fix (`kubectl-ciwe` → `kubectl-cwide`)
- Removed the previously-shipped `list all` command (already gone on `main` — pre-loop cleanup)

## Skipped (with reasons)

| # | Item | Why skipped |
|---|------|-------------|
| 22 | Marketplace signed templates | Needs real cosign/sigstore infra (KMS keys, transparency log, published signatures) — not viable from sandbox; needs owner decision on trust model |
| 23 | Marketplace private registries (OCI/S3/GitLab) | Each source needs its own client + auth + end-to-end tests; needs prioritization decision first |
| 24 | Marketplace publish command | Needs a real community index repo to PR against; no such repo exists yet (marketplace is index-in-source) |
| 25 | Interactive TUI (Bubbletea) | Project-scale work (list views, focus, fuzzy search, live data binding, tests); can't be honestly done in one loop iteration |
| 26 | `diff` command | Useful diff needs either live cluster + last-applied-configuration annotation, or git history, or explicit --from/--to — each a different feature |
| 27 | `get all-resources` partial-failure | Already correct — audited in-place, no code change needed |
| 28 | Discovery cache | `factory.ToDiscoveryClient()` is already a CachedDiscoveryClient (~/.kube/cache/discovery); no work to do |
| 29 | `template edit` race protection | File lock via os.O_EXCL solves single-host only; multi-host is the real failure mode. Needs design discussion |
| 30 | -A audit | Already consistent — audited, no change needed |
| 31 | Windows path handling | Audited — all filesystem paths use filepath.Join; only unrelated `/` uses are URL/GVK strings. Full support needs CI matrix (#36) to verify |
| 35 | Drop vendor dir | Affects offline builds and CI; policy call, not mechanical |
| 39 | Verify goreleaser + krew index | Needs live GitHub Actions observation for v0.7.0 — can't do from sandbox |
| 42 | Restructure README into `docs/` | Would break existing anchor links and reorganize content krew references. Individual doc pages added under `docs/` instead |

## Not covered (was implemented pre-loop or by hook)
- **Watch mode** — already present in `pkg/cmd/get/get.go` (`--watch`/`-w`)
- **Tree custom edge types** — already supported via `--rules` YAML

## Testing
- All existing tests pass: `go test ./...`
- New tests added (see above)
- CI matrix will exercise on Go 1.23/1.24 × linux/mac/windows once merged

## How to open

Because this sandbox can't reach github.com, please run manually:

```sh
gh pr create --repo reborn1867/kubectl-cwide \
  --base main --head brainstorm/auto \
  --title "brainstorm/auto: bulk implementation of brainstorm.md" \
  --body-file PR_BODY.md
```

or open through the GitHub web UI.
