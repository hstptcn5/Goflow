package nodes

import "strings"

// ErrorPolicy controls how the engine handles an executor error for one node.
type ErrorPolicy string

const (
	ErrorPolicyStop        ErrorPolicy = "stop"
	ErrorPolicyContinue    ErrorPolicy = "continue"
	ErrorPolicyErrorOutput ErrorPolicy = "error_output"
)

const (
	ErrorPolicyStopLabel        = "Stop workflow"
	ErrorPolicyContinueLabel    = "Continue"
	ErrorPolicyErrorOutputLabel = "Continue via error output"
)

// ErrorPolicyForNode returns a backward-compatible policy for a workflow node.
// Existing workflows do not contain on_error, so the empty value intentionally
// resolves to Stop workflow.
func ErrorPolicyForNode(node *Node) ErrorPolicy {
	if node == nil {
		return ErrorPolicyStop
	}
	raw, _ := node.Params["on_error"].(string)
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "continue":
		return ErrorPolicyContinue
	case "error_output", "error output", "continue via error output":
		return ErrorPolicyErrorOutput
	default:
		return ErrorPolicyStop
	}
}

// DefinitionWithErrorPolicy exposes the common per-node error behavior in the
// visual editor without changing the persisted workflow schema. Trigger nodes
// are excluded because they start executions rather than handle upstream work.
func DefinitionWithErrorPolicy(def NodeDefinition) NodeDefinition {
	if strings.EqualFold(strings.TrimSpace(def.Category), "TRIGGER") {
		return def
	}
	for _, param := range def.Params {
		if param.Name == "on_error" {
			return def
		}
	}
	def.Params = append(def.Params, ParamDefinition{
		Name:        "on_error",
		Label:       "On Error",
		Type:        "select",
		Default:     ErrorPolicyStopLabel,
		Options:     []string{ErrorPolicyStopLabel, ErrorPolicyContinueLabel, ErrorPolicyErrorOutputLabel},
		Required:    false,
		Description: "Choose whether this node stops the workflow, continues normally, or routes failure through the error output. Existing workflows without this setting default to Stop workflow.",
	})
	return def
}
