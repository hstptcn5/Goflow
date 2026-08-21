package nodes

import "testing"

func TestEvaluateConditionTypedOperators(t *testing.T) {
	tests := []struct {
		name     string
		field    interface{}
		operator string
		value    interface{}
		want     bool
	}{
		{name: "numeric greater", field: float64(12), operator: "greater_than", value: "10", want: true},
		{name: "numeric less equal", field: 4, operator: "less_or_equal", value: 4.0, want: true},
		{name: "typed numeric equality", field: 3, operator: "equals", value: float64(3), want: true},
		{name: "string is not numeric equality", field: "03x", operator: "equals", value: 3, want: false},
		{name: "exists false", field: nil, operator: "exists", want: false},
		{name: "not exists", field: nil, operator: "not_exists", want: true},
		{name: "empty array", field: []interface{}{}, operator: "empty", want: true},
		{name: "not empty zero", field: 0, operator: "not_empty", want: true},
		{name: "contains", field: "hello world", operator: "contains", value: "world", want: true},
		{name: "not contains", field: "hello", operator: "not_contains", value: "world", want: true},
		{name: "starts with", field: "goflow", operator: "starts_with", value: "go", want: true},
		{name: "ends with", field: "goflow", operator: "ends_with", value: "flow", want: true},
		{name: "regex", field: "INV-2026-42", operator: "regex", value: `^INV-\d{4}-\d+$`, want: true},
		{name: "boolean true", field: true, operator: "true", want: true},
		{name: "boolean false", field: false, operator: "false", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evaluateCondition(tt.field, tt.operator, tt.value)
			if err != nil {
				t.Fatalf("evaluateCondition() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("evaluateCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvaluateConditionRejectsInvalidTypesAndRegex(t *testing.T) {
	if _, err := evaluateCondition("abc", "greater_than", 2); err == nil {
		t.Fatal("numeric comparison accepted non-numeric input")
	}
	if _, err := evaluateCondition("abc", "regex", "["); err == nil {
		t.Fatal("regex comparison accepted invalid pattern")
	}
	if _, err := evaluateCondition(map[string]interface{}{"x": 1}, "true", nil); err == nil {
		t.Fatal("boolean comparison accepted object input")
	}
}

func TestConditionIFLegacyOperatorAliasesRemainValid(t *testing.T) {
	for _, raw := range []string{"==", "!=", "is_not_empty"} {
		if _, err := normalizeConditionOperator(raw); err != nil {
			t.Fatalf("legacy operator %q rejected: %v", raw, err)
		}
	}
}
