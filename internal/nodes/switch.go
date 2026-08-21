package nodes

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const maxSwitchCases = 16

var switchHandlePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

type SwitchCase struct {
	Handle   string      `json:"handle"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value,omitempty"`
}

type SwitchExecutor struct{}

func NewSwitchExecutor() *SwitchExecutor { return &SwitchExecutor{} }

func parseSwitchCases(raw interface{}) ([]SwitchCase, error) {
	if raw == nil || (func() bool {
		text, ok := raw.(string)
		return ok && strings.TrimSpace(text) == ""
	})() {
		return nil, nil
	}

	var encoded []byte
	switch typed := raw.(type) {
	case string:
		encoded = []byte(typed)
	case []interface{}, []SwitchCase:
		var err error
		encoded, err = json.Marshal(typed)
		if err != nil {
			return nil, fmt.Errorf("switch cases could not be encoded: %w", err)
		}
	default:
		return nil, fmt.Errorf("switch cases must be a JSON array")
	}

	var cases []SwitchCase
	if err := json.Unmarshal(encoded, &cases); err != nil {
		return nil, fmt.Errorf("switch cases must be a JSON array: %w", err)
	}
	if len(cases) > maxSwitchCases {
		return nil, fmt.Errorf("switch has %d cases; maximum is %d", len(cases), maxSwitchCases)
	}

	seen := make(map[string]struct{}, len(cases))
	for i := range cases {
		cases[i].Handle = strings.TrimSpace(cases[i].Handle)
		if cases[i].Handle == "" {
			cases[i].Handle = fmt.Sprintf("case_%d", i+1)
		}
		if !switchHandlePattern.MatchString(cases[i].Handle) {
			return nil, fmt.Errorf("switch case %d handle %q is invalid", i+1, cases[i].Handle)
		}
		if cases[i].Handle == "default" || cases[i].Handle == "error" {
			return nil, fmt.Errorf("switch case %d uses reserved handle %q", i+1, cases[i].Handle)
		}
		if _, exists := seen[cases[i].Handle]; exists {
			return nil, fmt.Errorf("switch handle %q is duplicated", cases[i].Handle)
		}
		seen[cases[i].Handle] = struct{}{}
		normalized, err := normalizeConditionOperator(cases[i].Operator)
		if err != nil {
			return nil, fmt.Errorf("switch case %d: %w", i+1, err)
		}
		cases[i].Operator = normalized
		if normalized == "regex" {
			if _, err := evaluateCondition("", normalized, cases[i].Value); err != nil {
				return nil, fmt.Errorf("switch case %d: %w", i+1, err)
			}
		}
	}
	return cases, nil
}

func (e *SwitchExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	cases, err := parseSwitchCases(node.Params["cases_json"])
	if err != nil {
		return nil, err
	}
	input := node.Params["value"]
	for i, item := range cases {
		matched, err := evaluateCondition(input, item.Operator, item.Value)
		if err != nil {
			return nil, fmt.Errorf("switch case %d (%s): %w", i+1, item.Handle, err)
		}
		if matched {
			return map[string]interface{}{
				"matched":        true,
				"matched_index":  i,
				"matched_handle": item.Handle,
				"target_handle":  item.Handle,
				"value":          input,
			}, nil
		}
	}
	return map[string]interface{}{
		"matched":        false,
		"matched_index":  -1,
		"matched_handle": "default",
		"target_handle":  "default",
		"value":          input,
	}, nil
}

func (e *SwitchExecutor) Validate(node *Node) error {
	_, err := parseSwitchCases(node.Params["cases_json"])
	return err
}

func (e *SwitchExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeSwitch, Name: "Switch", Description: "Routes one typed value to the first matching case or a default output", Icon: "GitBranch", Category: "LOGIC", Retryable: true,
		Params: []ParamDefinition{
			{Name: "value", Label: "Input Value", Type: "text", Default: "", Required: true, Description: "Value evaluated against each case in order; complete expressions preserve type"},
			{Name: "cases_json", Label: "Cases", Type: "json", Default: "[\n  {\"handle\":\"case_1\",\"operator\":\"equals\",\"value\":\"A\"},\n  {\"handle\":\"case_2\",\"operator\":\"equals\",\"value\":\"B\"}\n]", Required: false, Description: "Up to 16 ordered cases. Each case has handle, operator and value. Unmatched input uses the default output."},
		},
	}
}
