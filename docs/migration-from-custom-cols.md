# Migrating from kubectl-custom-cols

`kubectl-cwide` was inspired by [kubectl-custom-cols](https://github.com/webofmars/kubectl-custom-cols). If you're already using custom-cols, migration is straightforward — the JSONPath expressions carry over verbatim; only the surrounding structure changes.

## Concept mapping

| kubectl-custom-cols                       | kubectl-cwide                                              |
|-------------------------------------------|------------------------------------------------------------|
| One template file per resource            | Directory per resource; multiple named templates each      |
| Two lines: header + JSONPath              | Same (`.tpl`) or structured YAML (`.yaml`)                 |
| Called via `kubectl custom-cols <kind>`   | Called via `kubectl cwide get <kind>`                      |
| Global template dir                       | `--template-path` flag, or `~/.kubectl-cwide/config.yaml`  |
| Plain JSONPath only                       | JSONPath + `text/template` + Helm-style helpers            |
| No sharing                                | ConfigMap sync + Marketplace                               |

## Side-by-side syntax

### Pod overview

**custom-cols** (`~/.custom-cols/pod`):
```
NAME     STATUS         AGE
.metadata.name   .status.phase   .metadata.creationTimestamp
```

**cwide** (`~/.kubectl-cwide/templates/pod--v1/default.tpl`):
```
NAME     STATUS         AGE
.metadata.name   .status.phase   .metadata.creationTimestamp
```

Identical body — just the location changes. In cwide, `pod--v1` is `<resource>-<group>-<version>` where the empty group leaves a doubled dash.

### Selecting a template variant

**custom-cols** — one template per kind; no variants.

**cwide** — multiple named templates per kind:
```sh
kubectl cwide get pod --template debug
kubectl cwide get pod --template original-output
```

## Migration steps

1. **Copy your existing files** into the cwide layout:
   ```sh
   mkdir -p ~/.kubectl-cwide/templates/pod--v1
   cp ~/.custom-cols/pod ~/.kubectl-cwide/templates/pod--v1/default.tpl
   ```
   Repeat for each kind. The rule for the directory name is `<plural>-<group>-<version>` (built-ins have an empty group — hence `pod--v1`, `service--v1`, `configmap--v1`, `deployment-apps-v1`, `job-batch-v1`, etc.).

2. **Init to fill in the rest**. Once you've copied your bespoke templates, run:
   ```sh
   kubectl cwide init
   ```
   to auto-generate defaults for every other resource the cluster serves. Your copied files are preserved.

3. **Point your muscle memory**. Replace any `alias` for `kubectl custom-cols` with:
   ```sh
   alias k='kubectl cwide get'
   ```

## Features you gain by moving

- **Named variants** — `pod/default.tpl` and `pod/debug.tpl` coexist; pick with `--template`.
- **Text templates + JSONPath together** — call `{{ template "PodReady" . }}` from within a `.tpl` line.
- **Team sharing** — push templates into a ConfigMap so the whole cluster gets the same output shape.
- **Marketplace** — browse and install community-shared templates.
- **Aliases** — `kubectl cwide alias set vw validatingwebhookconfigurations`, then `kubectl cwide get vw` works everywhere.
- **Tree view** — `kubectl cwide tree deploy/my-app` walks ownerRefs, label selectors, and field refs.

## What doesn't carry over

- **Global vs per-kind config format** — custom-cols reads one file per kind; cwide reads a directory. There's no drop-in "flat file" mode.
- **Command name** — `custom-cols` becomes `cwide get`. If you have shell aliases or scripts, update them.

## Getting help

Open an issue at https://github.com/reborn1867/kubectl-cwide/issues if a migration edge case bites you — we'll add it to this page.
