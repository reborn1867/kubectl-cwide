# kubectl-cwide roadmap

Living plan for user-facing features and infra work, ordered so each item
unblocks or amplifies the next.

Legend

- Status: `todo` / `in-progress` / `blocked` / `done`
- Effort: rough day-band, `S` (< 1d), `M` (1–3d), `L` (> 3d)
- Value tier: T1 (high-leverage), T2 (clear value), T3 (polish), T4 (infra)

The autonomous evolution loop pulls the next `todo` item at each iteration.
Reordering here reorders the loop.

## Milestone: v0.9.4 — safety and quality-of-life

| # | Item | Tier | Effort | Status | Notes |
|---|------|------|--------|--------|-------|
| 1 | `template diff --against-file / --against-recipe` | T1 | M | in-progress | branch `feat/template-diff`. Local unified diff, no cluster contact. |
| 2 | Land in-flight PRs (#31–34) | T4 | S | in-progress | scaffold-examples, metadata, lint-recursive, alias-conflict |
| 3 | Cut v0.9.4 covering merged batch | T4 | S | todo | goreleaser tag after PR sweep lands |

## Milestone: v0.10.0 — power-user get

| # | Item | Tier | Effort | Status | Notes |
|---|------|------|--------|--------|-------|
| 4 | `get --sort <col>` | T1 | M | todo | Reuses printer column data; stable Go sort keyed by rendered cell. |
| 5 | `get --filter '<col><op><val>'` | T1 | M | todo | Small expr grammar over rendered cells. `==`, `!=`, `~=` (regex), numeric compare. |
| 6 | `get -o json\|yaml\|csv` (template-driven) | T1 | M | todo | Same projection as the table, machine-readable. Blocks scripts. |
| 7 | `get -w` with delta highlight | T1 | M | todo | ANSI color on cells that changed since previous tick. |
| 8 | `--label-columns=...` / `--show-labels` | T2 | S | todo | kubectl parity; no template edit needed. |

## Milestone: v0.11.0 — introspection and composition

| # | Item | Tier | Effort | Status | Notes |
|---|------|------|--------|--------|-------|
| 9 | `explain <template>` / `explain <alias>` | T1 | M | todo | Where it came from, fields it reads, resources it touches. |
| 10 | `get --explain-empty <col>` | T3 | M | todo | Print the JSONPath tried + keys actually present. Kills "why blank" tickets. |
| 11 | `--merge-templates a,b` column composition | T2 | L | todo | Layered rendering — base + diagnostic overlay. Design: precedence rules for duplicate headers. |
| 12 | `template diff --against-live <r/n>` | T2 | L | todo | Render one object with two templates side by side. |
| 13 | `template lint --against-crd <kind>` | T3 | M | todo | Fetch OpenAPI, verify `fieldSpec` paths resolve. Requires cluster. |

## Milestone: v0.12.0 — team surface

| # | Item | Tier | Effort | Status | Notes |
|---|------|------|--------|--------|-------|
| 14 | `marketplace install --all` | T2 | S | todo | Idempotent bulk install; respects existing `metadata.version`. |
| 15 | `marketplace update` | T2 | M | todo | Delta-refresh installed templates by provenance metadata. |
| 16 | `configmap push --aliases` | T2 | M | todo | Team-shared alias/template config via existing ConfigMap channel. |
| 17 | `kc cwide top` | T2 | L | todo | Metrics-server-backed usage columns, composed into any template. |

## Milestone: v0.13.0 — polish

| # | Item | Tier | Effort | Status | Notes |
|---|------|------|--------|--------|-------|
| 18 | `kc cwide doctor` | T3 | S | todo | Self-check for template path, shared helpers, kubeconfig, installed template lint. |
| 19 | `--output-errors=json` | T3 | S | todo | Machine-readable failure envelope for scripts. |
| 20 | Interactive `template edit --tui` | T3 | L | todo | bubbletea live-preview against a sample manifest. |

## Milestone: v0.14.0 — infrastructure hardening

| # | Item | Tier | Effort | Status | Notes |
|---|------|------|--------|--------|-------|
| 21 | Renderer benchmark suite | T4 | S | todo | `go test -bench` for the printer over a synthetic 10k-object list. |
| 22 | Fuzz JSONPath + text/template path | T4 | M | todo | `go test -fuzz`; guards the highest-blast-radius surface. |
| 23 | Krew-index PR from release workflow | T4 | S | todo | Auto-open the krew-index PR when a tag is cut. |
| 24 | Coverage floor in CI | T4 | S | todo | Fail CI below 60% package coverage. |

## Working order

The loop pulls items top-to-bottom within a milestone, then across
milestones. Anything `blocked` is skipped with a comment on why. `done`
items stay for history — do not delete.

## How to steer

- **Change priority**: reorder rows here; the next loop iteration picks
  the new top item.
- **Add an item**: append to the appropriate milestone table with a fresh
  `todo` status. Include a one-line "Notes" describing the wedge.
- **Kill an item**: set status to `wontfix` with a one-line rationale;
  the loop will skip it forever.
