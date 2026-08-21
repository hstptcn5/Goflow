package nodes

import "testing"

func TestErrorPolicyForNode(t *testing.T) {
	tests := []struct {
		name string
		raw  interface{}
		want ErrorPolicy
	}{
		{name: "missing defaults stop", raw: nil, want: ErrorPolicyStop},
		{name: "stop label", raw: ErrorPolicyStopLabel, want: ErrorPolicyStop},
		{name: "continue label", raw: ErrorPolicyContinueLabel, want: ErrorPolicyContinue},
		{name: "error output label", raw: ErrorPolicyErrorOutputLabel, want: ErrorPolicyErrorOutput},
		{name: "serialized error output", raw: "error_output", want: ErrorPolicyErrorOutput},
		{name: "unknown fails closed", raw: "ignore everything", want: ErrorPolicyStop},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := map[string]interface{}{}
			if tt.raw != nil {
				params["on_error"] = tt.raw
			}
			if got := ErrorPolicyForNode(&Node{Params: params}); got != tt.want {
				t.Fatalf("ErrorPolicyForNode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefinitionWithErrorPolicy(t *testing.T) {
	action := DefinitionWithErrorPolicy(NodeDefinition{Type: TypeHTTPRequest, Category: "ACTION"})
	if len(action.Params) != 1 {
		t.Fatalf("action params = %d, want 1", len(action.Params))
	}
	param := action.Params[0]
	if param.Name != "on_error" || param.Default != ErrorPolicyStopLabel {
		t.Fatalf("unexpected error policy param: %#v", param)
	}
	if len(param.Options) != 3 {
		t.Fatalf("options = %v, want 3 choices", param.Options)
	}

	trigger := DefinitionWithErrorPolicy(NodeDefinition{Type: TypeManualTrigger, Category: "TRIGGER"})
	if len(trigger.Params) != 0 {
		t.Fatalf("trigger received common error policy params: %#v", trigger.Params)
	}

	existing := NodeDefinition{Type: TypeHTTPRequest, Category: "ACTION", Params: []ParamDefinition{{Name: "on_error", Default: "custom"}}}
	wrapped := DefinitionWithErrorPolicy(existing)
	if len(wrapped.Params) != 1 || wrapped.Params[0].Default != "custom" {
		t.Fatalf("existing on_error definition was overwritten: %#v", wrapped.Params)
	}
}
