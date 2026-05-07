package apps

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var (
	buildFile   string
	buildOutput string
)

var buildCmd = &cobra.Command{
	Use:   "build [directory]",
	Short: "Build a WASM app with automatic schema generation",
	Long: `Build a WazeOS WASM application with automatic schema generation.

This command:
1. Scans your Go code for the Input struct
2. Generates JSON Schema from struct tags
3. Updates metadata.json with the schema
4. Compiles to WASM using TinyGo

Examples:
  # Build app in current directory
  wazeos apps build .

  # Build app in specific directory
  wazeos apps build bin/mycompany/myapp

  # Specify custom input file
  wazeos apps build . --file handler.go`,
	Args: cobra.MaximumNArgs(1),
	Run:  runBuild,
}

func init() {
	buildCmd.Flags().StringVarP(&buildFile, "file", "f", "main.go", "Go source file to scan for Input struct")
	buildCmd.Flags().StringVarP(&buildOutput, "output", "o", "app.wasm", "Output WASM file name")
}

func runBuild(cmd *cobra.Command, args []string) {
	// Determine directory
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	// Make absolute path
	absDir, err := filepath.Abs(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Building app in %s\n\n", absDir)

	// Step 1: Find and validate files
	mainFile := filepath.Join(absDir, buildFile)
	metadataFile := filepath.Join(absDir, "metadata.json")

	if _, err := os.Stat(mainFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: %s not found in %s\n", buildFile, absDir)
		os.Exit(1)
	}

	if _, err := os.Stat(metadataFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: metadata.json not found in %s\n", absDir)
		fmt.Fprintf(os.Stderr, "Run 'wazeos apps new' to create a new app project\n")
		os.Exit(1)
	}

	// Step 2: Extract schema from Input struct
	fmt.Println("→ Extracting schema from Input struct...")
	schema, err := extractSchemaFromFile(mainFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error extracting schema: %v\n", err)
		os.Exit(1)
	}

	if schema != nil {
		fmt.Printf("  ✓ Found Input struct with %d field(s)\n", len(schema["properties"].(map[string]interface{})))
	} else {
		fmt.Println("  ℹ No Input struct found (empty input schema)")
	}

	// Step 3: Update metadata.json
	fmt.Println("\n→ Updating metadata.json...")
	if err := updateMetadata(metadataFile, schema); err != nil {
		fmt.Fprintf(os.Stderr, "Error updating metadata: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ Updated metadata.json with schema")

	// Step 4: Check TinyGo
	fmt.Println("\n→ Checking TinyGo installation...")
	if err := checkTinyGo(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Install TinyGo: https://tinygo.org/getting-started/install/\n")
		os.Exit(1)
	}
	fmt.Println("  ✓ TinyGo found")

	// Step 5: Build WASM
	fmt.Println("\n→ Compiling to WASM...")
	outputPath := filepath.Join(absDir, buildOutput)
	if err := buildWASM(mainFile, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error building WASM: %v\n", err)
		os.Exit(1)
	}

	// Get file size
	info, _ := os.Stat(outputPath)
	sizeKB := float64(info.Size()) / 1024.0

	fmt.Printf("  ✓ Built %s (%.1f KB)\n", buildOutput, sizeKB)
	fmt.Printf("\n✓ Build complete!\n\n")
	fmt.Println("Next steps:")
	fmt.Printf("  wazeos apps package %s  # Create ZIP package\n", absDir)
	fmt.Printf("  wazeos apps install %s  # Build, package, and install\n", absDir)
}

// extractSchemaFromFile parses a Go file and extracts JSON Schema from Input struct
func extractSchemaFromFile(filename string) (map[string]interface{}, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Go file: %w", err)
	}

	// Find the Input struct
	var inputStruct *ast.StructType
	ast.Inspect(node, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		// Look for type named "Input"
		if typeSpec.Name.Name == "Input" {
			if structType, ok := typeSpec.Type.(*ast.StructType); ok {
				inputStruct = structType
				return false
			}
		}
		return true
	})

	// If no Input struct found, return nil (no schema needed)
	if inputStruct == nil {
		return nil, nil
	}

	// Build JSON Schema from struct fields
	properties := make(map[string]interface{})
	required := []string{}

	for _, field := range inputStruct.Fields.List {
		if len(field.Names) == 0 {
			continue // Skip embedded fields
		}

		fieldName := field.Names[0].Name
		if !ast.IsExported(fieldName) {
			continue // Skip private fields
		}

		// Parse struct tag
		var tag string
		if field.Tag != nil {
			tag = field.Tag.Value
			tag = strings.Trim(tag, "`")
		}

		// Extract JSON field name and options
		jsonName, omitempty := parseJSONTag(tag)
		if jsonName == "" {
			jsonName = fieldName
		}
		if jsonName == "-" {
			continue // Skip fields with json:"-"
		}

		// Determine if required (no omitempty and no default)
		defaultValue := parseTag(tag, "default")
		isRequired := !omitempty && defaultValue == ""

		// Build field schema
		fieldSchema := buildFieldSchema(field.Type, tag)

		properties[jsonName] = fieldSchema

		if isRequired {
			required = append(required, jsonName)
		}
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema, nil
}

// parseJSONTag extracts the field name and omitempty from json tag
func parseJSONTag(tag string) (name string, omitempty bool) {
	jsonTag := parseTag(tag, "json")
	if jsonTag == "" {
		return "", false
	}

	parts := strings.Split(jsonTag, ",")
	name = parts[0]
	for _, opt := range parts[1:] {
		if opt == "omitempty" {
			omitempty = true
		}
	}
	return name, omitempty
}

// parseTag extracts a specific tag value
func parseTag(tag, key string) string {
	fields := reflect.StructTag(tag)
	return fields.Get(key)
}

// buildFieldSchema creates JSON Schema for a field
func buildFieldSchema(fieldType ast.Expr, tag string) map[string]interface{} {
	schema := make(map[string]interface{})

	// Determine JSON Schema type from Go type
	var jsonType string
	switch t := fieldType.(type) {
	case *ast.Ident:
		switch t.Name {
		case "string":
			jsonType = "string"
		case "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64":
			jsonType = "integer"
		case "float32", "float64":
			jsonType = "number"
		case "bool":
			jsonType = "boolean"
		default:
			jsonType = "object"
		}
	case *ast.ArrayType:
		jsonType = "array"
		// TODO: Add items schema for arrays
	case *ast.MapType:
		jsonType = "object"
	default:
		jsonType = "object"
	}

	schema["type"] = jsonType

	// Add description
	if desc := parseTag(tag, "description"); desc != "" {
		schema["description"] = desc
	}

	// Add default value
	if defaultVal := parseTag(tag, "default"); defaultVal != "" {
		schema["default"] = parseDefaultValue(defaultVal, jsonType)
	}

	// Add example
	if example := parseTag(tag, "example"); example != "" {
		schema["examples"] = []string{example}
	}

	// Add validation constraints
	if jsonType == "string" {
		if pattern := parseTag(tag, "pattern"); pattern != "" {
			schema["pattern"] = pattern
		}
		if minLen := parseTag(tag, "minLength"); minLen != "" {
			if val, err := strconv.Atoi(minLen); err == nil {
				schema["minLength"] = val
			}
		}
		if maxLen := parseTag(tag, "maxLength"); maxLen != "" {
			if val, err := strconv.Atoi(maxLen); err == nil {
				schema["maxLength"] = val
			}
		}
		if enum := parseTag(tag, "enum"); enum != "" {
			schema["enum"] = strings.Split(enum, ",")
		}
	}

	if jsonType == "integer" || jsonType == "number" {
		if min := parseTag(tag, "min"); min != "" {
			if val, err := strconv.ParseFloat(min, 64); err == nil {
				if jsonType == "integer" {
					schema["minimum"] = int(val)
				} else {
					schema["minimum"] = val
				}
			}
		}
		if max := parseTag(tag, "max"); max != "" {
			if val, err := strconv.ParseFloat(max, 64); err == nil {
				if jsonType == "integer" {
					schema["maximum"] = int(val)
				} else {
					schema["maximum"] = val
				}
			}
		}
	}

	return schema
}

// parseDefaultValue converts string default to appropriate type
func parseDefaultValue(val string, jsonType string) interface{} {
	switch jsonType {
	case "integer":
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	case "number":
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	case "boolean":
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return val
}

// updateMetadata reads metadata.json, updates inputSchema, and writes back
func updateMetadata(filename string, schema map[string]interface{}) error {
	// Read existing metadata
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read metadata.json: %w", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("failed to parse metadata.json: %w", err)
	}

	// Update inputSchema
	if schema != nil {
		metadata["inputSchema"] = schema
	} else {
		delete(metadata, "inputSchema")
	}

	// Write back with pretty formatting
	output, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(filename, output, 0644); err != nil {
		return fmt.Errorf("failed to write metadata.json: %w", err)
	}

	return nil
}

// checkTinyGo verifies TinyGo is installed
func checkTinyGo() error {
	cmd := exec.Command("tinygo", "version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("TinyGo not found")
	}
	return nil
}

// buildWASM compiles Go code to WASM using TinyGo
func buildWASM(inputFile, outputFile string) error {
	cmd := exec.Command("tinygo", "build",
		"-o", outputFile,
		"-target=wasi",
		inputFile)

	// Set working directory to the app directory so go.mod is found
	cmd.Dir = filepath.Dir(inputFile)

	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compilation failed:\n%s", string(output))
	}

	return nil
}
