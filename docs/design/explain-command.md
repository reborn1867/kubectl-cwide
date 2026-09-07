# Design: `cwide explain` (roadmap v0.11.0 #9)

Status: **design** (implementation pending). This spec is grounded in the
current code so the eventual PR is mostly mechanical. No behavior exists yet.

## Goal

`kubectl cwide explain <template>` and `kubectl cwide explain <alias>` answer
"where did this come from, what does it read, and what does it touch?" without
the user opening the file or `config.yaml` by hand. It's a read-only
introspection command — no cluster calls in the base version.

## Command shape

```
kubectl cwide explain TEMPLATE [-r RESOURCE]     # explain a template
kubectl cwide explain ALIAS                       # explain an alias
```

Disambiguation: if the token matches a configured alias name (present in
`Config.Aliases` or `Config.AliasEntries`), explain it as an alias; otherwise
treat it as a template name and resolve it under the template root (requires
`-r`/`--resource` to locate the `<kind>-<group>-<version>` dir, mirroring
`template diff`/`template edit`). If both interpretations are possible, prefer
alias and print a one-line hint that a same-named template also exists.

## Template explanation

Resolve the file with the existing `.yaml`-then-`.tpl` order (reuse
`loadInstalledTemplate` from `pkg/cmd/template/diff.go`, or factor it into a
shared helper). Then report, all from data already in `models.YAMLTemplate`:

- **Provenance** — from `Metadata *TemplateMetadata`: `source`, `sourceRepo`,
  `sourceRef`, `version`, `installedAt`. If `Metadata == nil`, print
  "user-authored / legacy (no metadata block)".
- **Columns** — one line per `YAMLColumn`: `HEADER  <kind>  <spec>` where kind
  is `jsonpath` (has `FieldSpec`), `template` (has `Template`), or
  `default-printer` (FieldSpec == `$_defaultPrinterField`).
- **Fields read** — the set of JSONPath roots pulled from each `FieldSpec`
  (e.g. `.status.phase` → `status`). Best-effort static extraction; label it
  "approximate" for `template:` columns since Go-template field access can't
  be fully enumerated statically.
- **Functions used** — scan `Template` fields and the `Helpers`/`Funcs` blocks
  for known function names (`probeCheck`, `lookup`, `age`, `humanBytes`,
  `colorIf`, `truncate`, `b64dec`, `safeIndex`). Flag `probeCheck`/`lookup`
  specially: "performs live API calls per row."
- **`.tpl` templates** — no `Metadata`/structured columns; report the header
  line, the field line, and any `{{ define }}` helper names found.

## Alias explanation

From `models.Config` (already loaded via `utils.LoadConfig`):

- **Target** — `ResolveAliasTarget(name)`; note if it's a comma-separated group
  and list the kinds.
- **Source format** — whether it came from `AliasEntries` (rich) or the legacy
  `Aliases` map (`AliasEntries` wins when both present).
- **Template bindings** — from `AliasEntry`: `Template` (applies to all kinds)
  and `Templates` (per-kind overrides). Render as `kind → template`.
- **Group + TYPE/NAME caveat** — if the target is a group, restate the
  documented limitation that group aliases don't combine with `TYPE/NAME`
  (see README alias-groups note).

## Output

Human-readable sections by default. Add `-o yaml|json` for a structured dump
(reuse the existing output-format plumbing style in `pkg/cmd/get/output.go`;
this command's payload is small and static so a plain `yaml.Marshal` of a
result struct is enough — no need for the row pipeline).

## Reuse map (what already exists)

| Need | Existing code |
|------|---------------|
| locate template file | `loadInstalledTemplate` (`template/diff.go`) |
| resolve template root | `utils.ResolveTemplatePath` |
| parse YAML template | `models.YAMLTemplate` + existing YAML load path |
| alias target/bindings | `Config.ResolveAliasTarget`, `ResolveAliasTemplate` |
| known function names | `pkg/parser/funcs` registry |
| default-printer sentinel | `common.DefaultPrinterField` |

## Non-goals (defer)

- Live schema/field validation against a CRD — that's roadmap #13
  (`template lint --against-crd`), a separate cluster-connected command.
- Rendering a sample object — that overlaps #12 (`template diff --against-live`).

## Test plan

Pure unit tests, no cluster:

- template with full `Metadata` → provenance section reflects each field.
- template with no metadata → "user-authored / legacy".
- columns of each kind (jsonpath / template / default-printer) classified right.
- function detection finds `probeCheck`/`lookup` and flags the live-call warning.
- alias from `AliasEntries` vs legacy `Aliases`; group alias lists kinds.
- `-o json` shape round-trips.
