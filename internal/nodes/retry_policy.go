package nodes

import "strings"

const (
	defaultNodeAttempts   = 1
	retryableNodeAttempts = 3
)

// MaxAttemptsForNode returns the number of execution attempts the engine may make
// for a resolved node operation. The legacy NodeDefinition.Retryable flag remains
// the coarse default for node types whose operations all share the same retry
// semantics; mixed read/write executors are narrowed here so mutating operations
// are never implicitly replayed.
func MaxAttemptsForNode(node *Node, definition NodeDefinition) int {
	if node == nil || !definition.Retryable {
		return defaultNodeAttempts
	}

	switch node.Type {
	case TypeHTTPRequest:
		method := upperNodeParam(node, "method", "GET")
		switch method {
		case "GET", "HEAD":
			return retryableNodeAttempts
		default:
			return defaultNodeAttempts
		}

	case TypePostgresQuery, TypeMySQLQuery:
		if upperNodeParam(node, "query_type", "SELECT") == "SELECT" {
			return retryableNodeAttempts
		}
		return defaultNodeAttempts

	case TypeGoogleSheets:
		if upperNodeParam(node, "action", "APPEND") == "READ" {
			return retryableNodeAttempts
		}
		return defaultNodeAttempts

	case TypeGoogleDrive:
		if upperNodeParam(node, "action", "LIST") == "LIST" {
			return retryableNodeAttempts
		}
		return defaultNodeAttempts

	case TypeMongoDBCommand:
		if upperNodeParam(node, "command", "FIND_ONE") == "FIND_ONE" {
			return retryableNodeAttempts
		}
		return defaultNodeAttempts

	case TypeRedisCommand:
		switch upperNodeParam(node, "command", "GET") {
		case "GET", "EXISTS", "HGET":
			return retryableNodeAttempts
		default:
			return defaultNodeAttempts
		}
	}

	return retryableNodeAttempts
}

func upperNodeParam(node *Node, name, fallback string) string {
	if node == nil {
		return strings.ToUpper(strings.TrimSpace(fallback))
	}
	value, _ := node.Params[name].(string)
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	return strings.ToUpper(strings.TrimSpace(value))
}
