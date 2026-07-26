package application

import (
	"errors"
	"testing"
)

func TestValidateWorkflowInputAcceptsValidObject(t *testing.T) {
	schema := `{
		"type":"object",
		"required":["date"],
		"properties":{
			"date":{"type":"string"},
			"count":{"type":"integer"}
		},
		"additionalProperties":false
	}`
	input := map[string]interface{}{"date": "2026-07-26", "count": float64(2)}
	if err := validateWorkflowInput(schema, input); err != nil {
		t.Fatalf("expected valid input, got %v", err)
	}
}

func TestValidateWorkflowInputRejectsMissingRequired(t *testing.T) {
	err := validateWorkflowInput(`{"type":"object","required":["date"]}`, map[string]interface{}{})
	if !errors.Is(err, ErrInvalidWorkflowInput) {
		t.Fatalf("expected ErrInvalidWorkflowInput, got %v", err)
	}
}

func TestValidateWorkflowInputRejectsWrongType(t *testing.T) {
	schema := `{"type":"object","properties":{"date":{"type":"string"}}}`
	err := validateWorkflowInput(schema, map[string]interface{}{"date": float64(1)})
	if !errors.Is(err, ErrInvalidWorkflowInput) {
		t.Fatalf("expected ErrInvalidWorkflowInput, got %v", err)
	}
}

func TestValidateWorkflowInputRejectsAdditionalProperties(t *testing.T) {
	schema := `{"type":"object","properties":{"date":{"type":"string"}},"additionalProperties":false}`
	err := validateWorkflowInput(schema, map[string]interface{}{"date": "2026-07-26", "extra": true})
	if !errors.Is(err, ErrInvalidWorkflowInput) {
		t.Fatalf("expected ErrInvalidWorkflowInput, got %v", err)
	}
}

func TestValidateWorkflowInputSupportsExtendedSubset(t *testing.T) {
	schema := `{
		"type":"object",
		"properties":{
			"env":{"enum":["dev","prod"]},
			"kind":{"const":"release"},
			"score":{"type":"number","minimum":1,"maximum":10},
			"ticket":{"type":"string","minLength":3,"maxLength":12,"pattern":"^INC-[0-9]+$"},
			"target":{"oneOf":[{"type":"string"},{"type":"integer"}]},
			"mode":{"anyOf":[{"const":"fast"},{"const":"safe"}]}
		}
	}`
	input := map[string]interface{}{
		"env":    "prod",
		"kind":   "release",
		"score":  float64(7),
		"ticket": "INC-123",
		"target": "server-1",
		"mode":   "safe",
	}
	if err := validateWorkflowInput(schema, input); err != nil {
		t.Fatalf("expected valid extended subset input, got %v", err)
	}
}

func TestValidateWorkflowInputRejectsUnsupportedKeyword(t *testing.T) {
	err := validateWorkflowInput(`{"type":"object","format":"email"}`, map[string]interface{}{})
	if !errors.Is(err, ErrInvalidWorkflowInput) {
		t.Fatalf("expected ErrInvalidWorkflowInput, got %v", err)
	}
}

func TestValidateWorkflowInputRejectsPatternMismatch(t *testing.T) {
	schema := `{"type":"object","properties":{"ticket":{"type":"string","pattern":"^INC-[0-9]+$"}}}`
	err := validateWorkflowInput(schema, map[string]interface{}{"ticket": "TASK-1"})
	if !errors.Is(err, ErrInvalidWorkflowInput) {
		t.Fatalf("expected ErrInvalidWorkflowInput, got %v", err)
	}
}
