package pack

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strings"
)

// ValidateConfigValue applies the manifest's runtime value contract. The same
// validator is used by setup persistence and author-only offline fixtures.
func ValidateConfigValue(field ConfigField, value interface{}) (interface{}, error) {
	if value == nil {
		if field.Required {
			return nil, fmt.Errorf("value is required")
		}
		return nil, nil
	}
	switch field.Type {
	case "string":
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("must be a string")
		}
		if err := validateConfigStringLength(field, text); err != nil {
			return nil, err
		}
		return text, nil
	case "url":
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("must be a string")
		}
		if err := validateConfigStringLength(field, text); err != nil {
			return nil, err
		}
		parsed, err := url.Parse(text)
		if err != nil || !parsed.IsAbs() || parsed.Host == "" || (strings.ToLower(parsed.Scheme) != "http" && strings.ToLower(parsed.Scheme) != "https") {
			return nil, fmt.Errorf("must be an absolute http or https URL")
		}
		return text, nil
	case "integer":
		n, err := configNumberAsInt(value)
		if err != nil {
			return nil, err
		}
		if field.Min != nil && n < *field.Min {
			return nil, fmt.Errorf("must be >= %d", *field.Min)
		}
		if field.Max != nil && n > *field.Max {
			return nil, fmt.Errorf("must be <= %d", *field.Max)
		}
		return n, nil
	case "boolean":
		boolean, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("must be a boolean")
		}
		return boolean, nil
	case "select":
		key, err := configScalarKey(value)
		if err != nil {
			return nil, err
		}
		for _, option := range field.Options {
			if optionKey, err := configScalarKey(option); err == nil && optionKey == key {
				return value, nil
			}
		}
		return nil, fmt.Errorf("must match one configured option")
	default:
		return nil, fmt.Errorf("unsupported config type %q", field.Type)
	}
}

func validateConfigStringLength(field ConfigField, value string) error {
	if field.MinLength != nil && len(value) < *field.MinLength {
		return fmt.Errorf("must be at least %d characters", *field.MinLength)
	}
	if field.MaxLength != nil && len(value) > *field.MaxLength {
		return fmt.Errorf("must be at most %d characters", *field.MaxLength)
	}
	return nil
}

func configNumberAsInt(value interface{}) (int, error) {
	switch typed := value.(type) {
	case int:
		return typed, nil
	case float64:
		if math.Trunc(typed) != typed || typed > math.MaxInt || typed < math.MinInt {
			return 0, fmt.Errorf("must be an integer")
		}
		return int(typed), nil
	case json.Number:
		n, err := typed.Int64()
		if err != nil || n > math.MaxInt || n < math.MinInt {
			return 0, fmt.Errorf("must be an integer")
		}
		return int(n), nil
	default:
		return 0, fmt.Errorf("must be an integer")
	}
}

func configScalarKey(value interface{}) (string, error) {
	switch typed := value.(type) {
	case string:
		return "string:" + typed, nil
	case bool:
		return fmt.Sprintf("bool:%t", typed), nil
	case int:
		return fmt.Sprintf("number:%d", typed), nil
	case float64:
		if math.Trunc(typed) != typed {
			return "", fmt.Errorf("must be a scalar option")
		}
		return fmt.Sprintf("number:%0.f", typed), nil
	case json.Number:
		n, err := typed.Int64()
		if err != nil {
			return "", fmt.Errorf("must be a scalar option")
		}
		return fmt.Sprintf("number:%d", n), nil
	default:
		return "", fmt.Errorf("must be a scalar option")
	}
}
