# kubectl-cwide
krew plugin for customized wide output of kubectl

It's troublesome to print extra columns for specific information of k8s resources, even though kubectl already provide the ability to use jsonpath expression to customize table output, but it's still quite hard to memorize those long commands, you have to always note them somewhere. 

`kubectl-cwide` provide an easy way to persistent custom column formats and you are free to edit, increase, alias the format or even share it with your team members.

## Usage
- Init custom column template
  ```
  kubectl cwide init --template-path /tmp/cwide --kubeconfig <path-to-kubeconfig-file-with-crd-read-permission>
  ```

- Edit custom column template as you want in template-path 
- Check it out!
  ```
  kubectl cwide get <resource-kind> <resource-name>
  ```


## Reference 
- cli-runtime: a set of packages to share code with kubectl for printing output or sharing command-line options
- sample-cli-plugin: an example plugin implementation in Go

