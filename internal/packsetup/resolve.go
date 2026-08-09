package packsetup

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const MaxExpressionLength = 256

var setupExpressionRegex = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)

type ResolveContext struct {
	Input      interface{}
	Nodes      map[string]interface{}
	PackConfig map[string]interface{}
}

func ResolveParams(ctx ResolveContext, params map[string]interface{}) (map[string]interface{}, error) {
	if params == nil {
		return nil, nil
	}
	resolved := make(map[string]interface{}, len(params))
	for key, value := range params {
		next, err := resolveParamValue(ctx, value)
		if err != nil {
			return nil, fmt.Errorf("pack setup: param %q: %w", key, err)
		}
		resolved[key] = next
	}
	return resolved, nil
}

func resolveParamValue(ctx ResolveContext, value interface{}) (interface{}, error) {
	switch typed := value.(type) {
	case string:
		return resolveString(ctx, typed)
	case map[string]interface{}:
		resolved := make(map[string]interface{}, len(typed))
		for key, child := range typed {
			next, err := resolveParamValue(ctx, child)
			if err != nil {
				return nil, err
			}
			resolved[key] = next
		}
		return resolved, nil
	case []interface{}:
		resolved := make([]interface{}, len(typed))
		for i, child := range typed {
			next, err := resolveParamValue(ctx, child)
			if err != nil {
				return nil, err
			}
			resolved[i] = next
		}
		return resolved, nil
	default:
		return value, nil
	}
}

func resolveString(ctx ResolveContext, value string) (interface{}, error) {
	if matches := setupExpressionRegex.FindStringSubmatch(value); len(matches) == 2 && matches[0] == value {
		return resolveExpression(ctx, matches[1])
	}
	var firstErr error
	result := setupExpressionRegex.ReplaceAllStringFunc(value, func(match string) string {
		if firstErr != nil {
			return ""
		}
		matches := setupExpressionRegex.FindStringSubmatch(match)
		resolved, err := resolveExpression(ctx, matches[1])
		if err != nil {
			firstErr = err
			return ""
		}
		return stringifyInterpolated(resolved)
	})
	if firstErr != nil {
		return nil, firstErr
	}
	return result, nil
}

func resolveExpression(ctx ResolveContext, raw string) (interface{}, error) {
	expr := strings.TrimSpace(raw)
	if expr == "" || len(expr) > MaxExpressionLength {
		return nil, fmt.Errorf("invalid expression %q", boundedExpression(expr))
	}
	parts := strings.Split(expr, ".")
	for _, part := range parts {
		if !validPathPart(part) {
			return nil, fmt.Errorf("invalid expression %q", boundedExpression(expr))
		}
	}
	switch {
	case len(parts) >= 2 && parts[0] == "input":
		return valueAtPath(ctx.Input, parts[1:], expr)
	case len(parts) >= 3 && parts[0] == "nodes":
		root, ok := ctx.Nodes[parts[1]]
		if !ok {
			return nil, fmt.Errorf("missing expression path %q", boundedExpression(expr))
		}
		return valueAtPath(root, parts[2:], expr)
	case len(parts) >= 3 && parts[0] == "pack" && parts[1] == "config":
		return valueAtPath(ctx.PackConfig, parts[2:], expr)
	default:
		return nil, fmt.Errorf("unsupported expression %q", boundedExpression(expr))
	}
}

func valueAtPath(root interface{}, parts []string, expr string) (interface{}, error) {
	current := root
	for _, part := range parts {
		switch typed := current.(type) {
		case map[string]interface{}:
			next, ok := typed[part]
			if !ok {
				return nil, fmt.Errorf("missing expression path %q", boundedExpression(expr))
			}
			current = next
		case []interface{}:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, fmt.Errorf("missing expression path %q", boundedExpression(expr))
			}
			current = typed[index]
		default:
			return nil, fmt.Errorf("missing expression path %q", boundedExpression(expr))
		}
	}
	return current, nil
}

func stringifyInterpolated(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(data)
	}
}

func validPathPart(value string) bool {
	if value == "" || value == ".." {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '$' {
			continue
		}
		return false
	}
	return true
}

func boundedExpression(value string) string {
	if len(value) <= MaxExpressionLength {
		return value
	}
	return value[:MaxExpressionLength]
}

func splitBindingSource(source string) (string, string, bool) {
	if kind, key, ok := strings.Cut(source, "."); ok && (kind == "config" || kind == "credential") && key != "" {
		return kind, key, true
	}
	return "", "", false
}
