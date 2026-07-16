---
name: template-helper
description: Generate a kubectl-cwide column template (.yaml or .tpl) for a Kubernetes resource kind from a natural-language description of the columns the user wants. Use when the user asks you to "write a cwide template", "generate a template for <kind>", "show me X and Y as columns", or invokes /template-helper. Produces a single file the user can drop into ~/.kubectl-cwide/templates/<kind-group-version>/<name>.yaml.
---

# kubectl-cwide template helper

You are generating a **column template file** for `kubectl-cwide`, a kubectl plugin that renders resources as tables using per-column expressions.

Your job:

1. Ask the user which **kind** they want (Pod, Deployment, PVC, custom CR, etc.) and which **columns** they want if they haven't already said. Don't invent columns — every column must correspond to a field they asked for.
2. Choose the file format (**YAML** by default; `.tpl` only when the user explicitly asks or when the columns are trivially JSONPath-only).
3. Pick the right expression type per column — **`fieldSpec` (JSONPath)** for a direct field, **`template` (Go text/template)** for anything computed. Never both on one column.
4. Emit exactly one file, with a target path comment on the first line. Do not narrate — just produce the file.

## Capabilities you can rely on

### Expression types

| Field | When to use | Example |
|---|---|---|
| `fieldSpec` | A single JSONPath into the resource | `.metadata.name`, `.status.phase`, `.spec.containers[0].image` |
| `template` | Anything with logic (conditionals, iteration, arithmetic, formatting, cross-resource lookups) | `{{ range .status.containerStatuses }}...{{ end }}` |

JSONPath dialect: cwide accepts `.metadata.name` (bare) and `{.metadata.name}` (kubectl-style) — both work. Filter expressions like `.status.conditions[?(@.type=="Ready")].status` are supported.

### Built-in template functions (always available)

**Formatting / strings:**
- `humanBytes v` — int/float/numeric-string → `KiB`/`MiB`/`GiB`/…
- `age v` — RFC3339 timestamp → `3d`, `5h12m`
- `truncate n s` — cut string at N runes, append `…`
- `b64dec s` — base64-decode (returns input on decode error). Useful for Secret data.
- `colorIf cond color text` — wrap text in ANSI color when cond truthy. Colors: `red`, `green`, `yellow`, `blue`, `cyan`, `magenta`, `gray`. Respects `NO_COLOR` and `--no-color`.
- `safeIndex root keys...` — walk nested map/slice by keys/indices, `""` at any missing level instead of panicking.

**Data conversion (from Helm's set):**
- `toYaml`, `toYamlPretty`, `fromYaml`, `fromYamlArray`
- `toJson`, `fromJson`, `fromJsonArray`
- `toToml`, `fromToml`

**Cluster lookups (live API calls per row — use sparingly):**
- `lookup apiVersion kind namespace name` — fetch a single object. Returns `nil` if missing.
- `lookupByLabel apiVersion kind namespace label=value` — fetch a list matched by label selector.
- `probeCheck . probeType` — live-fire a pod's readiness/liveness/startup probe (`readiness`/`liveness`/`startup`). Returns `OK (200)`, `FAIL (...)`, `N/A`, `N/A (exec)`, `N/A (grpc)`.

**Helm-style pipeline helpers (partial list):** `add`, `sub`, `mul`, `div`, `mod`, `default`, `eq`, `ne`, `lt`, `gt`, `le`, `ge`, `and`, `or`, `not`, `printf`, `trim`, `upper`, `lower`, `title`, `contains`, `hasPrefix`, `hasSuffix`, `replace`, `regexMatch`, `int`, `float64`, `len`.

**Progress meter (interactive only):**
- `progressBar current total` — a small ASCII bar. Only useful for one-off status columns.

## Restrictions — read these carefully

1. **One expression kind per column.** Never set both `fieldSpec` and `template`. If you need Go template logic anywhere, use `template`.
2. **No file/exec/env/shell.** The template runs in-process; there is no way to read arbitrary files, run shell commands, or read environment variables from within it.
3. **`lookup` and `probeCheck` run one API call per row** and can hit rate limits. Warn the user if a column uses them across a large list.
4. **Column headers must be ALL_CAPS** by convention (`NAME`, `AGE`, `RESTARTS`) — this matches `kubectl get` and makes `-c NAME,AGE` selection intuitive.
5. **Multi-line output within a single cell is preserved.** If a template emits `\n` the row is expanded into multiple visual lines aligned under the same header.
6. **No `helpers:` block on `.tpl` templates.** Only YAML templates have a top-level `helpers:` field for shared `{{ define }}` blocks. On `.tpl` templates, put `{{ define }}` inline at the bottom.
7. **The `funcs:` map in YAML templates is two-pass**: each entry is parsed as a body, then invoked as if it were a template function taking `.` (or the pipeline args). Bodies see the top-level `helpers:` too.
8. **Filesystem layout is derived from GVK, lowercased:** `<plural>-<group>-<version>` — e.g. `pod--v1` (empty group), `deployment-apps-v1`, `certificate-cert-manager.io-v1`. Empty group leaves a doubled dash. Users can list templates with `kubectl cwide template list -r <resource>`.
9. **Only YAML templates support cross-file helpers** loaded from `<template-root>/_shared/*.tpl`. Emit standalone files unless the user asks for a shared helper.

## Output format

Emit a single fenced code block containing the file body. Prefix with a comment line that shows the exact path where it should live, so the user can copy-paste. Example:

    ```yaml
    # ~/.kubectl-cwide/templates/pod--v1/debug.yaml
    columns:
      - header: NAME
        fieldSpec: .metadata.name
    ```

Do not repeat the template content in prose after the block. Do not add explanatory text unless the user's request was ambiguous and you're pointing out a choice you made.

## Worked examples

Load `references/examples.md` for a set of before/after mappings from user requests to templates — including simple JSONPath columns, `text/template` bodies, `funcs:` blocks, cross-resource `lookup`, and `probeCheck` for live health checks. Consult it when the request touches an unfamiliar pattern.

## Workflow

1. If the user's request is missing the kind, ask.
2. If the request asks for something impossible (a computed value that needs data outside the resource + not covered by `lookup`), say so; don't invent a workaround.
3. Draft the template. Prefer `fieldSpec` when a single JSONPath suffices — it renders faster and is easier for the user to edit later.
4. Emit the file (see Output format above).
5. On a fresh line after the code block, print: `Save to: <path>` and a one-line usage hint (`kubectl cwide get <kind> -t <template-name>`).
