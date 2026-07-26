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
