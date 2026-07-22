package nodes

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var placeholderRegex = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)

// ResolveParams resolves dynamic placeholder variables in the node parameters
func ResolveParams(ctx *ExecutionContext, params map[string]interface{}) map[string]interface{} {
	if params == nil {
		return nil
	}
	resolved := make(map[string]interface{})
	for k, v := range params {
		resolved[k] = resolveValue(ctx, v)
	}
	return resolved
}

func resolveValue(ctx *ExecutionContext, val interface{}) interface{} {
	switch v := val.(type) {
	case string:
		return evaluateString(ctx, v)
	case map[string]interface{}:
		res := make(map[string]interface{})
		for k, val := range v {
			res[k] = resolveValue(ctx, val)
		}
		return res
	case []interface{}:
		res := make([]interface{}, len(v))
		for i, val := range v {
			res[i] = resolveValue(ctx, val)
		}
		return res
	default:
		return v
	}
}

func evaluateString(ctx *ExecutionContext, s string) interface{} {
	// If the entire string is exactly one placeholder: "{{path}}", return the resolved value directly to preserve type
	if matches := placeholderRegex.FindStringSubmatch(s); len(matches) == 2 && matches[0] == s {
		val := resolvePath(ctx, matches[1])
		if val == nil {
			return ""
		}
		return val
	}

	// Otherwise, replace all occurrences inline
	return placeholderRegex.ReplaceAllStringFunc(s, func(match string) string {
		inner := placeholderRegex.FindStringSubmatch(match)[1]
		val := resolvePath(ctx, inner)
		if val == nil {
			return ""
		}
		switch v := val.(type) {
		case string:
			return v
		case []byte:
			return string(v)
		default:
			// For maps, slices, etc. stringify to JSON
			if jsonBytes, err := json.Marshal(v); err == nil {
				return string(jsonBytes)
			}
			return fmt.Sprintf("%v", v)
		}
	})
}

func resolvePath(ctx *ExecutionContext, pathStr string) interface{} {
	pathStr = strings.TrimSpace(pathStr)
	if pathStr == "" {
		return nil
	}

	parts := strings.Split(pathStr, ".")
	if len(parts) == 0 {
		return nil
	}

	nodeID := parts[0]

	ctx.mu.RLock()
	nodeOutput, exists := ctx.Outputs[nodeID]
	ctx.mu.RUnlock()

	if !exists {
		return nil
	}

	return getValueAtPath(nodeOutput, parts[1:])
}

func getValueAtPath(obj interface{}, path []string) interface{} {
	current := obj
	for _, part := range path {
		if current == nil {
			return nil
		}

		switch val := current.(type) {
		case map[string]interface{}:
			current = val[part]
		case []interface{}:
			var idx int
			if _, err := fmt.Sscan(part, &idx); err == nil && idx >= 0 && idx < len(val) {
				current = val[idx]
			} else {
				return nil
			}
		default:
			return nil
		}
	}
	return current
}
