package template

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kubectl-cwide/pkg/cmd/completions"
	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
)

func NewCmdEdit() *cobra.Command {
	editCMD := &cobra.Command{
		Use:        "edit",
		Aliases:    []string{"e"},
		SuggestFor: []string{"modify", "update"},
		Short:      "Open a template file in an editor",
		Long: `Open the column template for the specified resource type in your preferred
editor. The editor is determined by the EDITOR environment variable, falling
back to vi.

By default the "default" template is opened. Use -t to specify a different
template name.`,
		Example: `  # Edit the default template for pods
  kubectl cwide template edit -r pod

  # Edit a specific named template
  kubectl cwide template edit -r deployment -t minimal

  # Edit from a specific directory
  kubectl cwide template edit -r pod --template-path ~/my-templates`,
		RunE: func(cmd *cobra.Command, args []string) error {
			absPath, err := utils.ResolveTemplatePath(cmd)
			if err != nil {
				return fmt.Errorf("failed to resolve template path: %w", err)
			}

			resourceType := cmd.Flag("resource").Value.String()
			templateName := cmd.Flag("template").Value.String()

			pattern := filepath.Join(absPath, fmt.Sprintf("%s-*", resourceType))
			files, err := filepath.Glob(pattern)
			if err != nil {
				return fmt.Errorf("failed to search for resource directories: %w", err)
			}

			if len(files) == 0 {
				return fmt.Errorf("no resource directory found for %q; run 'init' first", resourceType)
			}

			if len(files) != 1 {
				return fmt.Errorf("found multiple directories for %q: %v; specify a more precise resource type", resourceType, files)
			}

			// Try .yaml first, then .tpl
			yamlPath := filepath.Join(files[0], templateName+".yaml")
			tplPath := filepath.Join(files[0], templateName+".tpl")

			var targetPath string
			switch {
			case fileExists(yamlPath):
				targetPath = yamlPath
			case fileExists(tplPath):
				targetPath = tplPath
			default:
				available := installedTemplateNames(files[0])
				if len(available) == 0 {
					return fmt.Errorf("template %q not found for %q — no templates installed in %s (run 'kubectl cwide init' or 'kubectl cwide template scaffold %s')",
						templateName, resourceType, files[0], resourceType)
				}
				return fmt.Errorf("template %q not found for %q; available: %s",
					templateName, resourceType, strings.Join(available, ", "))
			}

			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}

			// Snapshot mtime + size so we can detect the file being
			// overwritten mid-edit (e.g. by a concurrent configmap sync).
			preStat, preErr := os.Stat(targetPath)

			editorCmd := exec.Command(editor, targetPath)
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr

			if err := editorCmd.Run(); err != nil {
				return fmt.Errorf("editor exited with error: %w", err)
			}

			if preErr == nil {
				postStat, err := os.Stat(targetPath)
				if err == nil && (postStat.ModTime() != preStat.ModTime() && postStat.Size() != preStat.Size()) {
					// Both mtime AND size changed — user probably saved. Fine.
				} else if err == nil && postStat.ModTime() != preStat.ModTime() && postStat.Size() == preStat.Size() {
					// mtime changed but size didn't — possible sync overwrite of identical content.
					fmt.Fprintf(cmd.ErrOrStderr(),
						"warning: %s mtime changed during edit but size is unchanged; a concurrent configmap sync may have run — re-check your changes\n",
						targetPath)
				}
			}
			return nil
		},
	}

	editCMD.Flags().StringP("resource", "r", "", "Resource type to edit the template for (e.g. pod, deployment)")
	_ = editCMD.RegisterFlagCompletionFunc("resource", completions.ResourceTypes)
	editCMD.Flags().StringP("template", "t", "default", "Name of the template to edit (without extension)")
	_ = editCMD.RegisterFlagCompletionFunc("template", completions.TemplateNames)
	editCMD.MarkFlagRequired("resource")

	return editCMD
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// installedTemplateNames returns the basenames (without extension) of every
// .yaml/.yml/.tpl template in dir, sorted and deduplicated. A .yaml and a .tpl
// sharing a basename collapse to one entry, matching how templates are resolved
// (.yaml preferred over .tpl). Used to list options in "template not found"
// errors so the user sees what's actually available.
func installedTemplateNames(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		switch {
		case strings.HasSuffix(name, ".yaml"):
			seen[strings.TrimSuffix(name, ".yaml")] = struct{}{}
		case strings.HasSuffix(name, ".yml"):
			seen[strings.TrimSuffix(name, ".yml")] = struct{}{}
		case strings.HasSuffix(name, ".tpl"):
			seen[strings.TrimSuffix(name, ".tpl")] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
