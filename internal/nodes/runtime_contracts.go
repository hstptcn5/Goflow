package nodes

import "strings"

type runtimeContractSeed struct {
	outputs         []PluginOutputDefinition
	outputReference *OutputReferenceContract
	triggerContract *TriggerContract
}

func directOutputContract(description string, dynamic bool, examples ...string) *OutputReferenceContract {
	return &OutputReferenceContract{
		RootMode:      "direct",
		Pattern:       "{{<node_id>.<field_or_path>}}",
		DynamicFields: dynamic,
		Description:   description,
		Examples:      append([]string(nil), examples...),
	}
}

var builtinRuntimeContractSeeds = map[NodeType]runtimeContractSeed{
	TypeManualTrigger: {
		outputs: []PluginOutputDefinition{
			{Name: "triggered_at", Type: "string", Description: "UTC RFC3339 timestamp for a manual run without an explicit trigger payload"},
			{Name: "source", Type: "string", Description: "manual when no explicit trigger payload is supplied"},
		},
		outputReference: directOutputContract("The trigger payload is the node output directly; there is no output envelope.", true,
			"{{<node_id>.triggered_at}}"),
		triggerContract: &TriggerContract{
			Invocation:  "manual",
			PayloadRoot: "direct",
			Notes:       []string{"When the workflow is manually run with an explicit trigger payload, that payload is returned directly by the Manual Trigger node."},
		},
	},
	TypeCronTrigger: {
		outputs: []PluginOutputDefinition{
			{Name: "triggered_at", Type: "string", Description: "UTC RFC3339 trigger timestamp"},
			{Name: "schedule", Type: "string", Description: "Configured five-field cron expression"},
		},
		outputReference: directOutputContract("Cron output fields are exposed directly at the node root.", false,
			"{{<node_id>.triggered_at}}", "{{<node_id>.schedule}}"),
		triggerContract: &TriggerContract{
			Invocation:     "schedule",
			RequiresActive: true,
			PayloadRoot:    "direct",
		},
	},
	TypeWebhookTrigger: {
		outputs: []PluginOutputDefinition{
			{Name: "body", Type: "any", Description: "Parsed JSON request body when Content-Type is application/json, otherwise raw request data"},
			{Name: "body_raw", Type: "string", Description: "Raw request body"},
			{Name: "headers", Type: "object", Description: "Request headers with sensitive headers removed"},
			{Name: "method", Type: "string", Description: "HTTP method"},
			{Name: "path", Type: "string", Description: "Actual request path"},
			{Name: "query", Type: "object", Description: "Query parameters"},
		},
		outputReference: directOutputContract("Webhook request fields are exposed directly at the node root; body is not wrapped in an additional output property.", false,
			"{{<node_id>.body}}", "{{<node_id>.body.quantity}}", "{{<node_id>.method}}"),
		triggerContract: &TriggerContract{
			Invocation:       "http_webhook",
			EndpointTemplate: "/webhook/{workflow_id}",
			RequiresActive:   true,
			PayloadRoot:      "direct",
			Notes: []string{
				"Invoke the workflow at POST /webhook/{workflow_id}; the node path parameter does not replace this public endpoint in the current runtime.",
				"Do not instruct users to POST directly to the node path parameter.",
			},
		},
	},
	TypeHTTPRequest: {
		outputs: []PluginOutputDefinition{
			{Name: "status_code", Type: "number", Description: "Final HTTP response status code"},
			{Name: "headers", Type: "object", Description: "Final HTTP response headers"},
			{Name: "data", Type: "any", Description: "Parsed response body; in pagination mode this is the accumulated item array"},
			{Name: "pages", Type: "array", Description: "Pagination summaries when pagination is enabled"},
			{Name: "page_count", Type: "number", Description: "Number of fetched pages when pagination is enabled"},
			{Name: "item_count", Type: "number", Description: "Number of accumulated items when pagination is enabled"},
		},
		outputReference: directOutputContract("HTTP response fields are exposed directly at the node root.", false,
			"{{<node_id>.status_code}}", "{{<node_id>.data}}", "{{<node_id>.data.items.0.id}}"),
	},
	TypeJSONTransform: {
		outputs: []PluginOutputDefinition{
			{Name: "transformed", Type: "object", Description: "Parsed JSON object produced from json_template"},
			{Name: "context", Type: "object", Description: "Snapshot of upstream node outputs"},
		},
		outputReference: directOutputContract("JSON Transform returns an object with transformed and context at the node root.", false,
			"{{<node_id>.transformed}}", "{{<node_id>.transformed.status}}"),
	},
	TypeConditionIF: {
		outputs: []PluginOutputDefinition{
			{Name: "result", Type: "boolean", Description: "Boolean condition result"},
			{Name: "target_handle", Type: "string", Description: "true or false branch handle"},
			{Name: "evaluated", Type: "string", Description: "Human-readable evaluation summary"},
			{Name: "execution_tag", Type: "string", Description: "Per-execution diagnostic UUID"},
		},
		outputReference: directOutputContract("IF output fields are exposed directly at the node root; routing edges use sourceHandle true or false.", false,
			"{{<node_id>.result}}", "{{<node_id>.target_handle}}"),
	},
	TypeSwitch: {
		outputs: []PluginOutputDefinition{
			{Name: "matched", Type: "boolean", Description: "Whether a configured case matched"},
			{Name: "matched_index", Type: "number", Description: "Zero-based matched case index, or -1 for default"},
			{Name: "matched_handle", Type: "string", Description: "Matched case handle, or default"},
			{Name: "target_handle", Type: "string", Description: "Routing handle used by the engine"},
			{Name: "value", Type: "any", Description: "Resolved input value evaluated by the switch"},
		},
		outputReference: directOutputContract("Switch output fields are exposed directly at the node root. The value parameter should reference the upstream value itself, not an invented output envelope.", false,
			"{{<node_id>.matched_handle}}", "{{<node_id>.value}}"),
	},
	TypePythonCode: {
		outputReference: directOutputContract("The value assigned to the Python variable output becomes the node output directly. Goflow does NOT wrap it as {output: ...}. If code sets output = {\"category\": \"NORMAL\"}, reference it as {{<node_id>.category}}, not {{<node_id>.output.category}}.", true,
			"{{<node_id>.<field>}}", "{{<node_id>.category}}", "{{<node_id>.sum}}"),
	},
}

func appendUniqueCapability(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// DefinitionWithRuntimeContract enriches the public node schema from one
// canonical contract table. Structured fields remain suitable for UI/API use,
// while compact capability tags carry the same facts into Agent Lab without
// bloating user-facing node descriptions.
func DefinitionWithRuntimeContract(def NodeDefinition) NodeDefinition {
	seed, ok := builtinRuntimeContractSeeds[def.Type]
	if ok {
		if len(def.Outputs) == 0 && len(seed.outputs) > 0 {
			def.Outputs = append([]PluginOutputDefinition(nil), seed.outputs...)
		}
		if def.OutputReference == nil && seed.outputReference != nil {
			copyRef := *seed.outputReference
			copyRef.Examples = append([]string(nil), seed.outputReference.Examples...)
			def.OutputReference = &copyRef
		}
		if def.TriggerContract == nil && seed.triggerContract != nil {
			copyTrigger := *seed.triggerContract
			copyTrigger.Notes = append([]string(nil), seed.triggerContract.Notes...)
			def.TriggerContract = &copyTrigger
		}
	}

	// Custom/plugin nodes already declare outputs. Unless a future manifest
	// supplies a more specific contract, documented output fields are direct.
	if def.OutputReference == nil && len(def.Outputs) > 0 {
		def.OutputReference = directOutputContract("Declared output fields are exposed directly at the node root.", false,
			"{{<node_id>.<field>}}")
	}

	if def.OutputReference != nil {
		def.Capabilities = appendUniqueCapability(def.Capabilities, "output_reference:"+def.OutputReference.RootMode)
		def.Capabilities = appendUniqueCapability(def.Capabilities, "output_pattern:"+def.OutputReference.Pattern)
		if def.OutputReference.DynamicFields {
			def.Capabilities = appendUniqueCapability(def.Capabilities, "output_dynamic_fields:true")
		}
		if def.OutputReference.Description != "" {
			def.Capabilities = appendUniqueCapability(def.Capabilities, "output_note:"+def.OutputReference.Description)
		}
		for _, example := range def.OutputReference.Examples {
			def.Capabilities = appendUniqueCapability(def.Capabilities, "output_example:"+example)
		}
	}
	for _, output := range def.Outputs {
		tag := "output_field:" + output.Name
		if output.Type != "" {
			tag += ":" + output.Type
		}
		def.Capabilities = appendUniqueCapability(def.Capabilities, tag)
	}
	if def.TriggerContract != nil {
		def.Capabilities = appendUniqueCapability(def.Capabilities, "trigger:"+def.TriggerContract.Invocation)
		if def.TriggerContract.EndpointTemplate != "" {
			def.Capabilities = appendUniqueCapability(def.Capabilities, "trigger_endpoint:"+def.TriggerContract.EndpointTemplate)
		}
		if def.TriggerContract.RequiresActive {
			def.Capabilities = appendUniqueCapability(def.Capabilities, "trigger_requires_active:true")
		}
		if def.TriggerContract.PayloadRoot != "" {
			def.Capabilities = appendUniqueCapability(def.Capabilities, "trigger_payload_root:"+def.TriggerContract.PayloadRoot)
		}
		for _, note := range def.TriggerContract.Notes {
			def.Capabilities = appendUniqueCapability(def.Capabilities, "trigger_note:"+note)
		}
	}
	return def
}
