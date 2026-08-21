package nodes

import "testing"

func TestMaxAttemptsForNodeMixedOperationSafety(t *testing.T) {
	tests := []struct {
		name      string
		typeID    NodeType
		params    map[string]interface{}
		retryable bool
		want      int
	}{
		{name: "http get retries", typeID: TypeHTTPRequest, params: map[string]interface{}{"method": "GET"}, retryable: true, want: 3},
		{name: "http head retries", typeID: TypeHTTPRequest, params: map[string]interface{}{"method": "HEAD"}, retryable: true, want: 3},
		{name: "http post does not retry", typeID: TypeHTTPRequest, params: map[string]interface{}{"method": "POST"}, retryable: true, want: 1},
		{name: "http put conservative no retry", typeID: TypeHTTPRequest, params: map[string]interface{}{"method": "PUT"}, retryable: true, want: 1},
		{name: "postgres select retries", typeID: TypePostgresQuery, params: map[string]interface{}{"query_type": "SELECT"}, retryable: true, want: 3},
		{name: "postgres execute does not retry", typeID: TypePostgresQuery, params: map[string]interface{}{"query_type": "EXECUTE"}, retryable: true, want: 1},
		{name: "mysql select retries", typeID: TypeMySQLQuery, params: map[string]interface{}{"query_type": "SELECT"}, retryable: true, want: 3},
		{name: "mysql execute does not retry", typeID: TypeMySQLQuery, params: map[string]interface{}{"query_type": "EXECUTE"}, retryable: true, want: 1},
		{name: "sheets read retries", typeID: TypeGoogleSheets, params: map[string]interface{}{"action": "READ"}, retryable: true, want: 3},
		{name: "sheets append does not retry", typeID: TypeGoogleSheets, params: map[string]interface{}{"action": "APPEND"}, retryable: true, want: 1},
		{name: "drive list retries", typeID: TypeGoogleDrive, params: map[string]interface{}{"action": "LIST"}, retryable: true, want: 3},
		{name: "drive upload does not retry", typeID: TypeGoogleDrive, params: map[string]interface{}{"action": "UPLOAD"}, retryable: true, want: 1},
		{name: "mongo find retries", typeID: TypeMongoDBCommand, params: map[string]interface{}{"command": "FIND_ONE"}, retryable: true, want: 3},
		{name: "mongo insert does not retry", typeID: TypeMongoDBCommand, params: map[string]interface{}{"command": "INSERT_ONE"}, retryable: true, want: 1},
		{name: "mongo update does not retry", typeID: TypeMongoDBCommand, params: map[string]interface{}{"command": "UPDATE_ONE"}, retryable: true, want: 1},
		{name: "mongo delete does not retry", typeID: TypeMongoDBCommand, params: map[string]interface{}{"command": "DELETE_ONE"}, retryable: true, want: 1},
		{name: "redis get retries", typeID: TypeRedisCommand, params: map[string]interface{}{"command": "GET"}, retryable: true, want: 3},
		{name: "redis exists retries", typeID: TypeRedisCommand, params: map[string]interface{}{"command": "EXISTS"}, retryable: true, want: 3},
		{name: "redis hget retries", typeID: TypeRedisCommand, params: map[string]interface{}{"command": "HGET"}, retryable: true, want: 3},
		{name: "redis set does not retry", typeID: TypeRedisCommand, params: map[string]interface{}{"command": "SET"}, retryable: true, want: 1},
		{name: "redis lpush does not retry", typeID: TypeRedisCommand, params: map[string]interface{}{"command": "LPUSH"}, retryable: true, want: 1},
		{name: "legacy nonretryable remains single attempt", typeID: TypeHTTPRequest, params: map[string]interface{}{"method": "GET"}, retryable: false, want: 1},
		{name: "uniform retryable node keeps legacy behavior", typeID: TypeJSONTransform, params: map[string]interface{}{}, retryable: true, want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &Node{Type: tt.typeID, Params: tt.params}
			def := NodeDefinition{Retryable: tt.retryable}
			if got := MaxAttemptsForNode(node, def); got != tt.want {
				t.Fatalf("MaxAttemptsForNode() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestMaxAttemptsForNodeUsesResolvedOperationDefaults(t *testing.T) {
	tests := []struct {
		name   string
		typeID NodeType
		want   int
	}{
		{name: "http defaults to GET", typeID: TypeHTTPRequest, want: 3},
		{name: "sql defaults to SELECT", typeID: TypePostgresQuery, want: 3},
		{name: "sheets defaults to APPEND", typeID: TypeGoogleSheets, want: 1},
		{name: "drive defaults to LIST", typeID: TypeGoogleDrive, want: 3},
		{name: "mongo defaults to FIND_ONE", typeID: TypeMongoDBCommand, want: 3},
		{name: "redis defaults to GET", typeID: TypeRedisCommand, want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &Node{Type: tt.typeID, Params: map[string]interface{}{}}
			if got := MaxAttemptsForNode(node, NodeDefinition{Retryable: true}); got != tt.want {
				t.Fatalf("MaxAttemptsForNode() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestMaxAttemptsForNodeUsesResolvedOperation(t *testing.T) {
	ctx := NewExecutionContext("wf", "exec")
	ctx.SetOutput("$trigger", map[string]interface{}{
		"method":     "POST",
		"query_type": "EXECUTE",
	})

	tests := []struct {
		name   string
		typeID NodeType
		param  string
		path   string
	}{
		{name: "resolved HTTP POST is single attempt", typeID: TypeHTTPRequest, param: "method", path: "{{$trigger.method}}"},
		{name: "resolved SQL EXECUTE is single attempt", typeID: TypePostgresQuery, param: "query_type", path: "{{$trigger.query_type}}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := &Node{Type: tt.typeID, Params: map[string]interface{}{tt.param: tt.path}}
			evaluated := &Node{Type: raw.Type, Params: ResolveParams(ctx, raw.Params)}
			if got := MaxAttemptsForNode(evaluated, NodeDefinition{Retryable: true}); got != 1 {
				t.Fatalf("MaxAttemptsForNode() = %d, want 1", got)
			}
		})
	}
}
