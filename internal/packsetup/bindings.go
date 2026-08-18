package packsetup

import (
	"encoding/json"
	"fmt"

	"goflow/internal/client"
	"goflow/internal/nodes"
	"goflow/internal/pack"
)

func ApplyBindings(workflow client.Workflow, manifest pack.Manifest, config map[string]interface{}, credentials map[string]CredentialSlot) (client.Workflow, error) {
	var nodeList []nodes.Node
	if err := json.Unmarshal([]byte(workflow.NodesJSON), &nodeList); err != nil {
		return client.Workflow{}, fmt.Errorf("pack setup: bindings: workflow nodes_json must be JSON: %w", err)
	}
	nodeIndex := make(map[string]int, len(nodeList))
	for i := range nodeList {
		nodeIndex[nodeList[i].ID] = i
		if nodeList[i].Params == nil {
			nodeList[i].Params = map[string]interface{}{}
		}
	}
	for _, binding := range manifest.Bindings {
		nodePos, ok := nodeIndex[binding.Target.NodeID]
		if !ok {
			return client.Workflow{}, fmt.Errorf("pack setup: binding target node %q does not exist", binding.Target.NodeID)
		}
		value, present, err := bindingValue(binding.Source, manifest, config, credentials)
		if err != nil {
			return client.Workflow{}, err
		}
		if !present {
			// Optional credential slots deliberately leave the pack-authored default
			// in place until the user assigns that credential. This lets workflows
			// safely branch around optional integrations without making setup fail.
			continue
		}
		nodeList[nodePos].Params[binding.Target.Param] = value
	}
	nodesJSON, err := json.Marshal(nodeList)
	if err != nil {
		return client.Workflow{}, fmt.Errorf("pack setup: marshal bound workflow nodes: %w", err)
	}
	cloned := workflow
	cloned.NodesJSON = string(nodesJSON)
	return cloned, nil
}

func bindingValue(source string, manifest pack.Manifest, config map[string]interface{}, credentials map[string]CredentialSlot) (interface{}, bool, error) {
	kind, key, ok := splitBindingSource(source)
	if !ok {
		return nil, false, fmt.Errorf("pack setup: binding source %q is invalid", source)
	}
	switch kind {
	case "config":
		value, exists := config[key]
		if !exists {
			return nil, false, fmt.Errorf("pack setup: config source %q is missing", key)
		}
		return value, true, nil
	case "credential":
		slot, exists := credentials[key]
		if exists {
			return slot.CredentialID, true, nil
		}
		requirement, declared := credentialRequirementByKey(manifest, key)
		if declared && !requirement.Required {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("pack setup: credential source %q is missing", key)
	default:
		return nil, false, fmt.Errorf("pack setup: binding source %q is invalid", source)
	}
}
