package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/kubectl-cwide/pkg/common"
	"github.com/spf13/cobra"
)

func NewCmdConfig() *cobra.Command {
	return &cobra.Command{
		Use:        "config",
		Aliases:    []string{"cfg"},
		SuggestFor: []string{"configure", "conf", "settings"},
		Short:      "Open the config file in an editor",
		Long: `Open the kubectl-cwide configuration file (~/.kubectl-cwide/config.yaml)
in your preferred editor. The editor is determined by the EDITOR environment
variable, falling back to vi.`,
		Example: `  # Edit the config file
  kubectl cwide config

  # Use a specific editor
  EDITOR=nano kubectl cwide config`,
		RunE: func(cmd *cobra.Command, args []string) error {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}

			configPath := filepath.Join(homeDir, common.ConfigPath)
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				return fmt.Errorf("config file not found at %s; run 'init' first", configPath)
			}

			return openInEditor(configPath)
		},
	}
}

func openInEditor(path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	editorCmd := exec.Command(editor, path)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("editor exited with error: %w", err)
	}
	return nil
}
