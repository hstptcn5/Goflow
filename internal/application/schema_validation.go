package application

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
)

type inputSchema struct {
	Type                 string                 `json:"type"`
	Required             []string               `json:"required"`
	Properties           map[string]inputSchema `json:"properties"`
	AdditionalProperties *bool                  `json:"additionalProperties"`
	Items                *inputSchema           `json:"items"`
	Enum                 []interface{}          `json:"enum"`
	Const                interface{}            `json:"const"`
	Minimum              *float64               `json:"minimum"`
	Maximum              *float64               `json:"maximum"`
	MinLength            *int                   `json:"minLength"`
	MaxLength            *int                   `json:"maxLength"`
	Pattern              string                 `json:"pattern"`
	OneOf                []inputSchema          `json:"oneOf"`
	AnyOf                []inputSchema          `json:"anyOf"`
}

func validateWorkflowInput(rawSchema string, input interface{}) error {
	rawSchema = strings.TrimSpace(rawSchema)
	if rawSchema == "" || rawSchema == "{}" {
		return nil
	}
	if err := validateSupportedSchemaKeywords([]byte(rawSchema), "$schema"); err != nil {
		return err
	}
	var schema inputSchema
	if err := json.Unmarshal([]byte(rawSchema), &schema); err != nil {
		return fmt.Errorf("%w: input_schema_json is invalid JSON", ErrInvalidWorkflowInput)
	}
	if schema.Type == "" && len(schema.Required) == 0 && len(schema.Properties) == 0 {
		return nil
	}
	if input == nil {
		input = map[string]interface{}{}
	}
	return validateSchemaValue(schema, input, "$")
}

func ValidateWorkflowSchema(rawSchema, field string) error {
	rawSchema = strings.TrimSpace(rawSchema)
	if rawSchema == "" || rawSchema == "{}" {
		return nil
	}
	if err := validateSupportedSchemaKeywords([]byte(rawSchema), field); err != nil {
		return err
	}
	var schema inputSchema
	if err := json.Unmarshal([]byte(rawSchema), &schema); err != nil {
		return fmt.Errorf("%w: %s is invalid JSON", ErrInvalidWorkflowInput, field)
	}
	return nil
}

func validateSchemaValue(schema inputSchema, value interface{}, path string) error {
	if len(schema.OneOf) > 0 {
		matches := 0
		for _, candidate := range schema.OneOf {
			if validateSchemaValue(candidate, value, path) == nil {
				matches++
			}
		}
		if matches != 1 {
			return fmt.Errorf("%w: %s must match exactly one oneOf schema, matched %d", ErrInvalidWorkflowInput, path, matches)
		}
	}
	if len(schema.AnyOf) > 0 {
		matched := false
		for _, candidate := range schema.AnyOf {
			if validateSchemaValue(candidate, value, path) == nil {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("%w: %s must match at least one anyOf schema", ErrInvalidWorkflowInput, path)
		}
	}
	if schema.Type != "" {
		if err := validateType(schema.Type, value, path); err != nil {
			return err
		}
	}
	if schema.Const != nil && !jsonValueEqual(schema.Const, value) {
		return fmt.Errorf("%w: %s must equal const value", ErrInvalidWorkflowInput, path)
	}
	if len(schema.Enum) > 0 {
		found := false
		for _, item := range schema.Enum {
			if jsonValueEqual(item, value) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: %s must be one of the enum values", ErrInvalidWorkflowInput, path)
		}
	}
	if err := validateNumericBounds(schema, value, path); err != nil {
		return err
	}
	if err := validateStringBounds(schema, value, path); err != nil {
		return err
	}
	if schema.Type == "object" || len(schema.Properties) > 0 || len(schema.Required) > 0 {
		obj, ok := value.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%w: %s must be an object", ErrInvalidWorkflowInput, path)
		}
		for _, field := range schema.Required {
			if _, exists := obj[field]; !exists {
				return fmt.Errorf("%w: %s.%s is required", ErrInvalidWorkflowInput, path, field)
			}
		}
		for field, childSchema := range schema.Properties {
			if child, exists := obj[field]; exists {
				if err := validateSchemaValue(childSchema, child, path+"."+field); err != nil {
					return err
				}
			}
		}
		if schema.AdditionalProperties != nil && !*schema.AdditionalProperties {
			for field := range obj {
				if _, allowed := schema.Properties[field]; !allowed {
					return fmt.Errorf("%w: %s.%s is not allowed", ErrInvalidWorkflowInput, path, field)
				}
			}
		}
	}
	if schema.Type == "array" && schema.Items != nil {
		items, ok := value.([]interface{})
		if !ok {
			return fmt.Errorf("%w: %s must be an array", ErrInvalidWorkflowInput, path)
		}
		for index, item := range items {
			if err := validateSchemaValue(*schema.Items, item, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateNumericBounds(schema inputSchema, value interface{}, path string) error {
	if schema.Minimum == nil && schema.Maximum == nil {
		return nil
	}
	number, ok := numberValue(value)
	if !ok {
		return nil
	}
	if schema.Minimum != nil && number < *schema.Minimum {
		return fmt.Errorf("%w: %s must be >= %v", ErrInvalidWorkflowInput, path, *schema.Minimum)
	}
	if schema.Maximum != nil && number > *schema.Maximum {
		return fmt.Errorf("%w: %s must be <= %v", ErrInvalidWorkflowInput, path, *schema.Maximum)
	}
	return nil
}

func validateStringBounds(schema inputSchema, value interface{}, path string) error {
	text, ok := value.(string)
	if !ok {
		return nil
	}
	if schema.MinLength != nil && len(text) < *schema.MinLength {
		return fmt.Errorf("%w: %s length must be >= %d", ErrInvalidWorkflowInput, path, *schema.MinLength)
	}
	if schema.MaxLength != nil && len(text) > *schema.MaxLength {
		return fmt.Errorf("%w: %s length must be <= %d", ErrInvalidWorkflowInput, path, *schema.MaxLength)
	}
	if schema.Pattern != "" {
		re, err := regexp.Compile(schema.Pattern)
		if err != nil {
			return fmt.Errorf("%w: %s pattern is invalid", ErrInvalidWorkflowInput, path)
		}
		if !re.MatchString(text) {
			return fmt.Errorf("%w: %s must match pattern", ErrInvalidWorkflowInput, path)
		}
	}
	return nil
}

func validateType(schemaType string, value interface{}, path string) error {
	switch schemaType {
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("%w: %s must be an object", ErrInvalidWorkflowInput, path)
		}
	case "array":
		if _, ok := value.([]interface{}); !ok {
			return fmt.Errorf("%w: %s must be an array", ErrInvalidWorkflowInput, path)
		}
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%w: %s must be a string", ErrInvalidWorkflowInput, path)
		}
	case "number":
		if !isNumber(value) {
			return fmt.Errorf("%w: %s must be a number", ErrInvalidWorkflowInput, path)
		}
	case "integer":
		if !isInteger(value) {
			return fmt.Errorf("%w: %s must be an integer", ErrInvalidWorkflowInput, path)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%w: %s must be a boolean", ErrInvalidWorkflowInput, path)
		}
	case "null":
		if value != nil {
			return fmt.Errorf("%w: %s must be null", ErrInvalidWorkflowInput, path)
		}
	}
	return nil
}

func isNumber(value interface{}) bool {
	_, ok := numberValue(value)
	return ok
}

func isInteger(value interface{}) bool {
	switch v := value.(type) {
	case float64:
		return math.Trunc(v) == v
	case float32:
		return math.Trunc(float64(v)) == float64(v)
	case int, int64, int32, uint, uint64, uint32:
		return true
	default:
		return false
	}
}

func numberValue(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint64:
		return float64(v), true
	case uint32:
		return float64(v), true
	default:
		return 0, false
	}
}

func jsonValueEqual(a, b interface{}) bool {
	aJSON, errA := json.Marshal(a)
	bJSON, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(aJSON) == string(bJSON)
}

func validateSupportedSchemaKeywords(raw []byte, path string) error {
	var schema map[string]json.RawMessage
	if err := json.Unmarshal(raw, &schema); err != nil {
		return fmt.Errorf("%w: %s is invalid JSON", ErrInvalidWorkflowInput, path)
	}
	allowed := map[string]bool{
		"type": true, "properties": true, "required": true, "additionalProperties": true,
		"items": true, "enum": true, "const": true, "minimum": true, "maximum": true,
		"minLength": true, "maxLength": true, "pattern": true, "oneOf": true, "anyOf": true,
	}
	for key, value := range schema {
		if !allowed[key] {
			return fmt.Errorf("%w: unsupported schema keyword %q at %s", ErrInvalidWorkflowInput, key, path)
		}
		switch key {
		case "properties":
			var properties map[string]json.RawMessage
			if err := json.Unmarshal(value, &properties); err != nil {
				return fmt.Errorf("%w: %s.properties must be an object", ErrInvalidWorkflowInput, path)
			}
			for name, child := range properties {
				if err := validateSupportedSchemaKeywords(child, path+".properties."+name); err != nil {
					return err
				}
			}
		case "items":
			if err := validateSupportedSchemaKeywords(value, path+".items"); err != nil {
				return err
			}
		case "oneOf", "anyOf":
			var schemas []json.RawMessage
			if err := json.Unmarshal(value, &schemas); err != nil {
				return fmt.Errorf("%w: %s.%s must be an array", ErrInvalidWorkflowInput, path, key)
			}
			for index, child := range schemas {
				if err := validateSupportedSchemaKeywords(child, fmt.Sprintf("%s.%s[%d]", path, key, index)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
