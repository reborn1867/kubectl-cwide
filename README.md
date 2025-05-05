# kubectl-cwide
A `krew` plugin for customized wide output of `kubectl`.

Special thanks to [kubectl-custom-cols](https://github.com/webofmars/kubectl-custom-cols), which inspired this project.

Managing Kubernetes resources often requires printing extra columns for specific information. While `kubectl` provides JSONPath expressions to customize table output, memorizing long commands can be tedious. 

`kubectl-cwide` simplifies this process by allowing you to persist custom column formats. You can easily edit, extend, alias, or share these formats with your team members.

## Highlights
- **Template Based Parsing**: In addition to native JSONPath parsing, you can effortlessly customize your table output using the same approach as Helm templates, harnessing the flexibility and power of [text.Template](https://pkg.go.dev/text/template).
- **Automatic Template Generation**: Automatically generate custom column templates for Kubernetes resources, saving time and effort.
- **Customizable Output**: Define and persist custom column formats for specific resource types with ease.
- **Editable Templates**: Modify and extend templates as needed to suit your workflow.
- **Team Collaboration**: Share custom column templates with team members for consistent and standardized output.

## Installation
As a [krew](https://github.com/kubernetes-sigs/krew) plugin, `kubectl-ciwe` can be installed with a simple command as following once it's officially accepted.
```
kubectl krew install cwide
```

## Usage
1. **Initialize Custom Column Template**: Generate a template file based on CRD.
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

### Sample Template File

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

### Sample Template File with `text.Template`
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

kubectl cwide get pod
NAME                       READY   STATUS    RESTARTS   AGE
fluentd-cpg6x              1/1     Running   0          3d2h
fluentd-pr48h              1/1     Running   0          3d2h
grafana-78578fcfd5-2lhf8   2/2     Running   0          7d23h
grafana-78578fcfd5-9s7q4   2/2     Running   0          7d23h
```

We managed to make output looks almost the same as `kubectl get pod` which is not supported by custom columns output `-ocustom-columns`. By leveraging various helm template functions (and there will be more in the future), you get to freely create your own customized output.

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

## Reference 
- **cli-runtime**: A set of packages to share code with `kubectl` for printing output or sharing command-line options.
- **sample-cli-plugin**: An example plugin implementation in Go.
- **go template**: Data-driven templates for generating textual output. 