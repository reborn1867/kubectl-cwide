package completion

import (
	"os"

	"github.com/spf13/cobra"
)

func NewCmdCompletion() *cobra.Command {
	cmd := &cobra.Command{
		Use:                   "completion [bash|zsh|fish|powershell]",
		Short:                 "Generate shell completion scripts",
		Long:                  completionLong,
		Example:               completionExample,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(c *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return c.Root().GenBashCompletionV2(os.Stdout, true)
			case "zsh":
				return c.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return c.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return c.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}
			return nil
		},
	}
	return cmd
}

const completionLong = `Output shell completion code for the specified shell (bash, zsh, fish, or powershell).
The script must be sourced to enable interactive completion of commands, flags, and arguments.`

const completionExample = `  # Load completion for the current bash session
  source <(kubectl cwide completion bash)

  # Persist bash completion (Linux)
  kubectl cwide completion bash > /etc/bash_completion.d/kubectl-cwide

  # Load completion for the current zsh session
  source <(kubectl cwide completion zsh)

  # Persist fish completion
  kubectl cwide completion fish > ~/.config/fish/completions/kubectl-cwide.fish

  # Save powershell completion
  kubectl cwide completion powershell > kubectl-cwide.ps1`
