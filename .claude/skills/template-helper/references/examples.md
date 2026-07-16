# template-helper — examples

Before/after mappings from user requests to templates. Each shows the file that should be emitted for a given prompt.

## 1. Simple JSONPath columns

**User:** *Show me pods with name, status, restarts, and age.*

```yaml
# ~/.kubectl-cwide/templates/pod--v1/basic.yaml
columns:
  - header: NAME
    fieldSpec: .metadata.name
  - header: STATUS
    fieldSpec: .status.phase
  - header: RESTARTS
    fieldSpec: .status.containerStatuses[0].restartCount
  - header: AGE
    fieldSpec: .metadata.creationTimestamp
```

Notes on choices:
- `.status.containerStatuses[0].restartCount` picks the first container. If the user has multi-container pods and asks for total restarts, use a `template:` field with `{{ range .status.containerStatuses }}` and sum via `add`.
- `AGE` is a raw timestamp because cwide's tabwriter already handles duration formatting for canonical `.metadata.creationTimestamp` reads. If the user wants explicit `3d5h` style, use `{{ age .metadata.creationTimestamp }}` in a template column.

## 2. Ready-count for multi-container pods

**User:** *I want pods where READY shows `<ready>/<total>` like `kubectl get pod`.*

```yaml
# ~/.kubectl-cwide/templates/pod--v1/ready.yaml
columns:
  - header: NAME
    fieldSpec: .metadata.name
  - header: READY
    template: |
      {{- $r := 0 -}}{{- $t := 0 -}}
      {{- range .status.containerStatuses -}}
        {{- $t = add 1 $t -}}
        {{- if .ready -}}{{- $r = add 1 $r -}}{{- end -}}
      {{- end -}}
      {{ $r }}/{{ $t }}
  - header: STATUS
    fieldSpec: .status.phase
  - header: AGE
    fieldSpec: .metadata.creationTimestamp
```

## 3. Colorized status

**User:** *Same as the previous, but color the STATUS column green if Running, red otherwise.*

```yaml
# ~/.kubectl-cwide/templates/pod--v1/colored.yaml
columns:
  - header: NAME
    fieldSpec: .metadata.name
  - header: STATUS
    template: >-
      {{- $s := .status.phase -}}
      {{- if eq $s "Running" -}}
        {{ colorIf true "green" $s }}
      {{- else -}}
        {{ colorIf true "red" $s }}
      {{- end -}}
  - header: AGE
    fieldSpec: .metadata.creationTimestamp
```

`colorIf` respects `NO_COLOR` and `--no-color` automatically, so this template stays clean in log capture / CI output.

## 4. Secret data preview with base64 decode

**User:** *Show me secrets with a truncated preview of their tls.crt.*

```yaml
# ~/.kubectl-cwide/templates/secret--v1/tls-preview.yaml
columns:
  - header: NAME
    fieldSpec: .metadata.name
  - header: TYPE
    fieldSpec: .type
  - header: TLS_CRT_PREVIEW
    template: '{{ truncate 30 (b64dec (safeIndex . "data" "tls.crt")) }}'
  - header: AGE
    fieldSpec: .metadata.creationTimestamp
```

Uses `safeIndex` because not every Secret has `data.tls.crt` — otherwise a lookup on a Secret without that key would break rendering.

## 5. Reusable helper via top-level `helpers:` and `funcs:`

**User:** *I want to call the ready count as a function and reuse it in multiple templates.*

```yaml
# ~/.kubectl-cwide/templates/pod--v1/functional.yaml
columns:
  - header: NAME
    fieldSpec: .metadata.name
  - header: READY
    template: '{{ template "PodReady" . }}'
  - header: RESTARTS
    template: '{{ podRestarts . }}'
  - header: STATUS
    fieldSpec: .status.phase

helpers: |
  {{- define "PodReady" -}}
    {{- $r := 0 -}}{{- $t := 0 -}}
    {{- range .status.containerStatuses -}}
      {{- $t = add 1 $t -}}
      {{- if .ready -}}{{- $r = add 1 $r -}}{{- end -}}
    {{- end -}}
    {{ $r }}/{{ $t }}
  {{- end -}}

funcs:
  podRestarts: >-
    {{- $n := 0 -}}
    {{- range .status.containerStatuses -}}
      {{- $n = add $n .restartCount -}}
    {{- end -}}
    {{ $n }}
```

Two ways to reuse a snippet: `helpers:` blocks are called with `{{ template "Name" . }}`, and `funcs:` entries become invokable like `{{ podRestarts . }}`. Prefer `funcs:` when the caller looks cleaner in the column expression.

## 6. Cross-resource lookup: PVC → bound PV → node

**User:** *For each PVC, show me its bound PV and the node hosting it.*

```yaml
# ~/.kubectl-cwide/templates/persistentvolumeclaim--v1/bound-node.yaml
columns:
  - header: NAME
    fieldSpec: .metadata.name
  - header: STATUS
    fieldSpec: .status.phase
  - header: STORAGE
    fieldSpec: .spec.resources.requests.storage
  - header: VOLUME
    fieldSpec: .spec.volumeName
  - header: NODE
    template: |
      {{- $pv := lookup "v1" "PersistentVolume" "" .spec.volumeName -}}
      {{- if $pv -}}
        {{- range $pv.spec.nodeAffinity.required.nodeSelectorTerms -}}
          {{- range .matchExpressions -}}
            {{- if eq .key "kubernetes.io/hostname" -}}{{ index .values 0 }}{{- end -}}
          {{- end -}}
        {{- end -}}
      {{- end -}}
```

Warn the user: each rendered row makes a live API call. Fine for `kubectl cwide get pvc` in a single namespace; slow if piped through `--all-namespaces` on hundreds of PVCs.

## 7. Label-selector-based lookup: Service → pod count

**User:** *Table of services with the number of pods each one selects.*

```yaml
# ~/.kubectl-cwide/templates/service--v1/pod-count.yaml
columns:
  - header: NAME
    fieldSpec: .metadata.name
  - header: TYPE
    fieldSpec: .spec.type
  - header: CLUSTER_IP
    fieldSpec: .spec.clusterIP
  - header: PODS
    template: |
      {{- $selector := "" -}}
      {{- range $k, $v := .spec.selector -}}
        {{- if $selector -}}{{- $selector = printf "%s,%s=%s" $selector $k $v -}}
        {{- else -}}{{- $selector = printf "%s=%s" $k $v -}}{{- end -}}
      {{- end -}}
      {{- $pods := lookupByLabel "v1" "Pod" .metadata.namespace $selector -}}
      {{- if $pods -}}{{ len $pods.items }}{{- else -}}0{{- end -}}
```

## 8. Live probe health check

**User:** *Show me pods with their readiness and liveness probe status live.*

```yaml
# ~/.kubectl-cwide/templates/pod--v1/probes.yaml
columns:
  - header: NAME
    fieldSpec: .metadata.name
  - header: READINESS
    template: '{{ probeCheck . "readiness" }}'
  - header: LIVENESS
    template: '{{ probeCheck . "liveness" }}'
  - header: STARTUP
    template: '{{ probeCheck . "startup" }}'
```

Each cell fires a real request through the API server's pod proxy. Only useful in interactive debugging — do not run under `--watch`.

## 9. Human-friendly bytes

**User:** *PVCs but show their capacity in KiB/MiB/GiB instead of raw bytes.*

```yaml
# ~/.kubectl-cwide/templates/persistentvolumeclaim--v1/human.yaml
columns:
  - header: NAME
    fieldSpec: .metadata.name
  - header: STATUS
    fieldSpec: .status.phase
  - header: CAPACITY
    template: '{{ humanBytes .status.capacity.storage }}'
```

Note: `.status.capacity.storage` is a Kubernetes resource-quantity string like `1Gi`. `humanBytes` accepts numeric input — for quantity strings you generally want to strip the suffix first or leave it as raw JSONPath. Suggest `.status.capacity.storage` as `fieldSpec` when the user is happy with `1Gi` etc.; use `humanBytes` when the value is a raw byte count (Node capacity fields, ephemeral-storage limits).

## 10. `.tpl` format (legacy / minimal)

**User:** *Give me the same as example 1 but in `.tpl` format for compactness.*

```
# ~/.kubectl-cwide/templates/pod--v1/basic.tpl
NAME           STATUS       RESTARTS                                          AGE
.metadata.name .status.phase .status.containerStatuses[0].restartCount .metadata.creationTimestamp
```

Line 1: whitespace-separated headers.
Line 2: whitespace-separated JSONPath expressions, positionally matching the headers.

`.tpl` templates support `{{ define }}` at the bottom of the file for helper snippets — but no top-level `helpers:` block. Prefer YAML for anything beyond flat JSONPath.

## Anti-patterns to avoid

- **Do NOT** put both `fieldSpec` and `template` on the same column. The parser silently prefers one; users won't know which they got.
- **Do NOT** wrap `fieldSpec` values in `{{ }}` — that's for templates.
- **Do NOT** invent columns the user didn't ask for. If they say "give me pods with name and status", produce two columns, not five.
- **Do NOT** use `range` without wrapping delimiters `{{- ... -}}` when concatenating — otherwise you get stray whitespace across rows.
- **Do NOT** call `lookup` inside another `range` unless you've warned the user about API cost (N×M requests).
- **Do NOT** invent template functions. Only the ones listed in the SKILL are available. `sprig`-style helpers (like `date`, `dict`, `list`) are NOT loaded.
