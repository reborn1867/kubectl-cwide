# kubectl-cwide
A `krew` plugin for customized wide output of `kubectl`.

Special thanks to [kubectl-custom-cols](https://github.com/webofmars/kubectl-custom-cols), which inspired this project.

Managing Kubernetes resources often requires printing extra columns for specific information. While `kubectl` provides JSONPath expressions to customize table output, memorizing long commands can be tedious. 

`kubectl-cwide` simplifies this process by allowing you to persist custom column formats. You can easily edit, extend, alias, or share these formats with your team members.

## Highlights
- **Template Based Parsing**: In addition to native JSONPath parsing, you can effortlessly customize your table output using the same approach as Helm templates, harnessing the flexibility and power of [text.Template](https://pkg.go.dev/text/template).
- **YAML Template Support**: Define templates in a structured YAML format for better readability and maintainability alongside the classic `.tpl` format.
- **Automatic Template Generation**: Automatically generate custom column templates for Kubernetes resources, saving time and effort.
- **Customizable Output**: Define and persist custom column formats for specific resource types with ease.
- **Editable Templates**: Modify and extend templates as needed to suit your workflow. Use `template edit` to open templates directly in your preferred editor.
- **Team Collaboration**: Share custom column templates with team members via Kubernetes ConfigMaps using `configmap push` and `configmap sync`.
- **Community Marketplace**: Browse and install community-shared templates from GitHub with `marketplace list`, `search`, and `install`.
- **Built-in Template Functions**: Use specialized functions like `probeCheck` to perform live health checks on pod probe endpoints directly in your templates.
- **Resource Tree View**: Visualize relationships between Kubernetes resources (owner references, label selectors, field references) with the `tree` command — including ancestor walks (`--reverse`), bounded depth, and automatic cycle detection.
- **Custom Resource Aliases**: Define short aliases for long resource type names (e.g. `vw` for `validatingwebhookconfigurations`) with automatic resolution in `get` and `tree` commands. Alias groups (`pod,svc,cm`) and cluster-scoped sync via ConfigMap are supported.
- **Structured & filtered output**: Project columns (`-c`), sort rows (`--sort-by`), filter with regex (`--filter`), and emit `-o json|yaml|csv`.
- **Template authoring tools**: `template lint` validates JSONPath and shape; `template scaffold` produces a starter file; `_shared/*.tpl` helpers are auto-included across every template.
- **Marketplace pinning**: `marketplace install --ref <sha|tag>` records the version in `~/.kubectl-cwide/marketplace.lock`.
- **Shell completion & ergonomics**: `completion` subcommand for bash/zsh/fish/powershell, `--no-color`/`NO_COLOR` respect, and clean Ctrl-C cancellation.

## Installation
As a [krew](https://github.com/kubernetes-sigs/krew) plugin, `kubectl-cwide` can be installed with a simple command as following once it's officially accepted.
```
kubectl krew install cwide
```

## Usage
1. **Initialize Custom Column Template**: Generate YAML template files for all discovered resources and CRDs.
   ```sh
   kubectl cwide init --template-path /tmp/cwide --kubeconfig <path-to-kubeconfig-file-with-crd-read-permission>
   ```

2. **Edit Template**: Modify the custom column template as needed in the `template-path`.

3. **View Customized Output**: Use the generated template to display resources.
   ```sh
   kubectl cwide get <resource-kind> <resource-name>
   ```

4. **List templates**: List all templates of a k8s resource. (resource name cannot be plural nor short name)
   ```
   kubectl cwide template list -r <resource-name>
   ```

   e.g.
   ```
   kubectl cwide template list -r pod
   default
   original-output
   ```

### Sample Template File (`.tpl` format)

```sh
cat /tmp/cwide/pod--v1/default.tpl

NAME    READY    RESTARTS    AGE    POD_READY_TO_START_CONTAINERS    INITIALIZED    READY CONTAINERS_READY    POD_SCHEDULED
.metadata.name .status.phase .status.containerStatuses[0].restartCount .metadata.creationTimestamp .status.conditions[?(@.type=="PodReadyToStartContainers")].status .status.conditions[?(@.type=="PodReadyToStartContainers")].status    .status.conditions[?(@.type=="PodReadyToStartContainers")].status .status.conditions[?(@.type=="PodReadyToStartContainers")].status .status.conditions[?(@.type=="PodReadyToStartContainers")].status

kubectl cwide get pod
NAME                       READY     RESTARTS   AGE     POD_READY_TO_START_CONTAINERS   INITIALIZED   READY   CONTAINERS_READY   POD_SCHEDULED
fluentd-2rnrb              Running   0          91m     True                            True          True    True               True
grafana-85cf45988b-5wttc   Running   0          4d13h   True                            True          True    True               True
grafana-85cf45988b-knmhn   Running   0          4d13h   True                            True          True    True               True
```

### Sample Template File with `text.Template` (`.tpl` format)
```sh
cat /tmp/cwide/pod--v1/original-output.tpl

NAME                                READY   STATUS    RESTARTS      AGE
.metadata.name {{ template "PodReady" . }} .status.phase {{ template "PodRestarts" . }} .metadata.creationTimestamp 

{{- define "PodReady" -}}
  {{- $ready := 0 | int  -}}
  {{- $total := 0 | int  -}}
  {{- range $idx, $status := .status.containerStatuses }}
    {{- $total = add 1 $total  -}}
    {{- if eq $status.ready true }}
      {{- $ready = add 1 $ready  -}}
    {{- end }}
  {{- end }}
  {{- printf "%d/%d" $ready $total -}}
{{- end }}

{{- define "PodRestarts" -}}
  {{- $restarts := 0 | int  -}}
  {{- range $idx, $status := .status.containerStatuses }}
    {{- $restarts = add $status.restartCount $restarts  -}}
  {{- end }}
  {{- $restarts -}}
{{- end }}

kubectl cwide get pod -t original-output
NAME                       READY   STATUS    RESTARTS   AGE
fluentd-cpg6x              1/1     Running   0          3d2h
fluentd-pr48h              1/1     Running   0          3d2h
grafana-78578fcfd5-2lhf8   2/2     Running   0          7d23h
grafana-78578fcfd5-9s7q4   2/2     Running   0          7d23h
```

We managed to make output looks almost the same as `kubectl get pod` which is not supported by custom columns output `-ocustom-columns`. By leveraging various helm template functions (and there will be more in the future), you get to freely create your own customized output.

### YAML Template Format

In addition to the classic `.tpl` format, kubectl-cwide supports a structured YAML template format. YAML templates are the default format generated by `kubectl cwide init`.

When resolving templates, kubectl-cwide tries `.yaml` first and falls back to `.tpl`, so both formats can coexist in the same template directory.

#### Basic YAML Template

```yaml
# /tmp/cwide/pod--v1/default.yaml
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

Each column entry has:
- `header` — the column header displayed in the output
- `fieldSpec` — a JSONPath expression to extract the value from the resource

#### Using Go Templates in YAML

For columns that need more complex logic, use the `template` field instead of `fieldSpec`:

```yaml
# /tmp/cwide/pod--v1/custom.yaml
columns:
  - header: NAME
    fieldSpec: .metadata.name
  - header: READY
    template: "{{ template \"PodReady\" . }}"
  - header: STATUS
    fieldSpec: .status.phase
  - header: RESTARTS
    template: "{{ template \"PodRestarts\" . }}"
  - header: AGE
    fieldSpec: .metadata.creationTimestamp
helpers: |
  {{- define "PodReady" -}}
    {{- $ready := 0 | int  -}}
    {{- $total := 0 | int  -}}
    {{- range $idx, $status := .status.containerStatuses }}
      {{- $total = add 1 $total  -}}
      {{- if eq $status.ready true }}
        {{- $ready = add 1 $ready  -}}
      {{- end }}
    {{- end }}
    {{- printf "%d/%d" $ready $total -}}
  {{- end }}
  {{- define "PodRestarts" -}}
    {{- $restarts := 0 | int  -}}
    {{- range $idx, $status := .status.containerStatuses }}
      {{- $restarts = add $status.restartCount $restarts  -}}
    {{- end }}
    {{- $restarts -}}
  {{- end }}
```

Use it with:
```sh
kubectl cwide get pod -t custom
```

#### YAML Template with Default Printer Fields

For default Kubernetes objects, the special `$_defaultPrinterField` value delegates rendering to kubectl's built-in printer:

```yaml
columns:
  - header: NAME
    fieldSpec: $_defaultPrinterField
  - header: READY
    fieldSpec: $_defaultPrinterField
  - header: STATUS
    fieldSpec: $_defaultPrinterField
  - header: AGE
    fieldSpec: $_defaultPrinterField
  - header: IMAGES
    fieldSpec: .spec.containers[*].image
```

You can freely mix default printer fields with custom JSONPath or Go template columns.

#### Managing YAML Templates

Create a new YAML template:
```sh
kubectl cwide template create -r pod -n my-template
# creates: <template-path>/pod--v1/my-template.yaml
```

List all templates (both `.yaml` and `.tpl`):
```sh
kubectl cwide template list -r pod
```

### Customization on Default Kubernetes Objects
For default k8s objects, kubectl-cwide generates a special template with mark `$_defaultPrinterField` to indicate that the column is printed by default printer of kubectl. You are free to build your customized output by appending new column, rearrange columns order or redo the whole output from scratch. 

e.g.
```
cat /tmp/cwide/pod--v1/default.tpl
NAME                  READY                 STATUS                RESTARTS              AGE                   IP                    NODE                  NOMINATED_NODE        READINESS_GATES
$_defaultPrinterField $_defaultPrinterField $_defaultPrinterField $_defaultPrinterField $_defaultPrinterField $_defaultPrinterField $_defaultPrinterField $_defaultPrinterField $_defaultPrinterField
```

Using default template for output rendering, it would look the same as kubectl get output.
```
kubectl cwide get pod
NAME                                            READY   STATUS      RESTARTS   AGE     IP       NODE                                                    NOMINATED_NODE   READINESS_GATES
fluentd-wx98t                                   1/1     Running     0          24m     <none>   shoot--di-demo--di-dmo-gcp-reg-default-z1-56f44-76dzv
fluentd-x55zk                                   1/1     Running     0          25m     <none>   shoot--di-demo--di-dmo-gcp-reg-default-z1-56f44-k7s7x
grafana-7475f448db-49zn9                        2/2     Running     0          4d23h   <none>   shoot--di-demo--di-dmo-gcp-reg-default-z3-6ffc9-99nkz
```

If you want to remove columns `NOMINATED_NODE` and `READINESS_GATES` which you don't care, and add a new column for images, the template would be modified like this:
```
NAME                  READY                 STATUS                RESTARTS              AGE                   IP                    NODE                  IMAGES
$_defaultPrinterField $_defaultPrinterField $_defaultPrinterField $_defaultPrinterField $_defaultPrinterField $_defaultPrinterField $_defaultPrinterField .spec.containers[*].image
```

And the output would be as following
```
NAME                                            READY   STATUS    RESTARTS   AGE     IP       NODE                                                    IMAGES
fluentd-wx98t                                   1/1     Running   0          37m     <none>   shoot--di-demo--di-dmo-gcp-reg-default-z1-56f44-76dzv   fluent/fluentd:v1.16
fluentd-x55zk                                   1/1     Running   0          39m     <none>   shoot--di-demo--di-dmo-gcp-reg-default-z1-56f44-k7s7x   fluent/fluentd:v1.16
grafana-7475f448db-49zn9                        2/2     Running   0          4d23h   <none>   shoot--di-demo--di-dmo-gcp-reg-default-z3-6ffc9-99nkz   grafana/grafana:11.5.4
```

### Editing Templates

Use `template edit` to open a template file directly in your preferred editor:

```sh
# Edit the default template for pods
kubectl cwide template edit -r pod

# Edit a specific named template
kubectl cwide template edit -r deployment -t minimal

# Use a custom editor
EDITOR=nano kubectl cwide template edit -r pod
```

The editor is determined by the `EDITOR` environment variable (defaults to `vi`). The command automatically resolves `.yaml` templates first, falling back to `.tpl`.

### Configuration

kubectl-cwide stores its configuration at `~/.kubectl-cwide/config.yaml`. Use the `config` command to open it in an editor:

```sh
kubectl cwide config
```

The config file supports the following fields:

```yaml
# Root directory for template files
templatePath: /tmp/cwide

# Priority order for resolving templates when using ConfigMap sync.
# "local" = local files take priority, "configmap" = ConfigMap takes priority.
templateSources:
  - local
  - configmap

# Custom short aliases for resource type names.
# Used automatically in 'get' and 'tree' commands.
aliases:
  pd: pods
  vw: validatingwebhookconfigurations
```

### ConfigMap Sync

Share templates across a team by storing them in a Kubernetes ConfigMap. The `configmap` command provides two subcommands: `push` and `sync`.

Each template is stored as a ConfigMap data key in the format `<resource-dir>/<template-name>` (e.g. `pod--v1/debug`).

#### Push local templates to a ConfigMap

```sh
# Push all local templates
kubectl cwide configmap push

# Push only pod templates
kubectl cwide configmap push -r pod

# Push to a custom ConfigMap name and namespace
kubectl cwide configmap push --name my-templates --cm-namespace default
```

If the ConfigMap does not exist, it is created automatically. Otherwise, existing keys are updated and new keys are added.

#### Sync templates from a ConfigMap to local

```sh
# Pull templates from the default ConfigMap (cwide-templates in kube-system)
kubectl cwide configmap sync

# Sync from a specific ConfigMap
kubectl cwide configmap sync --name my-templates --cm-namespace default

# Force overwrite all local files regardless of priority
kubectl cwide configmap sync --force
```

Whether existing local files are overwritten depends on the `templateSources` order in the config file:
- `["local", "configmap"]` (default) — local files take priority, existing files are skipped
- `["configmap", "local"]` — ConfigMap takes priority, existing files are overwritten

Use `--force` to always overwrite regardless of priority.

### Marketplace

Browse and install community-shared templates from a GitHub repository.

The default repository is [`reborn1867/kubectl-cwide-templates`](https://github.com/reborn1867/kubectl-cwide-templates). Use `--repo` on any subcommand to point to a different repository.

#### List available resource types

```sh
kubectl cwide marketplace list

# Example output:
# deployment-apps-v1
# pod--v1
# service--v1
```

#### Search for templates by resource type

```sh
kubectl cwide marketplace search -r pod

# Example output:
# pod--v1/debug
# pod--v1/networking
```

#### Install a template

```sh
# Install the "debug" template for pods
kubectl cwide marketplace install -r pod -t debug

# Overwrite an existing local template
kubectl cwide marketplace install -r pod -t debug --force

# Install from a custom repository
kubectl cwide marketplace install -r pod -t debug --repo myorg/my-templates
```

The template is downloaded and saved into the matching resource directory under your local template path.

### Resource Tree

Visualize the relationship hierarchy between Kubernetes resources. The `tree` command starts from a root resource and discovers related resources via owner references, label selectors, or field references.

#### Inline relationships

```sh
# Deployment → ReplicaSets → Pods (via ownerReference)
kubectl cwide tree deployment/nginx \
  --related=replicasets:ownerRef \
  --related=pods:ownerRef:replicasets
```

#### Label selector relationships

```sh
# Service → Pods (via label selector)
kubectl cwide tree service/my-svc --related=pods:labelSelector
```

#### YAML rules file

Define relationships in a YAML file for reuse:

```yaml
# deploy-stack.yaml
relations:
  - resource: replicasets
    bind:
      type: ownerRef
  - resource: pods
    bind:
      type: ownerRef
      parent: replicasets
  - resource: services
    bind:
      type: labelSelector
```

```sh
kubectl cwide tree deployment/nginx -f deploy-stack.yaml
```

Binding types:
| Type | Description |
|---|---|
| `ownerRef` | Child resources whose ownerReferences point to the parent |
| `labelSelector` | Resources matched by the parent's label selector (bidirectional) |
| `fieldRef` | Resources referenced by name in a parent's field (via JSONPath) |

### Resource Aliases

Define custom short aliases for Kubernetes resource type names. Aliases are persisted in `~/.kubectl-cwide/config.yaml` and automatically resolved when used in `get` and `tree` commands.

#### Set an alias

```sh
kubectl cwide alias set pd pods
kubectl cwide alias set vw validatingwebhookconfigurations
```

When setting an alias, cwide checks for conflicts against:
- Existing aliases in your config (warns if overwriting)
- Built-in Kubernetes resource short names (warns if the alias shadows a built-in name)

#### List aliases

```sh
kubectl cwide alias list

# Output:
# ALIAS   RESOURCE
# pd      pods
# vw      validatingwebhookconfigurations
```

#### Delete an alias

```sh
kubectl cwide alias delete pd
```

#### Using aliases

Once set, aliases work transparently in `get` and `tree` commands:

```sh
# These are equivalent:
kubectl cwide get pods
kubectl cwide get pd

# Also works with tree:
kubectl cwide tree pd/my-pod --related=replicasets:ownerRef
```

### Template Functions

In addition to standard Go template and Helm-style functions, kubectl-cwide provides built-in template functions for common Kubernetes operations.

#### `probeCheck` — Live probe health check

The `probeCheck` function pings a pod's probe endpoint through the Kubernetes API server proxy and returns the result. It supports `readiness`, `liveness`, and `startup` probes with `httpGet` and `tcpSocket` handlers.

Usage in a YAML template:

```yaml
columns:
  - header: NAME
    fieldSpec: .metadata.name
  - header: READINESS
    template: '{{ probeCheck . "readiness" }}'
  - header: LIVENESS
    template: '{{ probeCheck . "liveness" }}'
```

Usage in a `.tpl` template:

```
NAME    READINESS    LIVENESS
.metadata.name {{ probeCheck . "readiness" }} {{ probeCheck . "liveness" }}
```

Possible output values:
| Output | Meaning |
|---|---|
| `OK (200)` | HTTP probe returned a success status code |
| `OK` | TCP probe port is reachable |
| `FAIL (502)` | HTTP probe returned an error status code |
| `FAIL (...)` | Probe request failed (with truncated error) |
| `N/A` | No probe configured for the container |
| `N/A (exec)` | Probe uses exec handler (cannot be checked remotely) |
| `N/A (grpc)` | Probe uses gRPC handler (not supported) |

## New in v0.8.0

This section covers everything added between v0.7.0 and v0.8.0. Each feature is standalone — you can use one without adopting the others.

### `get`: column selection, output formats, sort, and filter

The `get` command grew a small post-render pipeline. All four of these flags work together and can be combined with `--template` and `--all-namespaces`.

#### `-c/--columns` — pick a subset of the template's columns

Say your `pod--v1/default.yaml` renders eight columns but you only want three. Instead of editing the template file:

```sh
kubectl cwide get pod -c NAME,STATUS,AGE
```

Column names are case-insensitive and matched against the header text emitted by the template. If a name doesn't match, the command errors out and lists the available headers.

#### `-o/--output` — structured output

Get JSON, YAML, or CSV of the rendered rows for piping into other tools.

```sh
# JSON, one object per row keyed by header
kubectl cwide get pod -o json | jq '.[] | select(.STATUS != "Running")'

# YAML — same shape as JSON, easier for humans to skim
kubectl cwide get deploy -o yaml

# CSV — feed into spreadsheets or awk
kubectl cwide get pod -o csv > pods.csv
```

The JSON/YAML output is a flat array of `{HEADER: value, …}` maps. Multi-line cells are preserved as strings with embedded newlines.

#### `--sort-by` — sort rendered rows

Sorts by the named column, case-insensitively. Numeric strings sort numerically ("10" > "9"), otherwise lexicographically.

```sh
kubectl cwide get pod --sort-by=RESTARTS
kubectl cwide get pod --sort-by=AGE
```

#### `--filter` — post-render filtering

Applied after the template renders, so you can filter on template-computed columns (e.g. a custom `READY` column), not just raw fields. Repeatable — multiple `--filter` flags AND together.

| Operator | Meaning |
|---|---|
| `=` or `==` | exact equality |
| `!=` | not equal |
| `~regex` | regex match |
| `!~regex` | regex non-match |

Examples:

```sh
# Only Running pods
kubectl cwide get pod --filter='STATUS=Running'

# Pods with restarts, not in kube-system, using the "restart-reason" template
kubectl cwide get pod -t restart-reason \
  --filter='RESTARTS!=0' --filter='NAMESPACE!=kube-system'

# Pods whose name matches a regex
kubectl cwide get pod --filter='NAME~^web-'
```

Combining everything:

```sh
kubectl cwide get pod -A \
  -c NAMESPACE,NAME,STATUS,RESTARTS,AGE \
  --filter='STATUS!=Running' \
  --sort-by=RESTARTS \
  -o csv
```

### `tree`: bounded depth, cycle detection, and reverse walks

#### `--max-depth`

Cap render depth. `0` (the default) is unbounded. Everything below the cap is replaced with a `... (N more, --max-depth=D)` marker.

```sh
kubectl cwide tree deployment/nginx -f deploy-stack.yaml --max-depth 2
```

#### Automatic cycle detection

If two resources reference each other (via ownerRefs or custom rules), the walk breaks the loop with a `(cycle)` marker on the repeated node. No configuration needed — it's always on.

#### `--reverse` — walk ancestors

By default `tree` shows descendants. `--reverse` follows the controller ownerReference chain upward from the given resource.

```sh
# From a pod, walk up to the ReplicaSet and then the Deployment
kubectl cwide tree pod/nginx-abc-123 --reverse
```

You don't need `--rules` or `--related` in reverse mode — it uses ownerReferences directly. The chain stops when a resource has no ownerReferences or references a kind not resolvable via the current RESTMapper.

### New template functions

Available in both `.yaml` (via `template:` fields) and `.tpl` templates.

| Function | Signature | Purpose |
|---|---|---|
| `humanBytes` | `humanBytes v` | Format a byte count as `KiB`/`MiB`/`GiB`/… Accepts int, float, or numeric string. |
| `age` | `age v` | RFC3339 timestamp → human duration (`3d`, `5h12m`). Empty on parse errors. |
| `truncate` | `truncate n s` | Cut string at N runes, append `…` if truncated. |
| `b64dec` | `b64dec s` | Base64-decode. Returns input unchanged on decode error. Useful for Secret data. |
| `colorIf` | `colorIf cond color text` | Wrap `text` in ANSI color codes when `cond` is truthy. Colors: `red`, `green`, `yellow`, `blue`, `cyan`, `magenta`, `gray`. Respects `--no-color` and `NO_COLOR`. |
| `safeIndex` | `safeIndex root paths...` | Walk a nested map/slice by keys/indices. Returns `""` at any missing level instead of panicking. |

Example — Secret data preview:

```yaml
columns:
  - header: NAME
    fieldSpec: .metadata.name
  - header: TYPE
    fieldSpec: .type
  - header: PREVIEW
    template: '{{ truncate 20 (b64dec (safeIndex . "data" "tls.crt")) }}'
```

Example — colorize pod status:

```yaml
columns:
  - header: NAME
    fieldSpec: .metadata.name
  - header: STATUS
    template: '{{ $s := .status.phase }}{{ colorIf (eq $s "Running") "green" $s | colorIf (eq $s "Failed") "red" }}'
  - header: AGE
    template: '{{ age .metadata.creationTimestamp }}'
```

Set `NO_COLOR=1` in your environment or pass `--no-color` on any invocation to disable all `colorIf` output globally.

### Template inheritance via `_shared/`

Any `*.tpl` file under `<template-root>/_shared/` is loaded before every template you use. Define helper functions once and reference them from any resource template.

```
~/.kubectl-cwide/templates/
├── _shared/
│   └── helpers.tpl          ← defines PodReady, PodRestarts, ...
├── pod--v1/
│   ├── default.yaml
│   └── debug.yaml
└── deployment-apps-v1/
    └── default.yaml
```

Contents of `_shared/helpers.tpl`:

```
{{- define "AgeHuman" -}}{{ age .metadata.creationTimestamp }}{{- end -}}
{{- define "PodReady" -}}
  {{- $r := 0 -}}{{- $t := 0 -}}
  {{- range .status.containerStatuses -}}
    {{- $t = add 1 $t -}}
    {{- if .ready -}}{{- $r = add 1 $r -}}{{- end -}}
  {{- end -}}
  {{ $r }}/{{ $t }}
{{- end -}}
```

Any template can now call `{{ template "PodReady" . }}` or `{{ template "AgeHuman" . }}` without redefining them.

### Per-context and per-namespace default template

Set a different default template based on which kubeconfig context or namespace you're targeting. Edit `~/.kubectl-cwide/config.yaml`:

```yaml
templatePath: /home/you/.kubectl-cwide/templates
defaultTemplateContext:
  prod: compact
  dev: verbose
defaultTemplateNamespace:
  kube-system: minimal
  monitoring: full
```

When you run `kubectl cwide get <kind>` without `--template`, cwide picks the effective default in this order:

1. Namespace match (via `defaultTemplateNamespace`)
2. Context match (via `defaultTemplateContext`)
3. Literal `default`

Passing `-t <name>` explicitly always overrides the resolved default.

### `template lint` and `template scaffold`

Two new subcommands to help authoring.

```sh
# Lint one template
kubectl cwide template lint ~/.kubectl-cwide/templates/pod--v1/default.yaml

# Lint every YAML template under the tree
find ~/.kubectl-cwide/templates -name '*.yaml' \
  -exec kubectl cwide template lint {} \;
```

Lint checks that the file parses, at least one column exists, every column has a header, every column has either a `fieldSpec` or a `template`, and every `fieldSpec` is a valid JSONPath.

```sh
# Generate a starter template for a new resource
kubectl cwide template scaffold pod > ~/.kubectl-cwide/templates/pod--v1/starter.yaml
```

The scaffold emits three uncommented columns (NAMESPACE, NAME, AGE) plus a set of commented-out common columns you can flip on.

### Alias groups and cluster-scoped sync

#### Alias groups

An alias target may be a comma-separated list. `get` and `tree` pass the list straight through to Kubernetes' resource builder, so a single alias can list multiple kinds at once.

```sh
kubectl cwide alias set core pod,service,configmap
kubectl cwide get core
```

This lists pods, services, and configmaps in one call.

#### Cluster-scoped alias sync

Share aliases across a team by riding along on the templates ConfigMap. A reserved data key `__aliases__` stores the YAML-marshaled alias map.

```sh
# Team lead: push local aliases into the ConfigMap
kubectl cwide configmap push --with-aliases

# Team members: pull them into their local config
kubectl cwide configmap sync
```

`configmap sync` merges the remote aliases into `~/.kubectl-cwide/config.yaml`. Existing local aliases are preserved unless `--force` is passed, in which case the ConfigMap wins on conflict.

### Marketplace version pinning

Install a template at a specific git ref (branch, tag, or commit SHA). The pin is recorded in `~/.kubectl-cwide/marketplace.lock` so future `sync` operations know which version you locked to.

```sh
# Install "debug" template for pods, pinned to a tag
kubectl cwide marketplace install -r pod -t debug --ref v1.2.0

# Install at a specific commit
kubectl cwide marketplace install -r pod -t debug --ref 4a3b2c1d
```

Lock file entries look like:

```yaml
pins:
  - repo: reborn1867/kubectl-cwide-templates
    resource: pod
    template: debug
    ref: v1.2.0
```

### Shell completion

```sh
# Bash (current session)
source <(kubectl cwide completion bash)

# Bash (persistent, Linux)
kubectl cwide completion bash | sudo tee /etc/bash_completion.d/kubectl-cwide

# Zsh
source <(kubectl cwide completion zsh)

# Fish
kubectl cwide completion fish > ~/.config/fish/completions/kubectl-cwide.fish

# PowerShell
kubectl cwide completion powershell > kubectl-cwide.ps1
```

Completes commands, subcommands, flags, and **dynamic argument values**:

- `kubectl cwide get <TAB>` — cluster's resource types + short names + user aliases
- `kubectl cwide tree <TAB>` — same, before the `/` separator
- `kubectl cwide alias delete <TAB>` — currently-configured alias names
- `--template=<TAB>` — template names discovered from the template root
- `--context=<TAB>` — kubeconfig contexts
- `-o <TAB>` — `json`, `yaml`, `csv`
- `template list -r <TAB>`, `template edit -r <TAB>`, `template scaffold <TAB>` — resource types
- `template edit -t <TAB>` — templates that exist for the specified resource

Dynamic completions are best-effort — if the cluster is unreachable, they return no suggestions rather than a visible error.

### Color and Ctrl-C

- **`--no-color`** — global flag on every subcommand. Overrides `NO_COLOR`.
- **`NO_COLOR=1`** — standard env var; disables color output for the process.
- **Signal handling** — the root command installs a `SIGINT`/`SIGTERM` handler that cancels the shared request context. Ctrl-C now interrupts long-running list/watch calls cleanly instead of leaving them hanging.

## Reference 
- **cli-runtime**: A set of packages to share code with `kubectl` for printing output or sharing command-line options.
- **sample-cli-plugin**: An example plugin implementation in Go.
- **go template**: Data-driven templates for generating textual output. 
- **Cookbook**: [`docs/cookbook.md`](docs/cookbook.md) — ready-to-use template recipes.
- **Migration guide**: [`docs/migration-from-custom-cols.md`](docs/migration-from-custom-cols.md) — moving from `kubectl-custom-cols`.
