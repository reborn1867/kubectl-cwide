package completion

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func NewCmdCompletion() *cobra.Command {
	var forName string

	cmd := &cobra.Command{
		Use:                   "completion [bash|zsh|fish|powershell]",
		Short:                 "Generate shell completion scripts",
		Long:                  completionLong,
		Example:               completionExample,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(c *cobra.Command, args []string) error {
			shell := args[0]
			// When --command is set, we generate the script for that invocation
			// name instead of the binary's own name. This lets users tab-complete
			// against a shell alias (e.g. `alias k=kubectl-cwide`) or the kubectl
			// plugin path (`kubectl cwide ...`) — cobra keys completion off the
			// first word, so the registered function has to match.
			root := c.Root()
			if forName != "" {
				root.Use = forName
			}

			switch shell {
			case "bash":
				return root.GenBashCompletionV2(os.Stdout, true)
			case "zsh":
				return root.GenZshCompletion(os.Stdout)
			case "fish":
				return root.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(os.Stdout)
			}
			return fmt.Errorf("unsupported shell %q", shell)
		},
	}

	cmd.Flags().StringVar(&forName, "command", "",
		"Register completion under this command name instead of the binary name. "+
			"Use this for shell aliases (e.g. --command=kc) or when invoking via `kubectl <plugin>` "+
			"(pass a single word — bash keys completion off the first word only).")

	_ = cmd.RegisterFlagCompletionFunc("command", func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		suggestions := []string{"kubectl-cwide", "cwide", "kc"}
		var out []string
		for _, s := range suggestions {
			if strings.HasPrefix(s, toComplete) {
				out = append(out, s)
			}
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	})

	return cmd
}

const completionLong = `Output shell completion code for the specified shell (bash, zsh, fish, or powershell).
The script must be sourced to enable interactive completion of commands, flags, and arguments.

IMPORTANT — bash/zsh key completion off the FIRST word of the command line. If you
invoke cwide via a wrapper (e.g. the ` + "`kubectl cwide`" + ` plugin path, or a personal
alias), you MUST generate completions for THAT invocation name, not for
kubectl-cwide directly. Use --command to change the registered name.

Examples:
  # Direct binary usage — the default
  source <(kubectl-cwide completion bash)

  # You invoke it as a shell alias 'kc' (e.g. 'alias kc=kubectl-cwide')
  source <(kubectl-cwide completion bash --command=kc)

  # NOTE: kubectl plugin completion (kubectl cwide <TAB>) is not natively
  # supported by kubectl itself. The most portable workaround is a shell
  # alias that shadows the plugin path:
  #     alias 'kubectl cwide'=kubectl-cwide     # bash won't accept spaces in aliases
  # So use a single-word alias like 'kc' and generate completion for that.`

const completionExample = `  # Load completion for the current bash session (direct binary)
  source <(kubectl-cwide completion bash)

  # Register completion under a shell alias
  echo "alias kc=kubectl-cwide" >> ~/.bashrc
  echo 'source <(kubectl-cwide completion bash --command=kc)' >> ~/.bashrc
  # then 'kc get <TAB>' completes resource types

  # Persist bash completion system-wide (Linux)
  kubectl-cwide completion bash | sudo tee /etc/bash_completion.d/kubectl-cwide

  # Zsh
  source <(kubectl-cwide completion zsh)

  # Fish
  kubectl-cwide completion fish > ~/.config/fish/completions/kubectl-cwide.fish

  # PowerShell
  kubectl-cwide completion powershell > kubectl-cwide.ps1`
