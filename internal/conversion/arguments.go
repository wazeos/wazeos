package conversion

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// CLIConfig holds configuration for converting MCP arguments to CLI flags.
type CLIConfig struct {
	// FlagFormat specifies how flags should be formatted.
	// Supports placeholders: {key}, {value}
	// Examples: "--{key}={value}", "-{key} {value}", "{key}={value}"
	FlagFormat string `json:"flagFormat"`

	// ArgumentOrder specifies the order for positional arguments.
	// If specified, arguments will be positional instead of flags.
	ArgumentOrder []string `json:"argumentOrder"`

	// ShortFlags maps long flag names to short versions.
	// Example: {"path": "p", "format": "f"}
	ShortFlags map[string]string `json:"shortFlags"`
}

// DefaultCLIConfig returns the default CLI configuration.
func DefaultCLIConfig() CLIConfig {
	return CLIConfig{
		FlagFormat: "--{key}={value}",
	}
}

// ConvertToCLIArgs converts MCP key-value arguments to CLI argument strings.
// Uses the provided CLIConfig to determine flag format.
func ConvertToCLIArgs(args map[string]interface{}, config CLIConfig) ([]string, error) {
	if len(args) == 0 {
		return []string{}, nil
	}

	// Use default config if not specified
	if config.FlagFormat == "" && len(config.ArgumentOrder) == 0 {
		config = DefaultCLIConfig()
	}

	// If argumentOrder is specified, use positional arguments
	if len(config.ArgumentOrder) > 0 {
		return convertToPositionalArgs(args, config.ArgumentOrder)
	}

	// Otherwise, use flag format
	return convertToFlagArgs(args, config)
}

// convertToPositionalArgs converts to positional arguments in specified order.
func convertToPositionalArgs(args map[string]interface{}, order []string) ([]string, error) {
	result := make([]string, 0, len(order))

	for _, key := range order {
		value, ok := args[key]
		if !ok {
			// Skip missing optional arguments
			continue
		}

		valueStr, err := formatValue(value)
		if err != nil {
			return nil, fmt.Errorf("failed to format value for %s: %w", key, err)
		}

		result = append(result, valueStr)
	}

	return result, nil
}

// convertToFlagArgs converts to flag-style arguments.
func convertToFlagArgs(args map[string]interface{}, config CLIConfig) ([]string, error) {
	// Sort keys for deterministic output
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make([]string, 0, len(keys))

	for _, key := range keys {
		value := args[key]

		// Format the value
		valueStr, err := formatValue(value)
		if err != nil {
			return nil, fmt.Errorf("failed to format value for %s: %w", key, err)
		}

		// Use short flag if available
		flagName := key
		if shortFlag, ok := config.ShortFlags[key]; ok && shortFlag != "" {
			flagName = shortFlag
		}

		// Apply flag format
		flag := formatFlag(flagName, valueStr, config.FlagFormat)
		result = append(result, flag)
	}

	return result, nil
}

// formatFlag formats a flag according to the format string.
// Supports: --{key}={value}, -{key} {value}, {key}={value}, etc.
func formatFlag(key, value, format string) string {
	// Replace placeholders
	result := strings.ReplaceAll(format, "{key}", key)
	result = strings.ReplaceAll(result, "{value}", value)

	// Check if format includes space (means value is separate argument)
	if strings.Contains(result, " ") {
		// This will be split by shell, so return as is
		return result
	}

	return result
}

// formatValue converts a value to a string suitable for CLI arguments.
// Complex types (objects, arrays) are JSON-encoded.
func formatValue(value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil

	case bool:
		if v {
			return "true", nil
		}
		return "false", nil

	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v), nil

	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v), nil

	case float32, float64:
		return fmt.Sprintf("%g", v), nil

	case map[string]interface{}, []interface{}:
		// Complex types: JSON-encode
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to JSON-encode complex value: %w", err)
		}
		return string(jsonBytes), nil

	case nil:
		return "", nil

	default:
		// Fallback: try JSON encoding
		jsonBytes, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v), nil
		}
		return string(jsonBytes), nil
	}
}

// ExtractCLIConfig extracts CLI configuration from a JSON Schema.
// Looks for x-cli-config extension in the schema.
func ExtractCLIConfig(schema map[string]interface{}) CLIConfig {
	config := DefaultCLIConfig()

	// Look for x-cli-config extension
	if cliConfig, ok := schema["x-cli-config"].(map[string]interface{}); ok {
		// Extract flagFormat
		if flagFormat, ok := cliConfig["flagFormat"].(string); ok {
			config.FlagFormat = flagFormat
		}

		// Extract argumentOrder
		if argOrder, ok := cliConfig["argumentOrder"].([]interface{}); ok {
			config.ArgumentOrder = make([]string, 0, len(argOrder))
			for _, arg := range argOrder {
				if argStr, ok := arg.(string); ok {
					config.ArgumentOrder = append(config.ArgumentOrder, argStr)
				}
			}
		}

		// Extract shortFlags
		if shortFlags, ok := cliConfig["shortFlags"].(map[string]interface{}); ok {
			config.ShortFlags = make(map[string]string)
			for key, val := range shortFlags {
				if valStr, ok := val.(string); ok {
					config.ShortFlags[key] = valStr
				}
			}
		}
	}

	return config
}
