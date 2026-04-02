package marketplace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubectl-cwide/pkg/utils"
	"github.com/spf13/cobra"
)

func NewCmdInstall() *cobra.Command {
	var force bool

	installCMD := &cobra.Command{
		Use:   "install",
		Short: "Install a template from the marketplace",
		Long: `Download a community template and save it to the local template directory.

The template is fetched from the remote GitHub repository and saved under the
matching resource directory. Existing files are not overwritten unless --force
is specified.`,
		Example: `  # Install the "debug" template for pods
  kubectl cwide marketplace install -r pod -t debug

  # Overwrite an existing template
  kubectl cwide marketplace install -r pod -t debug --force

  # Install from a custom repository
  kubectl cwide marketplace install -r pod -t debug --repo myorg/my-templates`,
		RunE: func(cmd *cobra.Command, args []string) error {
			repo := cmd.Flag("repo").Value.String()
			resource := cmd.Flag("resource").Value.String()
			templateName := cmd.Flag("template").Value.String()

			// Resolve the remote directory for this resource
			entries, err := listContents(repo, basePath)
			if err != nil {
				return fmt.Errorf("failed to list marketplace: %w", err)
			}

			prefix := strings.ToLower(resource) + "-"
			var matchedDirs []string
			for _, e := range entries {
				if e.Type == "dir" && strings.HasPrefix(e.Name, prefix) {
					matchedDirs = append(matchedDirs, e.Name)
				}
			}

			if len(matchedDirs) == 0 {
				return fmt.Errorf("no resource directory found for %q in the marketplace", resource)
			}
			if len(matchedDirs) > 1 {
				return fmt.Errorf("multiple directories match %q: %v; specify a more precise resource type", resource, matchedDirs)
			}

			remoteDir := matchedDirs[0]
			fileName := templateName + ".yaml"

			// Find the file in the remote directory
			files, err := listContents(repo, basePath+"/"+remoteDir)
			if err != nil {
				return fmt.Errorf("failed to list templates in %s: %w", remoteDir, err)
			}

			var downloadURL string
			for _, f := range files {
				if f.Name == fileName {
					downloadURL = f.DownloadURL
					break
				}
			}
			if downloadURL == "" {
				return fmt.Errorf("template %q not found in %s", templateName, remoteDir)
			}

			// Resolve local template path
			absPath, err := utils.ResolveTemplatePath(cmd)
			if err != nil {
				return fmt.Errorf("failed to resolve template path: %w", err)
			}

			localDir := filepath.Join(absPath, remoteDir)
			localPath := filepath.Join(localDir, fileName)

			if !force && utils.CheckFileExists(localPath) {
				return fmt.Errorf("template already exists at %s; use --force to overwrite", localPath)
			}

			// Download the template
			data, err := downloadFile(downloadURL)
			if err != nil {
				return fmt.Errorf("failed to download template: %w", err)
			}

			// Ensure the directory exists and write the file
			if err := os.MkdirAll(localDir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", localDir, err)
			}
			if err := os.WriteFile(localPath, data, 0644); err != nil {
				return fmt.Errorf("failed to write template: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Installed template: %s\n", localPath)
			return nil
		},
	}

	installCMD.Flags().StringP("resource", "r", "", "Resource type (e.g. pod, deployment)")
	installCMD.Flags().StringP("template", "t", "", "Template name to install (without extension)")
	installCMD.Flags().BoolVar(&force, "force", false, "Overwrite existing template file")
	installCMD.MarkFlagRequired("resource")
	installCMD.MarkFlagRequired("template")

	return installCMD
}
