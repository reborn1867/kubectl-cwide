# kubectl-cwide
A `krew` plugin for customized wide output of `kubectl`.

Special thanks to [kubectl-custom-cols](https://github.com/webofmars/kubectl-custom-cols), which inspired this project.

Managing Kubernetes resources often requires printing extra columns for specific information. While `kubectl` provides JSONPath expressions to customize table output, memorizing long commands can be tedious. 

`kubectl-cwide` simplifies this process by allowing you to persist custom column formats. You can easily edit, extend, alias, or share these formats with your team members.

## Highlights
- **Automatic Template Generation**: Automatically generate custom column templates for Kubernetes resources, saving time and effort.
- **Customizable Output**: Easily define and persist custom column formats for specific resource types.
- **Editable Templates**: Modify and extend templates as needed to suit your workflow.
- **Team Collaboration**: Share custom column templates with team members for consistent and standardized output.
- **Support for All Resources**: Works with all Kubernetes resource types, including CRDs.
- **Integration with `krew`**: Seamlessly install and manage the plugin using the Kubernetes `krew` plugin manager.

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

## Reference 
- **cli-runtime**: A set of packages to share code with `kubectl` for printing output or sharing command-line options.
- **sample-cli-plugin**: An example plugin implementation in Go.