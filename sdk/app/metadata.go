package app

import (
	"encoding/json"
	"fmt"
	"unsafe"
)

// MetadataBuilder builds app metadata with JSON Schema.
// Inspired by Benthos ConfigSpec pattern for excellent developer experience.
type MetadataBuilder struct {
	name        string
	version     string
	author      string
	description string
	fields      []SchemaField
	cliConfig   *CLIConfig
}

// CLIConfig specifies how CLI arguments should be formatted.
type CLIConfig struct {
	FlagFormat    string            `json:"flagFormat,omitempty"`
	ArgumentOrder []string          `json:"argumentOrder,omitempty"`
	ShortFlags    map[string]string `json:"shortFlags,omitempty"`
}

// SchemaField represents a schema field definition.
type SchemaField interface {
	ToJSONSchema() map[string]interface{}
	GetName() string
	IsRequired() bool
}

// NewMetadata creates a new metadata builder.
func NewMetadata() *MetadataBuilder {
	return &MetadataBuilder{
		fields: make([]SchemaField, 0),
	}
}

// Name sets the app name.
func (b *MetadataBuilder) Name(name string) *MetadataBuilder {
	b.name = name
	return b
}

// Version sets the app version.
func (b *MetadataBuilder) Version(version string) *MetadataBuilder {
	b.version = version
	return b
}

// Author sets the app author.
func (b *MetadataBuilder) Author(author string) *MetadataBuilder {
	b.author = author
	return b
}

// Description sets the app description.
func (b *MetadataBuilder) Description(description string) *MetadataBuilder {
	b.description = description
	return b
}

// Field adds a field to the input schema.
func (b *MetadataBuilder) Field(field SchemaField) *MetadataBuilder {
	b.fields = append(b.fields, field)
	return b
}

// CLIConfig sets the CLI configuration for argument formatting.
func (b *MetadataBuilder) CLIConfig(config CLIConfig) *MetadataBuilder {
	b.cliConfig = &config
	return b
}

// Build generates the final JSON metadata string and returns a pointer to it.
// The pointer is suitable for returning from wazeos_metadata() export.
func (b *MetadataBuilder) Build() *byte {
	metadata := map[string]interface{}{
		"name":    b.name,
		"version": b.version,
		"author":  b.author,
	}

	if b.description != "" {
		metadata["description"] = b.description
	}

	// Build input schema if fields are present
	if len(b.fields) > 0 {
		schema := map[string]interface{}{
			"type":       "object",
			"properties": make(map[string]interface{}),
		}

		required := make([]string, 0)

		for _, field := range b.fields {
			fieldName := field.GetName()
			schema["properties"].(map[string]interface{})[fieldName] = field.ToJSONSchema()

			if field.IsRequired() {
				required = append(required, fieldName)
			}
		}

		if len(required) > 0 {
			schema["required"] = required
		}

		schema["additionalProperties"] = false

		// Add CLI config if present
		if b.cliConfig != nil {
			cliConfig := make(map[string]interface{})
			if b.cliConfig.FlagFormat != "" {
				cliConfig["flagFormat"] = b.cliConfig.FlagFormat
			}
			if len(b.cliConfig.ArgumentOrder) > 0 {
				cliConfig["argumentOrder"] = b.cliConfig.ArgumentOrder
			}
			if len(b.cliConfig.ShortFlags) > 0 {
				cliConfig["shortFlags"] = b.cliConfig.ShortFlags
			}
			if len(cliConfig) > 0 {
				schema["x-cli-config"] = cliConfig
			}
		}

		metadata["inputSchema"] = schema
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(metadata)
	if err != nil {
		// Fallback to minimal metadata if marshaling fails
		jsonBytes = []byte(fmt.Sprintf(`{"name":"%s","version":"%s","author":"%s"}`, b.name, b.version, b.author))
	}

	// Return pointer to first byte (C-style string)
	// The Go runtime keeps the slice alive as long as the pointer is reachable
	if len(jsonBytes) > 0 {
		return &jsonBytes[0]
	}
	return nil
}

// BuildString is a convenience method that returns the JSON string directly.
// Useful for testing, but Build() should be used for wazeos_metadata exports.
func (b *MetadataBuilder) BuildString() string {
	ptr := b.Build()
	if ptr == nil {
		return ""
	}

	// Read C-style string from pointer
	bytes := make([]byte, 0, 1024)
	p := uintptr(unsafe.Pointer(ptr))
	for i := 0; i < 100*1024; i++ { // Max 100KB
		b := *(*byte)(unsafe.Pointer(p + uintptr(i)))
		if b == 0 {
			break
		}
		bytes = append(bytes, b)
	}

	return string(bytes)
}

// StringField creates a string field.
func StringField(name string) *StringFieldBuilder {
	return &StringFieldBuilder{
		name:         name,
		fieldType:    "string",
		requiredFlag: false,
	}
}

// IntField creates an integer field.
func IntField(name string) *IntFieldBuilder {
	return &IntFieldBuilder{
		name:         name,
		fieldType:    "integer",
		requiredFlag: false,
	}
}

// BoolField creates a boolean field.
func BoolField(name string) *BoolFieldBuilder {
	return &BoolFieldBuilder{
		name:         name,
		fieldType:    "boolean",
		requiredFlag: false,
	}
}

// FloatField creates a number (float) field.
func FloatField(name string) *FloatFieldBuilder {
	return &FloatFieldBuilder{
		name:         name,
		fieldType:    "number",
		requiredFlag: false,
	}
}

// EnumField creates a string field with enum values.
func EnumField(name string, values ...string) *EnumFieldBuilder {
	return &EnumFieldBuilder{
		name:         name,
		fieldType:    "string",
		enumValues:   values,
		requiredFlag: false,
	}
}

// StringFieldBuilder builds a string field definition.
type StringFieldBuilder struct {
	name         string
	fieldType    string
	description  string
	defaultValue *string
	example      *string
	pattern      string
	minLength    *int
	maxLength    *int
	requiredFlag bool
}

func (f *StringFieldBuilder) Description(desc string) *StringFieldBuilder {
	f.description = desc
	return f
}

func (f *StringFieldBuilder) Default(val string) *StringFieldBuilder {
	f.defaultValue = &val
	return f
}

func (f *StringFieldBuilder) Example(val string) *StringFieldBuilder {
	f.example = &val
	return f
}

func (f *StringFieldBuilder) Pattern(pattern string) *StringFieldBuilder {
	f.pattern = pattern
	return f
}

func (f *StringFieldBuilder) MinLength(min int) *StringFieldBuilder {
	f.minLength = &min
	return f
}

func (f *StringFieldBuilder) MaxLength(max int) *StringFieldBuilder {
	f.maxLength = &max
	return f
}

func (f *StringFieldBuilder) Required() *StringFieldBuilder {
	f.requiredFlag = true
	return f
}

func (f *StringFieldBuilder) GetName() string {
	return f.name
}

func (f *StringFieldBuilder) IsRequired() bool {
	return f.requiredFlag
}

func (f *StringFieldBuilder) ToJSONSchema() map[string]interface{} {
	schema := map[string]interface{}{
		"type": f.fieldType,
	}

	if f.description != "" {
		schema["description"] = f.description
	}
	if f.defaultValue != nil {
		schema["default"] = *f.defaultValue
	}
	if f.example != nil {
		schema["examples"] = []string{*f.example}
	}
	if f.pattern != "" {
		schema["pattern"] = f.pattern
	}
	if f.minLength != nil {
		schema["minLength"] = *f.minLength
	}
	if f.maxLength != nil {
		schema["maxLength"] = *f.maxLength
	}

	return schema
}

// EnumFieldBuilder builds an enum field definition.
type EnumFieldBuilder struct {
	name         string
	fieldType    string
	description  string
	enumValues   []string
	defaultValue *string
	requiredFlag bool
}

func (f *EnumFieldBuilder) Description(desc string) *EnumFieldBuilder {
	f.description = desc
	return f
}

func (f *EnumFieldBuilder) Default(val string) *EnumFieldBuilder {
	f.defaultValue = &val
	return f
}

func (f *EnumFieldBuilder) Required() *EnumFieldBuilder {
	f.requiredFlag = true
	return f
}

func (f *EnumFieldBuilder) GetName() string {
	return f.name
}

func (f *EnumFieldBuilder) IsRequired() bool {
	return f.requiredFlag
}

func (f *EnumFieldBuilder) ToJSONSchema() map[string]interface{} {
	schema := map[string]interface{}{
		"type": f.fieldType,
		"enum": f.enumValues,
	}

	if f.description != "" {
		schema["description"] = f.description
	}
	if f.defaultValue != nil {
		schema["default"] = *f.defaultValue
	}

	return schema
}

// IntFieldBuilder builds an integer field definition.
type IntFieldBuilder struct {
	name         string
	fieldType    string
	description  string
	defaultValue *int
	example      *int
	minimum      *int
	maximum      *int
	requiredFlag bool
}

func (f *IntFieldBuilder) Description(desc string) *IntFieldBuilder {
	f.description = desc
	return f
}

func (f *IntFieldBuilder) Default(val int) *IntFieldBuilder {
	f.defaultValue = &val
	return f
}

func (f *IntFieldBuilder) Example(val int) *IntFieldBuilder {
	f.example = &val
	return f
}

func (f *IntFieldBuilder) Min(min int) *IntFieldBuilder {
	f.minimum = &min
	return f
}

func (f *IntFieldBuilder) Max(max int) *IntFieldBuilder {
	f.maximum = &max
	return f
}

func (f *IntFieldBuilder) Required() *IntFieldBuilder {
	f.requiredFlag = true
	return f
}

func (f *IntFieldBuilder) GetName() string {
	return f.name
}

func (f *IntFieldBuilder) IsRequired() bool {
	return f.requiredFlag
}

func (f *IntFieldBuilder) ToJSONSchema() map[string]interface{} {
	schema := map[string]interface{}{
		"type": f.fieldType,
	}

	if f.description != "" {
		schema["description"] = f.description
	}
	if f.defaultValue != nil {
		schema["default"] = *f.defaultValue
	}
	if f.example != nil {
		schema["examples"] = []int{*f.example}
	}
	if f.minimum != nil {
		schema["minimum"] = *f.minimum
	}
	if f.maximum != nil {
		schema["maximum"] = *f.maximum
	}

	return schema
}

// BoolFieldBuilder builds a boolean field definition.
type BoolFieldBuilder struct {
	name         string
	fieldType    string
	description  string
	defaultValue *bool
	requiredFlag bool
}

func (f *BoolFieldBuilder) Description(desc string) *BoolFieldBuilder {
	f.description = desc
	return f
}

func (f *BoolFieldBuilder) Default(val bool) *BoolFieldBuilder {
	f.defaultValue = &val
	return f
}

func (f *BoolFieldBuilder) Required() *BoolFieldBuilder {
	f.requiredFlag = true
	return f
}

func (f *BoolFieldBuilder) GetName() string {
	return f.name
}

func (f *BoolFieldBuilder) IsRequired() bool {
	return f.requiredFlag
}

func (f *BoolFieldBuilder) ToJSONSchema() map[string]interface{} {
	schema := map[string]interface{}{
		"type": f.fieldType,
	}

	if f.description != "" {
		schema["description"] = f.description
	}
	if f.defaultValue != nil {
		schema["default"] = *f.defaultValue
	}

	return schema
}

// FloatFieldBuilder builds a float field definition.
type FloatFieldBuilder struct {
	name         string
	fieldType    string
	description  string
	defaultValue *float64
	example      *float64
	minimum      *float64
	maximum      *float64
	requiredFlag bool
}

func (f *FloatFieldBuilder) Description(desc string) *FloatFieldBuilder {
	f.description = desc
	return f
}

func (f *FloatFieldBuilder) Default(val float64) *FloatFieldBuilder {
	f.defaultValue = &val
	return f
}

func (f *FloatFieldBuilder) Example(val float64) *FloatFieldBuilder {
	f.example = &val
	return f
}

func (f *FloatFieldBuilder) Min(min float64) *FloatFieldBuilder {
	f.minimum = &min
	return f
}

func (f *FloatFieldBuilder) Max(max float64) *FloatFieldBuilder {
	f.maximum = &max
	return f
}

func (f *FloatFieldBuilder) Required() *FloatFieldBuilder {
	f.requiredFlag = true
	return f
}

func (f *FloatFieldBuilder) GetName() string {
	return f.name
}

func (f *FloatFieldBuilder) IsRequired() bool {
	return f.requiredFlag
}

func (f *FloatFieldBuilder) ToJSONSchema() map[string]interface{} {
	schema := map[string]interface{}{
		"type": f.fieldType,
	}

	if f.description != "" {
		schema["description"] = f.description
	}
	if f.defaultValue != nil {
		schema["default"] = *f.defaultValue
	}
	if f.example != nil {
		schema["examples"] = []float64{*f.example}
	}
	if f.minimum != nil {
		schema["minimum"] = *f.minimum
	}
	if f.maximum != nil {
		schema["maximum"] = *f.maximum
	}

	return schema
}
