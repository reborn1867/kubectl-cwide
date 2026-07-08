# Curated Native Kubernetes Templates

A set of hand-tuned custom-column templates for the most common built-in Kubernetes resources. They surface information that `kubectl get -o wide` omits (image, resource requests/limits, probe endpoints, HPA metrics, PV volume source, etc.) so you spend less time in `describe` and `-o yaml`.

## Install

Two ways to use them.

### Point cwide at this directory
```sh
kubectl cwide get pod --template-path ./templates/native
```

### Copy into your own template root
```sh
kubectl cwide init --template-path ~/.cwide-templates
cp -r templates/native/* ~/.cwide-templates/
```

## What's included

| Resource | Template | What it adds over `kubectl get` |
|---|---|---|
| `pod` | `default` | Reason (CrashLoopBackOff/ImagePullBackOff), summed restart count, node, pod IP, QoS |
| `pod` | `resources` | Per-pod CPU/memory requests and limits, QoS class |
| `pod` | `probes` | Liveness/readiness/startup probe endpoints |
| `pod` | `images` | Container names, image tags, imagePullPolicy |
| `deployment` | `default` | Ready/desired ratio, strategy, first image, progress condition reason |
| `statefulset` | `default` | Current vs updated replicas, headless service, update strategy, image |
| `daemonset` | `default` | Desired/current/ready/available/misscheduled, node selector |
| `replicaset` | `default` | Owner reference (Deployment name), image |
| `service` | `default` | External IP, ports:nodePorts, selector labels |
| `node` | `default` | Ready status, roles, kubelet version, internal IP, OS/kernel, runtime, allocatable, schedulability |
| `persistentvolumeclaim` | `default` | Bound volume, capacity, access modes, storage class, volume mode |
| `persistentvolume` | `default` | Capacity, reclaim policy, claim, storage class, underlying volume source (csi/hostPath/nfs/…) |
| `horizontalpodautoscaler` | `default` | Metric type/name, current utilization %, min/max/current/desired replicas |
| `ingress` | `default` | Class, hosts, address, ports, path → backend mapping |
| `job` | `default` | Completions ratio, active/failed counts, condition, start→completion window |
| `cronjob` | `default` | Schedule, timezone, suspend, active count, last schedule/success, concurrency policy |
| `configmap` | `default` | Key count (regular and binary), owner reference |
| `secret` | `default` | Type, list of key names, owner reference |
| `event` | `default` | Last seen, type, reason, involved object, source, count, message |

## Usage

Once installed, get uses the `default` template automatically:
```sh
kubectl cwide get pod -A
```

Switch to a non-default template with `-t`:
```sh
kubectl cwide get pod -t resources
kubectl cwide get pod -t probes
kubectl cwide get pod -t images
```

## Contributing

If you improve one of these or add a new resource, open a PR. Guidelines:
- Prefer `fieldSpec` (JSONPath) for simple fields; use `template` only when you need computation.
- Guard nil paths (`.status.readyReplicas | default 0`) — otherwise pending resources will render `<no value>`.
- Keep `AGE` last so it prints as a relative duration in the default column formatter.
- Include `NAMESPACE` first when the resource is namespaced, so `-A` output stays readable.
