package application

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

type inputSchema struct {
	Type                 string                 `json:"type"`
	Required             []string               `json:"required"`
	Properties           map[string]inputSchema `json:"properties"`
	AdditionalProperties *bool                  `json:"additionalProperties"`
	Items                *inputSchema           `json:"items"`
}

func validateWorkflowInput(rawSchema string, input interface{}) error {
	rawSchema = strings.TrimSpace(rawSchema)
	if rawSchema == "" || rawSchema == "{}" {
		return nil
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

func validateSchemaValue(schema inputSchema, value interface{}, path string) error {
	if schema.Type != "" {
		if err := validateType(schema.Type, value, path); err != nil {
			return err
		}
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
	switch value.(type) {
	case float64, float32, int, int64, int32, uint, uint64, uint32:
		return true
	default:
		return false
	}
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
