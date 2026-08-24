# Next Features Plan — Template Ecosystem

Scope: next 1–2 releases. Focus: making cwide templates easier to **discover, share, verify, and evolve**. All items are sized to land in weeks (no design docs required).

## Guiding constraints

- Every item ships as a single reviewable PR.
- No breaking changes to existing template files — v0.8.1 templates must keep parsing.
- Cluster-agnostic where possible: features should work against `kind` in CI.
- Prefer extending existing subcommands over adding new top-level commands.

## Items

### 1. `template export` / `template import` — round-trip a template directory

**Motivation.** Users currently ferry templates between machines via `configmap push/sync` (needs a cluster) or manual `scp` (no bundling). A local-only `tar+yaml` round trip covers the "share with a colleague on Slack" and "check into git" cases.

**Design.**
- `kubectl cwide template export [--out FILE] [-r RESOURCE]` — walks the template root, emits a single `.cwide-bundle.tar.gz` (or stdout when no `--out`). Includes a `manifest.yaml` at the root with format version, timestamp, list of resource dirs, and optional user description.
- `kubectl cwide template import FILE [--force] [--only RESOURCE,…]` — extracts into the local template root. Respects `templateSources` priority just like `configmap sync`. `--force` overrides.
- Bundle format is intentionally boring: a tar of the on-disk YAML/TPL files plus a `manifest.yaml` with SHA256 of each entry.

**Effort.** ~200 LOC + tests. One PR.

### 2. `template diff` — compare local vs marketplace / vs ConfigMap

**Motivation.** After a `configmap sync` or `marketplace install --ref v1.2.0`, users don't know what changed. A diff command surfaces column additions/removals and helper-block edits.

**Design.**
- `kubectl cwide template diff -r RESOURCE -t NAME --against configmap|marketplace [--ref REF]`
- Parses both sides as `YAMLTemplate`, then does a semantic diff: added columns, removed columns, header renames, JSONPath changes, helper-block textual diff.
- Output is colorized (respects `--no-color` / `NO_COLOR`).

**Effort.** ~300 LOC. Reuses existing marketplace / configmap fetchers. One PR.

### 3. Marketplace: categories and search-by-tag

**Motivation.** The current marketplace is one flat namespace of resource directories. As the community index grows, users need to filter (`ops`, `security`, `finops`, `debug`).

**Design.**
- Extend the index repo's optional `manifest.yaml` per resource dir with `categories: [ops, debug]` and `tags: [pod, ready, restart]`.
- `kubectl cwide marketplace list --category ops --tag pod`
- `kubectl cwide marketplace search "restart reason"` already exists — extend to also match on `tags`.
- Backward compatible: manifests without categories/tags still show up under `--category=""`.

**Effort.** ~150 LOC in `pkg/cmd/marketplace/`. One PR + a follow-up PR to the community index adding categories to existing templates.

### 4. `template validate --against` — run lint plus a live-cluster schema check

**Motivation.** `template lint` (shipped in v0.8.0) only checks JSONPath syntax. It can't tell you a field doesn't exist on the actual CRD. A cluster-connected mode catches typos like `.status.readReplicas` (missing `y`).

**Design.**
- `kubectl cwide template validate FILE --against <resource>` — connect to the cluster, fetch the OpenAPI v3 schema for the resource, walk every `fieldSpec` and verify each path segment exists.
- Falls back to `template lint` behavior (no cluster contact) when `--against` is omitted.
- Reports unknown paths with a suggestion (Levenshtein-nearest known field).

**Effort.** ~250 LOC. Uses `factory.OpenAPIV3Client()`. One PR.

### 5. Sample library: `kubectl cwide template scaffold --from <example>`

**Motivation.** The v0.8.0 `template scaffold <resource>` emits a bare stub. Users want to say "give me the *cookbook.md pod restart reason recipe*" without opening a browser.

**Design.**
- Bundle the recipes from `docs/cookbook.md` as first-class scaffold sources.
- `kubectl cwide template scaffold pod` — existing behavior.
- `kubectl cwide template scaffold pod --from restart-reason` — emits the cookbook's restart-reason template.
- `kubectl cwide template scaffold --list` — enumerates all bundled examples across all kinds.
- The examples are compiled in via `embed.FS` — no network, no docs-out-of-sync risk.

**Effort.** ~200 LOC + moving cookbook recipes into `assets/scaffolds/`. One PR.

### 6. `template version` — stamp a version into each template file

**Motivation.** Templates evolve. Right now there's no way to tell whether the `pod--v1/default.yaml` on disk matches the marketplace `v1.2.0` or was hand-edited.

**Design.**
- Add an optional top-level `metadata:` block to `YAMLTemplate`:
  ```yaml
  metadata:
    version: 1.2.0
    source: marketplace
    sourceRepo: reborn1867/kubectl-cwide-templates
    sourceRef: v1.2.0
    installedAt: 2026-07-16T12:00:00Z
  columns: [...]
  ```
- `marketplace install --ref v1.2.0` populates it automatically.
- `template list -r pod` shows a `VERSION` column pulled from this block.
- Absence is fine — no version metadata means user-authored/legacy.

**Effort.** ~150 LOC. One PR. Coordinates with #2 (diff surfaces the version delta).

### 7. `template lint --recursive` — lint an entire tree

**Motivation.** Users copy-pasting the "find … -exec lint" one-liner from the README hit the SIGPIPE / xargs mismatch on macOS.

**Design.**
- `kubectl cwide template lint --recursive [PATH]` — walks the given path (default template root) and lints every `*.yaml`/`*.tpl`.
- Aggregates results: prints `OK` count and each failure with the path.
- Exit code 1 if any file fails.

**Effort.** ~50 LOC. Tiny PR.

### 8. Marketplace: `install --all -r <resource>`

**Motivation.** Onboarding a new dev today: `init` gets a default per kind, but the community-shared debug/prod/compact variants have to be installed one by one. `--all` for a resource pulls every variant.

**Design.**
- `kubectl cwide marketplace install -r pod --all` — installs every template file the marketplace has under `templates/pod-*/`.
- `--all --force` for reinstall.
- Prints a summary: `Installed 4 templates for pod (debug, prod-audit, compact, ready)`.

**Effort.** ~80 LOC. One PR.

## Release cadence

- **v0.9.0**: items #1, #4, #6, #7 — the local/authoring story.
- **v0.10.0**: items #2, #3, #5, #8 — the marketplace/sharing story.

Both releases are non-breaking. The `metadata:` block from #6 is additive; existing templates without it continue to work.

## Explicitly deferred

- **Template signing / cosign integration** — needs a trust model discussion first (who signs? key rotation? verification defaults on/off?). Not ready for a first PR.
- **Template registry beyond GitHub (OCI, S3)** — one PR per source; needs a scope decision.
- **`marketplace publish`** — blocked on the community index repo formalizing its contribution process.
- **Template sandbox with resource limits** — `probeCheck`/`lookup` cost is real; adding per-column timeouts and per-run API budgets is worth doing but needs its own design pass.
- **WASM columns / custom template function plugins** — extensibility bet, moonshot territory. Out of scope for the next 1–2 releases.

## Working notes

- The `template-helper` skill (`.claude/skills/template-helper/`) should be updated alongside item #6 so the LLM knows about the `metadata:` block.
- Every item that mutates on-disk files should re-use `utils.ResolveTemplatePath` for consistent path resolution.
- Items #3/#5/#6 each imply small tweaks to the community index repo — do those in a follow-up PR against that repo, not this one.
