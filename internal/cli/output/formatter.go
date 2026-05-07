package output

import (
	"encoding/json"
	"fmt"

	"github.com/wazeos/wazeos/internal/types"
	"gopkg.in/yaml.v3"
)

// Format represents an output format type
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
)

// Formatter provides methods to format various data types for output
type Formatter interface {
	FormatPackageList(apps []*types.AppMetadata) (string, error)
	FormatPackageDetails(app *types.AppMetadata) (string, error)
	FormatSecretList(keys []string) (string, error)
	FormatError(err error) string
	FormatSuccess(message string) string
}

// NewFormatter creates a formatter of the specified type
func NewFormatter(format Format, noColor bool) Formatter {
	switch format {
	case FormatJSON:
		return &JSONFormatter{}
	case FormatYAML:
		return &YAMLFormatter{}
	case FormatTable:
		fallthrough
	default:
		return &TableFormatter{NoColor: noColor}
	}
}

// Common helper for structured output
type structuredOutput struct {
	Success bool        `json:"success" yaml:"success"`
	Data    interface{} `json:"data,omitempty" yaml:"data,omitempty"`
	Error   string      `json:"error,omitempty" yaml:"error,omitempty"`
	Message string      `json:"message,omitempty" yaml:"message,omitempty"`
}

// JSONFormatter formats output as JSON
type JSONFormatter struct{}

func (f *JSONFormatter) FormatPackageList(apps []*types.AppMetadata) (string, error) {
	output := structuredOutput{
		Success: true,
		Data: map[string]interface{}{
			"packages": apps,
			"count":    len(apps),
		},
	}
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *JSONFormatter) FormatPackageDetails(app *types.AppMetadata) (string, error) {
	output := structuredOutput{
		Success: true,
		Data:    app,
	}
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *JSONFormatter) FormatSecretList(keys []string) (string, error) {
	output := structuredOutput{
		Success: true,
		Data: map[string]interface{}{
			"keys":  keys,
			"count": len(keys),
		},
	}
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *JSONFormatter) FormatError(err error) string {
	output := structuredOutput{
		Success: false,
		Error:   err.Error(),
	}
	data, _ := json.MarshalIndent(output, "", "  ")
	return string(data)
}

func (f *JSONFormatter) FormatSuccess(message string) string {
	output := structuredOutput{
		Success: true,
		Message: message,
	}
	data, _ := json.MarshalIndent(output, "", "  ")
	return string(data)
}

// YAMLFormatter formats output as YAML
type YAMLFormatter struct{}

func (f *YAMLFormatter) FormatPackageList(apps []*types.AppMetadata) (string, error) {
	output := structuredOutput{
		Success: true,
		Data: map[string]interface{}{
			"packages": apps,
			"count":    len(apps),
		},
	}
	data, err := yaml.Marshal(output)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *YAMLFormatter) FormatPackageDetails(app *types.AppMetadata) (string, error) {
	output := structuredOutput{
		Success: true,
		Data:    app,
	}
	data, err := yaml.Marshal(output)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *YAMLFormatter) FormatSecretList(keys []string) (string, error) {
	output := structuredOutput{
		Success: true,
		Data: map[string]interface{}{
			"keys":  keys,
			"count": len(keys),
		},
	}
	data, err := yaml.Marshal(output)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *YAMLFormatter) FormatError(err error) string {
	output := structuredOutput{
		Success: false,
		Error:   err.Error(),
	}
	data, _ := yaml.Marshal(output)
	return string(data)
}

func (f *YAMLFormatter) FormatSuccess(message string) string {
	output := structuredOutput{
		Success: true,
		Message: message,
	}
	data, _ := yaml.Marshal(output)
	return string(data)
}

// ParseFormat converts a string to a Format type
func ParseFormat(s string) Format {
	switch s {
	case "json":
		return FormatJSON
	case "yaml", "yml":
		return FormatYAML
	case "table":
		return FormatTable
	default:
		return FormatTable
	}
}

// FormatSimpleMessage formats a simple message based on format type
func FormatSimpleMessage(format Format, message string) string {
	switch format {
	case FormatJSON:
		data, _ := json.Marshal(map[string]string{"message": message})
		return string(data)
	case FormatYAML:
		return fmt.Sprintf("message: %s\n", message)
	default:
		return message
	}
}
