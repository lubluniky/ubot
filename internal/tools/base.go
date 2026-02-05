// Package tools provides the interface and utilities for agent tools.
package tools

import (
	"context"
	"fmt"
	"reflect"
)

// Tool defines the interface for agent tools.
type Tool interface {
	// Name returns the tool's identifier.
	Name() string
	// Description returns human-readable description for LLM.
	Description() string
	// Parameters returns JSON Schema for tool parameters.
	Parameters() map[string]interface{}
	// Execute runs the tool with given parameters.
	Execute(ctx context.Context, params map[string]interface{}) (string, error)
}

// ToolDefinition represents a tool in OpenAI function calling format.
type ToolDefinition struct {
	Type     string             `json:"type"` // "function"
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition represents a function definition for OpenAI API.
type FunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToDefinition converts a Tool to OpenAI function calling format.
func ToDefinition(t Tool) ToolDefinition {
	return ToolDefinition{
		Type: "function",
		Function: FunctionDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		},
	}
}

// ValidateParams validates parameters against a JSON schema.
// Returns a list of error messages (empty if valid).
func ValidateParams(params map[string]interface{}, schema map[string]interface{}) []string {
	var errors []string

	// Check required fields
	if required, ok := schema["required"].([]interface{}); ok {
		for _, r := range required {
			if reqField, ok := r.(string); ok {
				if _, exists := params[reqField]; !exists {
					errors = append(errors, fmt.Sprintf("missing required field: %s", reqField))
				}
			}
		}
	}
	// Also handle []string type for required (common in Go)
	if required, ok := schema["required"].([]string); ok {
		for _, reqField := range required {
			if _, exists := params[reqField]; !exists {
				errors = append(errors, fmt.Sprintf("missing required field: %s", reqField))
			}
		}
	}

	// Get properties schema
	properties, hasProps := schema["properties"].(map[string]interface{})
	if !hasProps {
		return errors
	}

	// Validate each parameter
	for key, value := range params {
		propSchema, exists := properties[key]
		if !exists {
			continue // Allow additional properties by default
		}

		propSchemaMap, ok := propSchema.(map[string]interface{})
		if !ok {
			continue
		}

		fieldErrors := validateField(key, value, propSchemaMap)
		errors = append(errors, fieldErrors...)
	}

	return errors
}

// validateField validates a single field against its schema.
func validateField(key string, value interface{}, schema map[string]interface{}) []string {
	var errors []string

	// Get expected type
	expectedType, hasType := schema["type"].(string)
	if !hasType {
		return errors
	}

	// Type validation
	typeError := validateType(key, value, expectedType)
	if typeError != "" {
		errors = append(errors, typeError)
		return errors // Return early if type doesn't match
	}

	// Type-specific validations
	switch expectedType {
	case "string":
		strVal, _ := value.(string)
		errors = append(errors, validateString(key, strVal, schema)...)
	case "integer", "number":
		numVal := toFloat64(value)
		errors = append(errors, validateNumber(key, numVal, schema)...)
	case "array":
		arrVal, _ := value.([]interface{})
		errors = append(errors, validateArray(key, arrVal, schema)...)
	case "object":
		objVal, _ := value.(map[string]interface{})
		errors = append(errors, validateObject(key, objVal, schema)...)
	}

	return errors
}

// validateType checks if the value matches the expected JSON schema type.
func validateType(key string, value interface{}, expectedType string) string {
	if value == nil {
		return "" // nil is valid for any type (unless required, which is checked separately)
	}

	valid := false
	switch expectedType {
	case "string":
		_, valid = value.(string)
	case "integer":
		valid = isInteger(value)
	case "number":
		valid = isNumber(value)
	case "boolean":
		_, valid = value.(bool)
	case "array":
		valid = isArray(value)
	case "object":
		_, valid = value.(map[string]interface{})
	case "null":
		valid = value == nil
	}

	if !valid {
		return fmt.Sprintf("field %s: expected type %s, got %s", key, expectedType, reflect.TypeOf(value))
	}
	return ""
}

// isInteger checks if a value is an integer type.
func isInteger(value interface{}) bool {
	switch v := value.(type) {
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	case float64:
		return v == float64(int64(v))
	case float32:
		return v == float32(int32(v))
	}
	return false
}

// isNumber checks if a value is a number type.
func isNumber(value interface{}) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	case float32, float64:
		return true
	}
	return false
}

// isArray checks if a value is an array/slice type.
func isArray(value interface{}) bool {
	if value == nil {
		return false
	}
	v := reflect.ValueOf(value)
	return v.Kind() == reflect.Slice || v.Kind() == reflect.Array
}

// toFloat64 converts a numeric value to float64.
func toFloat64(value interface{}) float64 {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int8:
		return float64(v)
	case int16:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case uint:
		return float64(v)
	case uint8:
		return float64(v)
	case uint16:
		return float64(v)
	case uint32:
		return float64(v)
	case uint64:
		return float64(v)
	case float32:
		return float64(v)
	case float64:
		return v
	}
	return 0
}

// validateString validates string-specific constraints.
func validateString(key, value string, schema map[string]interface{}) []string {
	var errors []string

	// Check minLength
	if minLen, ok := schema["minLength"]; ok {
		minLenInt := int(toFloat64(minLen))
		if len(value) < minLenInt {
			errors = append(errors, fmt.Sprintf("field %s: length %d is less than minimum %d", key, len(value), minLenInt))
		}
	}

	// Check maxLength
	if maxLen, ok := schema["maxLength"]; ok {
		maxLenInt := int(toFloat64(maxLen))
		if len(value) > maxLenInt {
			errors = append(errors, fmt.Sprintf("field %s: length %d exceeds maximum %d", key, len(value), maxLenInt))
		}
	}

	// Check enum
	if enum, ok := schema["enum"]; ok {
		if !isInEnum(value, enum) {
			errors = append(errors, fmt.Sprintf("field %s: value %q is not in allowed enum values", key, value))
		}
	}

	return errors
}

// validateNumber validates number-specific constraints.
func validateNumber(key string, value float64, schema map[string]interface{}) []string {
	var errors []string

	// Check minimum
	if min, ok := schema["minimum"]; ok {
		minVal := toFloat64(min)
		if value < minVal {
			errors = append(errors, fmt.Sprintf("field %s: value %v is less than minimum %v", key, value, minVal))
		}
	}

	// Check maximum
	if max, ok := schema["maximum"]; ok {
		maxVal := toFloat64(max)
		if value > maxVal {
			errors = append(errors, fmt.Sprintf("field %s: value %v exceeds maximum %v", key, value, maxVal))
		}
	}

	// Check exclusiveMinimum
	if min, ok := schema["exclusiveMinimum"]; ok {
		minVal := toFloat64(min)
		if value <= minVal {
			errors = append(errors, fmt.Sprintf("field %s: value %v must be greater than %v", key, value, minVal))
		}
	}

	// Check exclusiveMaximum
	if max, ok := schema["exclusiveMaximum"]; ok {
		maxVal := toFloat64(max)
		if value >= maxVal {
			errors = append(errors, fmt.Sprintf("field %s: value %v must be less than %v", key, value, maxVal))
		}
	}

	// Check enum
	if enum, ok := schema["enum"]; ok {
		if !isInEnum(value, enum) {
			errors = append(errors, fmt.Sprintf("field %s: value %v is not in allowed enum values", key, value))
		}
	}

	return errors
}

// validateArray validates array-specific constraints.
func validateArray(key string, value []interface{}, schema map[string]interface{}) []string {
	var errors []string

	// Check minItems
	if minItems, ok := schema["minItems"]; ok {
		minItemsInt := int(toFloat64(minItems))
		if len(value) < minItemsInt {
			errors = append(errors, fmt.Sprintf("field %s: array length %d is less than minimum %d items", key, len(value), minItemsInt))
		}
	}

	// Check maxItems
	if maxItems, ok := schema["maxItems"]; ok {
		maxItemsInt := int(toFloat64(maxItems))
		if len(value) > maxItemsInt {
			errors = append(errors, fmt.Sprintf("field %s: array length %d exceeds maximum %d items", key, len(value), maxItemsInt))
		}
	}

	// Validate items schema
	if itemsSchema, ok := schema["items"].(map[string]interface{}); ok {
		for i, item := range value {
			itemKey := fmt.Sprintf("%s[%d]", key, i)
			itemErrors := validateField(itemKey, item, itemsSchema)
			errors = append(errors, itemErrors...)
		}
	}

	return errors
}

// validateObject validates object-specific constraints (nested objects).
func validateObject(key string, value map[string]interface{}, schema map[string]interface{}) []string {
	var errors []string

	// Get nested properties schema
	if properties, ok := schema["properties"].(map[string]interface{}); ok {
		// Check required fields within the nested object
		if required, ok := schema["required"].([]interface{}); ok {
			for _, r := range required {
				if reqField, ok := r.(string); ok {
					if _, exists := value[reqField]; !exists {
						errors = append(errors, fmt.Sprintf("field %s: missing required field %s", key, reqField))
					}
				}
			}
		}
		if required, ok := schema["required"].([]string); ok {
			for _, reqField := range required {
				if _, exists := value[reqField]; !exists {
					errors = append(errors, fmt.Sprintf("field %s: missing required field %s", key, reqField))
				}
			}
		}

		// Validate each property in the nested object
		for propKey, propValue := range value {
			propSchema, exists := properties[propKey]
			if !exists {
				continue
			}

			propSchemaMap, ok := propSchema.(map[string]interface{})
			if !ok {
				continue
			}

			nestedKey := fmt.Sprintf("%s.%s", key, propKey)
			propErrors := validateField(nestedKey, propValue, propSchemaMap)
			errors = append(errors, propErrors...)
		}
	}

	return errors
}

// isInEnum checks if a value is in the enum list.
func isInEnum(value interface{}, enum interface{}) bool {
	switch e := enum.(type) {
	case []interface{}:
		for _, allowed := range e {
			if reflect.DeepEqual(value, allowed) {
				return true
			}
		}
	case []string:
		strVal, ok := value.(string)
		if ok {
			for _, allowed := range e {
				if strVal == allowed {
					return true
				}
			}
		}
	}
	return false
}

// BaseTool provides common functionality for tools.
type BaseTool struct {
	name        string
	description string
	parameters  map[string]interface{}
}

// NewBaseTool creates a new BaseTool with the given attributes.
func NewBaseTool(name, description string, parameters map[string]interface{}) BaseTool {
	return BaseTool{
		name:        name,
		description: description,
		parameters:  parameters,
	}
}

// Name returns the tool's identifier.
func (t *BaseTool) Name() string {
	return t.name
}

// Description returns human-readable description for LLM.
func (t *BaseTool) Description() string {
	return t.description
}

// Parameters returns JSON Schema for tool parameters.
func (t *BaseTool) Parameters() map[string]interface{} {
	return t.parameters
}

// ErrParamNotFound is returned when a required parameter is missing.
type ErrParamNotFound struct {
	Key string
}

func (e ErrParamNotFound) Error() string {
	return fmt.Sprintf("parameter %q not found", e.Key)
}

// ErrParamTypeMismatch is returned when a parameter has an unexpected type.
type ErrParamTypeMismatch struct {
	Key      string
	Expected string
	Actual   interface{}
}

func (e ErrParamTypeMismatch) Error() string {
	return fmt.Sprintf("parameter %q: expected %s, got %T", e.Key, e.Expected, e.Actual)
}

// GetStringParam extracts a string parameter from the params map.
// Returns ErrParamNotFound if the key doesn't exist.
// Returns ErrParamTypeMismatch if the value is not a string.
func GetStringParam(params map[string]interface{}, key string) (string, error) {
	val, ok := params[key]
	if !ok {
		return "", ErrParamNotFound{Key: key}
	}
	str, ok := val.(string)
	if !ok {
		return "", ErrParamTypeMismatch{Key: key, Expected: "string", Actual: val}
	}
	return str, nil
}

// GetIntParam extracts an integer parameter from the params map.
// Returns ErrParamNotFound if the key doesn't exist.
// Returns ErrParamTypeMismatch if the value is not an integer.
// Note: JSON numbers are typically decoded as float64, so this handles that case.
func GetIntParam(params map[string]interface{}, key string) (int, error) {
	val, ok := params[key]
	if !ok {
		return 0, ErrParamNotFound{Key: key}
	}
	switch v := val.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		// JSON numbers are decoded as float64
		return int(v), nil
	default:
		return 0, ErrParamTypeMismatch{Key: key, Expected: "int", Actual: val}
	}
}

// GetBoolParam extracts a boolean parameter from the params map.
// Returns ErrParamNotFound if the key doesn't exist.
// Returns ErrParamTypeMismatch if the value is not a boolean.
func GetBoolParam(params map[string]interface{}, key string) (bool, error) {
	val, ok := params[key]
	if !ok {
		return false, ErrParamNotFound{Key: key}
	}
	b, ok := val.(bool)
	if !ok {
		return false, ErrParamTypeMismatch{Key: key, Expected: "bool", Actual: val}
	}
	return b, nil
}

// GetStringParamOr extracts a string parameter from the params map,
// returning the default value if the key doesn't exist or the value is not a string.
func GetStringParamOr(params map[string]interface{}, key, defaultVal string) string {
	val, err := GetStringParam(params, key)
	if err != nil {
		return defaultVal
	}
	return val
}

// GetIntParamOr extracts an integer parameter from the params map,
// returning the default value if the key doesn't exist or the value is not an integer.
func GetIntParamOr(params map[string]interface{}, key string, defaultVal int) int {
	val, err := GetIntParam(params, key)
	if err != nil {
		return defaultVal
	}
	return val
}

// GetBoolParamOr extracts a boolean parameter from the params map,
// returning the default value if the key doesn't exist or the value is not a boolean.
func GetBoolParamOr(params map[string]interface{}, key string, defaultVal bool) bool {
	val, err := GetBoolParam(params, key)
	if err != nil {
		return defaultVal
	}
	return val
}

// GetFloatParam extracts a float64 parameter from the params map.
// Returns ErrParamNotFound if the key doesn't exist.
// Returns ErrParamTypeMismatch if the value is not a number.
func GetFloatParam(params map[string]interface{}, key string) (float64, error) {
	val, ok := params[key]
	if !ok {
		return 0, ErrParamNotFound{Key: key}
	}
	switch v := val.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	default:
		return 0, ErrParamTypeMismatch{Key: key, Expected: "float64", Actual: val}
	}
}

// GetFloatParamOr extracts a float64 parameter from the params map,
// returning the default value if the key doesn't exist or the value is not a number.
func GetFloatParamOr(params map[string]interface{}, key string, defaultVal float64) float64 {
	val, err := GetFloatParam(params, key)
	if err != nil {
		return defaultVal
	}
	return val
}

// GetSliceParam extracts a slice parameter from the params map.
// Returns ErrParamNotFound if the key doesn't exist.
// Returns ErrParamTypeMismatch if the value is not a slice.
func GetSliceParam(params map[string]interface{}, key string) ([]interface{}, error) {
	val, ok := params[key]
	if !ok {
		return nil, ErrParamNotFound{Key: key}
	}
	slice, ok := val.([]interface{})
	if !ok {
		return nil, ErrParamTypeMismatch{Key: key, Expected: "[]interface{}", Actual: val}
	}
	return slice, nil
}

// GetMapParam extracts a map parameter from the params map.
// Returns ErrParamNotFound if the key doesn't exist.
// Returns ErrParamTypeMismatch if the value is not a map.
func GetMapParam(params map[string]interface{}, key string) (map[string]interface{}, error) {
	val, ok := params[key]
	if !ok {
		return nil, ErrParamNotFound{Key: key}
	}
	m, ok := val.(map[string]interface{})
	if !ok {
		return nil, ErrParamTypeMismatch{Key: key, Expected: "map[string]interface{}", Actual: val}
	}
	return m, nil
}
