# Cookbook

Short recipes for common template patterns. Each recipe shows the template file and the resulting `kubectl cwide get` output.

## Pod restart reason

Show why containers restarted — surface `lastState.terminated.reason` alongside restart count.

`~/.kubectl-cwide/templates/pod--v1/restart-reason.tpl`:

```
NAME             RESTARTS         LAST_REASON                                                          LAST_EXIT
.metadata.name   .status.containerStatuses[0].restartCount   .status.containerStatuses[0].lastState.terminated.reason   .status.containerStatuses[0].lastState.terminated.exitCode
```

Run:
```sh
kubectl cwide get pod --template restart-reason
```

## PVC bound-to node

Trace a PVC through its bound PV to the node currently hosting it.

`~/.kubectl-cwide/templates/persistentvolumeclaim--v1/bound-node.tpl`:

```
NAME             STATUS               STORAGE                                            VOLUME               NODE
.metadata.name   .status.phase        .spec.resources.requests.storage                   .spec.volumeName     {{ template "PVCNode" . }}

{{- define "PVCNode" -}}
{{- $pv := lookup "v1" "PersistentVolume" "" .spec.volumeName -}}
{{- if $pv -}}
  {{- if $pv.spec.nodeAffinity -}}
    {{- range $pv.spec.nodeAffinity.required.nodeSelectorTerms -}}
      {{- range .matchExpressions -}}
        {{- if eq .key "kubernetes.io/hostname" -}}{{- index .values 0 -}}{{- end -}}
      {{- end -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- end -}}
```

## HPA current vs desired

At-a-glance capacity signal.

`~/.kubectl-cwide/templates/horizontalpodautoscaler-autoscaling-v2/desired.tpl`:

```
NAME             MIN                        MAX                        CURRENT                     DESIRED
.metadata.name   .spec.minReplicas          .spec.maxReplicas          .status.currentReplicas     .status.desiredReplicas
```

## Deployment rollout at a glance

Ready vs updated vs available.

`~/.kubectl-cwide/templates/deployment-apps-v1/rollout.tpl`:

```
NAME             READY                      UPDATED                       AVAILABLE                 UNAVAILABLE
.metadata.name   .status.readyReplicas      .status.updatedReplicas       .status.availableReplicas .status.unavailableReplicas
```

## Node pressure

Which nodes are under memory / disk pressure right now.

`~/.kubectl-cwide/templates/node--v1/pressure.tpl`:

```
NAME             READY                                                                                          MEM_PRESSURE                                                                                DISK_PRESSURE                                                                                     PID_PRESSURE
.metadata.name   .status.conditions[?(@.type=="Ready")].status   .status.conditions[?(@.type=="MemoryPressure")].status   .status.conditions[?(@.type=="DiskPressure")].status   .status.conditions[?(@.type=="PIDPressure")].status
```

## Ingress backend routes

Show all `host → path → service:port` rows a single Ingress serves.

`~/.kubectl-cwide/templates/ingress-networking.k8s.io-v1/routes.tpl`:

```
NAME             HOST                            PATH                                    SERVICE                                              PORT
.metadata.name   .spec.rules[0].host             .spec.rules[0].http.paths[0].path       .spec.rules[0].http.paths[0].backend.service.name    .spec.rules[0].http.paths[0].backend.service.port.number
```

Note: this only shows the first rule/path. For fully expanded routing, use `text/template` with `range` over `.spec.rules`.

## Secret age & keys

Names of the keys stored in each Secret plus how old it is.

`~/.kubectl-cwide/templates/secret--v1/keys.tpl`:

```
NAME             TYPE                        KEYS                                                          AGE
.metadata.name   .type                       {{ range $k, $_ := .data }}{{$k}} {{ end }}                   .metadata.creationTimestamp
```

## Service to endpoints

Which service targets which pod IPs. Great for debugging "why is nothing responding on my Service?".

`~/.kubectl-cwide/templates/service--v1/endpoints.tpl`:

```
NAME             TYPE                                CLUSTERIP                     PORTS                                              SELECTOR
.metadata.name   .spec.type                          .spec.clusterIP               .spec.ports[0].port                                {{ range $k, $v := .spec.selector }}{{$k}}={{$v}} {{ end }}
```

## Job success/failure

Job progress across attempts.

`~/.kubectl-cwide/templates/job-batch-v1/attempts.tpl`:

```
NAME             COMPLETIONS                                     SUCCEEDED                     FAILED                     ACTIVE                          START
.metadata.name   .status.succeeded/.spec.completions             .status.succeeded             .status.failed             .status.active                  .status.startTime
```

---

### Contributing recipes

Have a template you use daily? Add it here in the same format:
1. One sentence describing what it shows.
2. Path & filename.
3. Template body in a code block.

Then open a PR.
