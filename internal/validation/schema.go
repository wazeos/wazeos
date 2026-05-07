package validation

import (
	"encoding/json"
	"fmt"

	"github.com/xeipuuv/gojsonschema"
)

// SchemaValidator validates data against a JSON Schema.
type SchemaValidator struct {
	schema *gojsonschema.Schema
}

// NewSchemaValidator creates a new schema validator from a JSON Schema.
func NewSchemaValidator(schemaJSON []byte) (*SchemaValidator, error) {
	schemaLoader := gojsonschema.NewBytesLoader(schemaJSON)
	schema, err := gojsonschema.NewSchema(schemaLoader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}
	return &SchemaValidator{schema: schema}, nil
}

// Validate validates data against the schema.
func (v *SchemaValidator) Validate(data map[string]interface{}) error {
	documentLoader := gojsonschema.NewGoLoader(data)
	result, err := v.schema.Validate(documentLoader)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	if !result.Valid() {
		var errors []string
		for _, err := range result.Errors() {
			errors = append(errors, err.String())
		}
		return fmt.Errorf("validation failed: %v", errors)
	}

	return nil
}

// ValidateJSONSchema validates that a schema is valid JSON Schema (Draft 7).
func ValidateJSONSchema(schemaData []byte) error {
	var schema map[string]interface{}
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	// Validate against JSON Schema meta-schema
	metaSchema := gojsonschema.NewReferenceLoader("http://json-schema.org/draft-07/schema#")
	schemaLoader := gojsonschema.NewGoLoader(schema)

	result, err := gojsonschema.Validate(metaSchema, schemaLoader)
	if err != nil {
		return fmt.Errorf("failed to validate schema: %w", err)
	}

	if !result.Valid() {
		var errors []string
		for _, err := range result.Errors() {
			errors = append(errors, err.String())
		}
		return fmt.Errorf("invalid schema: %v", errors)
	}

	return nil
}
