package nodes

import "testing"

func TestSwitchRoutesFirstMatchAndDefault(t *testing.T) {
	executor := NewSwitchExecutor()
	cases := `[
		{"handle":"large","operator":"greater_or_equal","value":10},
		{"handle":"small","operator":"less_than","value":10}
	]`

	out, err := executor.Execute(NewExecutionContext("wf", "exec"), &Node{Params: map[string]interface{}{
		"value":      float64(12),
		"cases_json": cases,
	}})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := out.(map[string]interface{})["target_handle"]; got != "large" {
		t.Fatalf("target_handle = %#v, want large", got)
	}

	out, err = executor.Execute(NewExecutionContext("wf", "exec"), &Node{Params: map[string]interface{}{
		"value":      "neither",
		"cases_json": `[{"handle":"a","operator":"equals","value":"A"}]`,
	}})
	if err != nil {
		t.Fatalf("Execute(default) error = %v", err)
	}
	if got := out.(map[string]interface{})["target_handle"]; got != "default" {
		t.Fatalf("default target_handle = %#v, want default", got)
	}
}

func TestSwitchRejectsUnsafeOrDuplicateHandles(t *testing.T) {
	for _, raw := range []string{
		`[{"handle":"default","operator":"equals","value":"x"}]`,
		`[{"handle":"same","operator":"equals","value":"x"},{"handle":"same","operator":"equals","value":"y"}]`,
		`[{"handle":"bad handle","operator":"equals","value":"x"}]`,
	} {
		if _, err := parseSwitchCases(raw); err == nil {
			t.Fatalf("parseSwitchCases(%s) unexpectedly succeeded", raw)
		}
	}
}

func TestSwitchSupportsTypedConditionOperators(t *testing.T) {
	cases, err := parseSwitchCases(`[{"handle":"invoice","operator":"regex","value":"^INV-"},{"handle":"other","operator":"not_empty"}]`)
	if err != nil {
		t.Fatalf("parseSwitchCases() error = %v", err)
	}
	if len(cases) != 2 || cases[0].Operator != "regex" || cases[1].Operator != "not_empty" {
		t.Fatalf("unexpected cases: %#v", cases)
	}
}
