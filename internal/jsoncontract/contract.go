package jsoncontract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
)

type Contract struct {
	Required map[string]FieldRule `json:"required"`
}

type FieldRule struct {
	Type     string   `json:"type"`
	NonEmpty bool     `json:"non_empty,omitempty"`
	Minimum  *float64 `json:"minimum,omitempty"`
}

type Summary struct {
	RequiredFields int
	ReportDate     string
}

func Parse(raw interface{}) (Contract, error) {
	if raw == nil {
		return Contract{}, fmt.Errorf("response_contract is required")
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return Contract{}, fmt.Errorf("response_contract must be JSON: %w", err)
	}
	if text, ok := raw.(string); ok {
		data = []byte(text)
	}
	var contract Contract
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&contract); err != nil {
		return Contract{}, fmt.Errorf("response_contract is invalid: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return Contract{}, fmt.Errorf("response_contract is invalid: %w", err)
	}
	if len(contract.Required) == 0 {
		return Contract{}, fmt.Errorf("response_contract.required must contain at least one field")
	}
	for name, rule := range contract.Required {
		if strings.TrimSpace(name) == "" || len(name) > 128 {
			return Contract{}, fmt.Errorf("response_contract contains an invalid field name")
		}
		switch rule.Type {
		case "string":
			if rule.Minimum != nil {
				return Contract{}, fmt.Errorf("response_contract field %q cannot use minimum with string", name)
			}
		case "number", "integer":
			if rule.NonEmpty {
				return Contract{}, fmt.Errorf("response_contract field %q cannot use non_empty with %s", name, rule.Type)
			}
		case "boolean":
			if rule.NonEmpty || rule.Minimum != nil {
				return Contract{}, fmt.Errorf("response_contract field %q has unsupported boolean constraints", name)
			}
		default:
			return Contract{}, fmt.Errorf("response_contract field %q has unsupported type %q", name, rule.Type)
		}
	}
	return contract, nil
}

func Decode(data []byte) (interface{}, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, err
	}
	return value, nil
}

func Validate(value interface{}, contract Contract) (Summary, error) {
	object, ok := value.(map[string]interface{})
	if !ok {
		return Summary{}, fmt.Errorf("JSON root must be an object")
	}
	summary := Summary{RequiredFields: len(contract.Required)}
	for name, rule := range contract.Required {
		field, exists := object[name]
		if !exists || field == nil {
			return Summary{}, fmt.Errorf("missing required field %q", name)
		}
		if err := validateField(name, field, rule); err != nil {
			return Summary{}, err
		}
	}
	if reportDate, ok := object["report_date"].(string); ok && safeDateSummary(reportDate) {
		summary.ReportDate = reportDate
	}
	return summary, nil
}

func validateField(name string, value interface{}, rule FieldRule) error {
	switch rule.Type {
	case "string":
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("field %q must be a string", name)
		}
		if rule.NonEmpty && strings.TrimSpace(text) == "" {
			return fmt.Errorf("field %q must be a non-empty string", name)
		}
	case "number", "integer":
		number, ok := asNumber(value)
		if !ok {
			return fmt.Errorf("field %q must be a JSON %s", name, rule.Type)
		}
		if rule.Type == "integer" && math.Trunc(number) != number {
			return fmt.Errorf("field %q must be an integer", name)
		}
		if rule.Minimum != nil && number < *rule.Minimum {
			return fmt.Errorf("field %q must be at least %v", name, *rule.Minimum)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("field %q must be a boolean", name)
		}
	}
	return nil
}

func asNumber(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil && !math.IsNaN(number) && !math.IsInf(number, 0)
	case float64:
		return typed, !math.IsNaN(typed) && !math.IsInf(typed, 0)
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}

func safeDateSummary(value string) bool {
	if len(value) != len("2006-01-02") {
		return false
	}
	for index, char := range value {
		if index == 4 || index == 7 {
			if char != '-' {
				return false
			}
			continue
		}
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values are not allowed")
		}
		return err
	}
	return nil
}
