package utils

import (
	"os"
	"strings"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// create a temp directory if not exists
func CreateTempDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.Mkdir(path, 0755); err != nil {
			return err
		}
	}
	return nil
}

// create or update file in given path
func CreateOrUpdateFile(path string, content []byte) error {
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
		content = append(content, []byte(col.Name)...)
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
