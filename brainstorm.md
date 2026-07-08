# kubectl-cwide — Brainstorm

A working list of ideas and known rough edges. Nothing here is committed to; use it as a menu.

## Features

### Templating
- **Watch mode for `get`** — `-w/--watch` that re-renders the custom template as the informer stream fires, similar to `kubectl get -w`. Would need incremental row updates rather than full redraw.
- **JSON/YAML/CSV output** — `-o json|yaml|csv` after template evaluation so users can pipe cwide-computed columns into `jq`, spreadsheets, or dashboards without re-implementing the JSONPath expressions.
- **Sort/filter flags** — `--sort-by=<column>` and `--filter='column=value'` operating on rendered rows, so users don't need to write awk/grep pipelines.
- **Column selection at call time** — `-c NAME,STATUS,AGE` to project a subset of columns from a template without editing the file.
- **Template inheritance / includes** — `{{ include "common.podHealth" . }}` across templates so shared snippets aren't duplicated in every resource file. Right now `text/template` `define` only works within a single file.
- **Template validation** — `kubectl cwide template lint <file>` that checks JSONPath expressions against the resource's OpenAPI schema and reports unknown fields before runtime.
- **Namespaced defaults** — per-namespace or per-context default template selection, driven by `~/.kubectl-cwide/config.yaml` (right now `default.tpl` is global).
- **More built-in template functions** — `humanBytes`, `age` (already-formatted duration), `colorIf`, `truncate`, `json`, `b64dec` for secrets. `probeCheck` proves the pattern works; extend it.
- **Colorized output** — ANSI colors bound to condition/status columns (Ready green, NotReady red, Pending yellow), with `--no-color` and `NO_COLOR` env var support.

### Marketplace / sharing
- **Template versioning** — pin a marketplace template to a version and record it in `~/.kubectl-cwide/config.yaml` so `sync` won't silently upgrade.
- **Signed templates** — cosign/sigstore signatures on marketplace entries; `marketplace install` verifies before writing to disk.
- **Private registries** — allow non-GitHub sources (OCI registries via ORAS, S3 buckets, GitLab). The marketplace API today assumes GitHub.
- **`marketplace publish`** — one-command flow to open a PR against the community index repo from a local template directory.

### Discovery & UX
- **Interactive TUI** — `kubectl cwide tui` with fuzzy resource search, live-rendered custom columns, and drill-down into `tree`. Bubbletea-based.
- **Shell completion** — `completion bash|zsh|fish|powershell` producing scripts that complete resource kinds, template names, aliases, and marketplace entries.
- **`explain` integration** — `kubectl cwide template scaffold <resource>` that reads `kubectl explain --recursive` and pre-populates a template with every leaf field commented out, so users can uncomment what they want.
- **`diff` command** — `kubectl cwide diff <resource>` comparing two revisions of the same object (via `resourceVersion` history from audit log or `kubectl.kubernetes.io/last-applied-configuration`) using the current template columns.

### Tree
- **Cyclic reference detection** — protect the tree walk from ownerRef loops (rare, but happens with broken controllers) with a visited set + `--max-depth`.
- **Custom edge types** — user-defined "follow this label to that kind" edges in a config file, extending the built-in owner/selector/field rules.
- **`--reverse`** — show ancestors of a resource, not just descendants.

### Aliases
- **Alias groups** — `kubectl cwide alias set core "pod,svc,deploy,cm"` for one-shot multi-kind gets.
- **Cluster-scoped aliases via ConfigMap** — same sync mechanism as templates so a team can share `vw` and friends.

## Bug fixes / hardening

- **`context.TODO()` everywhere** — `pkg/cmd/configmap/push.go`, `pkg/cmd/configmap/sync.go`, `pkg/cmd/initialization/init.go`. Thread `cmd.Context()` through so callers can cancel and set timeouts (relevant for `sync` in CI).
- **`get all-resources` failure modes** — one failing API group currently can taint the whole listing (or not, depending on error handling). Audit: partial results should print with a warning, not abort.
- **Discovery cache** — `init` and `get all-resources` re-hit the discovery endpoint on every invocation. Reuse `~/.kube/cache/discovery` like kubectl does.
- **Template edit race** — `template edit` opens `$EDITOR` and rewrites the file on save. If the user edits while `configmap sync` runs, one silently overwrites the other. Add an fs lock or at least a mtime check on save.
- **Krew manifest name typo** — README line 24 says `kubectl-ciwe` (should be `cwide`). Small, but the code block is what users copy.
- **`get -A` semantics** — verify all-namespaces flag is plumbed through consistently across `get`, `tree`, and `get all-resources`.
- **Nil-pointer risk in JSONPath filters** — expressions like `.status.containerStatuses[0].restartCount` panic-adjacent when `containerStatuses` is empty (pending pod). Add a safe-navigation helper `{{ safeIndex . "status" "containerStatuses" 0 "restartCount" }}` and document it as the recommended pattern.
- **Windows path handling** — template paths are joined with `/` in a few places. Sanity-check on Windows or drop it from the support matrix explicitly.
- **`--kubeconfig` at persistent-flag level but not always read** — some subcommands construct their own client factory and ignore the flag. Route everything through a shared `genericclioptions.ConfigFlags`.
- **Test coverage gaps** — `pkg/cmd/configmap/`, `pkg/cmd/marketplace/`, and `pkg/cmd/template/edit.go` have no `_test.go`. `get`, `tree`, `list`, `alias` do. Marketplace especially needs mocked HTTP tests.
- **Goreleaser + krew index** — verify the release just cut (v0.7.0) actually reaches the krew index; the previous automation PRs (#4, #5, #7) suggest this has been flaky.

## Refactors / infra

- **Adopt `cmd.Context()`** — see above; also lets us wire up signal handling once at root and get Ctrl-C mid-render for free.
- **Extract `client-go` factory** — every command re-constructs REST config, discovery client, dynamic client. One `pkg/clients` package would remove ~200 lines of duplication.
- **Vendor dir vs go modules** — repo vendors deps *and* has `go.sum`. Pick one. Vendoring made sense pre-modules; today it doubles review noise on dep bumps.
- **CI matrix** — GitHub Actions currently (check `.github/`) may only run one Go version / OS. Add 1.22 + 1.23 × linux/mac/windows.
- **E2E harness** — `15a05d7` added e2e tests. Wire them into CI against a kind cluster on PRs, not just locally.

## Docs

- **Cookbook page** — one-liners for common asks: "show pod restart reason", "show pvc bound-to node", "show hpa current vs desired". Each with the template file that produces it.
- **Migration guide from `kubectl-custom-cols`** — the project cwide was inspired by. Show side-by-side syntax so users can move.
- **`README` restructure** — README is currently ~450 lines linear. Split into `docs/` with Getting Started, Templates, Marketplace, Tree, Aliases, and Reference sections.
