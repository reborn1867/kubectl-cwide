package utils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kubectl-cwide/pkg/common"
	"github.com/kubectl-cwide/pkg/models"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// create a file if not exists, create parent directories if not exists
func CreateFileIfNotExits(path string, content []byte) error {
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

// get template path from config.yaml
func GetTemplatePathFromConfig() (string, error) {

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %v", err)
	}

	// Read the configuration file
	configFile, err := os.ReadFile(filepath.Join(homeDir, common.ConfigPath))
	if err != nil {
		return "", err
	}

	// Parse the configuration file
	var config models.Config
	err = yaml.Unmarshal(configFile, &config)
	if err != nil {
		return "", err
	}

	// Check if the template path is set
	if config.TemplatePath == "" {
		return "", errors.New("template path not found in configuration")
	}

	return config.TemplatePath, nil
}

func CheckFileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}
