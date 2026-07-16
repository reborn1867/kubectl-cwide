package utils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubectl-cwide/pkg/common"
	"github.com/kubectl-cwide/pkg/models"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// create a file if not exists, create parent directories if not exists
func CreateFileIfNotExists(path string, content []byte) error {
	// Create parent directories if not exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	// Create the file if it doesn't exist
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write content to the file
	_, err = file.Write(content)
	if err != nil {
		return err
	}

	return nil
}

// create or update file in given path
func CreateOrFormatFile(path string, content []byte) error {
	// Create parent directories if not exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	// Read the content of the file if exists
	if _, err := os.Stat(path); err == nil {
		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file: %v", err)
		}

		content = formatContent(b)
	}

	// Open the file, create if it doesn't exist
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write content to the file
	_, err = file.Write(content)
	if err != nil {
		return err
	}

	return nil
}

// Ensure the first character of each column is aligned in every line.
// Each column should be left-aligned.
// The first line is the header, the second line is the jsonpath.
func formatContent(content []byte) []byte {
	lines := strings.Split(string(content), "\n")
	if len(lines) < 2 {
		return content // Return as is if there are less than two lines
	}

	// Split the first line (header) and the second line (jsonpath) into columns
	headerColumns := strings.Fields(lines[0])
	jsonPathColumns := strings.Fields(lines[1])

	// Calculate the maximum width of each column
	columnWidths := make([]int, len(headerColumns))
	for i := range headerColumns {
		columnWidths[i] = len(headerColumns[i])
		if i < len(jsonPathColumns) && len(jsonPathColumns[i]) > columnWidths[i] {
			columnWidths[i] = len(jsonPathColumns[i])
		}
	}

	// Format the header row
	var formattedHeader strings.Builder
	for i, col := range headerColumns {
		formattedHeader.WriteString(strings.ToUpper(col))
		if i < len(columnWidths)-1 {
			formattedHeader.WriteString(strings.Repeat(" ", columnWidths[i]-len(col)+1))
		}
	}

	// Format the jsonpath row
	var formattedJsonPath strings.Builder
	for i, col := range jsonPathColumns {
		formattedJsonPath.WriteString(col)
		if i < len(columnWidths)-1 {
			formattedJsonPath.WriteString(strings.Repeat(" ", columnWidths[i]-len(col)+1))
		}
	}

	// Combine the formatted rows
	return []byte(formattedHeader.String() + "\n" + formattedJsonPath.String() + "\n")
}

// build byte array in following format from AdditionalPrinterColumns, the text has two lines, the first row is headers, the second row is the jsonpath
// correct indentation is important
// BuildColumnTemplate formats columns with the same indentation
// prompt:
// 1. Calculate the maximum length of the Name and JSONPath fields.
// 2. Use the maximum length to ensure both rows have the same indentation.
// 3. Build the header row and JSONPath row with proper indentation.
// 4. Ensure the first character of each column is aligned in every line.
// 5. Return the formatted content.
func BuildColumnTemplate(columns []v1.CustomResourceColumnDefinition) []byte {
	var content []byte
	var maxLen int

	// Calculate maximum lengths
	for _, col := range columns {
		if len(col.Name) > maxLen {
			maxLen = len(col.Name)
		}
		if len(col.JSONPath) > maxLen {
			maxLen = len(col.JSONPath)
		}
	}

	// Build header row with proper indentation
	for _, col := range columns {
		content = append(content, []byte(strings.ToUpper(col.Name))...)
		content = append(content, []byte(strings.Repeat(" ", maxLen-len(col.Name)+1))...)
	}
	content = append(content, []byte("\n")...)

	// Build JSONPath row with proper indentation
	for _, col := range columns {
		content = append(content, []byte(col.JSONPath)...)
		content = append(content, []byte(strings.Repeat(" ", maxLen-len(col.JSONPath)+1))...)
	}
	content = append(content, []byte("\n")...)

	return content
}

func BuildTableColumnTemplate(columns []metav1.TableColumnDefinition) []byte {
	var content []byte
	var maxLen int

	// Calculate maximum lengths
	for _, col := range columns {
		if len(col.Name) > maxLen {
			maxLen = len(col.Name)
		}
		if len(common.DefaultPrinterField) > maxLen {
			maxLen = len(common.DefaultPrinterField)
		}
	}

	// Build header row with proper indentation
	for _, col := range columns {
		content = append(content, []byte(strings.ToUpper(strings.ReplaceAll(col.Name, " ", "_")))...)
		content = append(content, []byte(strings.Repeat(" ", maxLen-len(col.Name)+1))...)
	}
	content = append(content, []byte("\n")...)

	// Build JSONPath row with proper indentation
	for _, col := range columns {
		_ = col
		content = append(content, []byte(common.DefaultPrinterField)...)
		content = append(content, []byte(strings.Repeat(" ", maxLen-len(common.DefaultPrinterField)+1))...)
	}
	content = append(content, []byte("\n")...)

	return content
}

func GenerateDirNameByGVK(gvk schema.GroupVersionKind) string {
	return strings.ToLower(fmt.Sprintf("%s-%s-%s", gvk.Kind, gvk.Group, gvk.Version))
}

// BuildYAMLColumnTemplate generates a YAML template from CRD AdditionalPrinterColumns.
func BuildYAMLColumnTemplate(columns []v1.CustomResourceColumnDefinition) ([]byte, error) {
	tmpl := models.YAMLTemplate{
		Columns: make([]models.YAMLColumn, len(columns)),
	}
	for i, col := range columns {
		tmpl.Columns[i] = models.YAMLColumn{
			Header:    strings.ToUpper(col.Name),
			FieldSpec: col.JSONPath,
		}
	}
	return yaml.Marshal(&tmpl)
}

// BuildYAMLTableColumnTemplate generates a YAML template from default k8s table column definitions.
func BuildYAMLTableColumnTemplate(columns []metav1.TableColumnDefinition) ([]byte, error) {
	tmpl := models.YAMLTemplate{
		Columns: make([]models.YAMLColumn, len(columns)),
	}
	for i, col := range columns {
		tmpl.Columns[i] = models.YAMLColumn{
			Header:    strings.ToUpper(strings.ReplaceAll(col.Name, " ", "_")),
			FieldSpec: common.DefaultPrinterField,
		}
	}
	return yaml.Marshal(&tmpl)
}

// CreateOrFormatYAMLFile creates a YAML template file or preserves the existing one.
func CreateOrFormatYAMLFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	// If file already exists, preserve user edits
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	return os.WriteFile(path, content, 0644)
}

// get template path from config.yaml
func GetTemplatePathFromConfig() (string, error) {
	config, err := LoadConfig()
	if err != nil {
		return "", err
	}
	if config.TemplatePath == "" {
		return "", errors.New("template path not found in configuration")
	}
	return config.TemplatePath, nil
}

// LoadConfig reads and parses the cwide config file.
func LoadConfig() (*models.Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %v", err)
	}

	configFile, err := os.ReadFile(filepath.Join(homeDir, common.ConfigPath))
	if err != nil {
		return nil, err
	}

	var config models.Config
	if err := yaml.Unmarshal(configFile, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func CheckFileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// ResolveTemplatePath returns the absolute template root path by checking
// the --template-path flag first, then falling back to the config file.
func ResolveTemplatePath(cmd *cobra.Command) (string, error) {
	if cmd.Flag("template-path") != nil && cmd.Flag("template-path").Changed {
		p := cmd.Flag("template-path").Value.String()
		return filepath.Abs(p)
	}
	return GetTemplatePathFromConfig()
}

// SaveConfig writes the config struct back to ~/.kubectl-cwide/config.yaml.
func SaveConfig(config *models.Config) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath := filepath.Join(homeDir, common.ConfigPath)
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return os.WriteFile(configPath, data, 0644)
}

// ResolveAlias rewrites the resource-type token in the first argument to
// its configured target. Handles both kubectl arg forms:
//
//   ["pd"]                    → ["pods"]                    (bare TYPE)
//   ["pd", "my-pod"]          → ["pods", "my-pod"]          (TYPE NAME)
//   ["pd/my-pod"]             → ["pods/my-pod"]             (TYPE/NAME)
//
// Returns the args unchanged if no alias matches or if the config cannot
// be loaded.
func ResolveAlias(args []string) []string {
	if len(args) == 0 {
		return args
	}

	config, err := LoadConfig()
	if err != nil || len(config.Aliases) == 0 {
		return args
	}

	result := make([]string, len(args))
	copy(result, args)

	// Split on the first "/" so "rr/foo" resolves "rr" without losing "/foo".
	head, rest, hasSlash := strings.Cut(args[0], "/")
	target, ok := config.Aliases[head]
	if !ok {
		return args
	}
	if hasSlash {
		result[0] = target + "/" + rest
	} else {
		result[0] = target
	}
	return result
}

// ResolveAliasString resolves a single resource-type token through the alias
// map. Accepts both bare "TYPE" and "TYPE/NAME" forms; the "/NAME" suffix is
// preserved. Returns the input unchanged if no alias matches.
func ResolveAliasString(name string) string {
	config, err := LoadConfig()
	if err != nil || len(config.Aliases) == 0 {
		return name
	}

	head, rest, hasSlash := strings.Cut(name, "/")
	target, ok := config.Aliases[head]
	if !ok {
		return name
	}
	if hasSlash {
		return target + "/" + rest
	}
	return target
}
