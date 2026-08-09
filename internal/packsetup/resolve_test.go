package packsetup

import (
	"reflect"
	"strings"
	"testing"
)

func TestResolveParamsPathOnlyExpressions(t *testing.T) {
	ctx := ResolveContext{
		Input: map[string]interface{}{
			"store_name": "Demo Store",
			"items":      []interface{}{"first"},
		},
		Nodes: map[string]interface{}{
			"fetch": map[string]interface{}{
				"data": map[string]interface{}{
					"total": 42,
				},
			},
		},
		PackConfig: map[string]interface{}{
			"report_title": "Daily Report",
			"threshold":    10,
		},
	}
	params := map[string]interface{}{
		"title":       "{{ pack.config.report_title }}",
		"threshold":   "{{pack.config.threshold}}",
		"summary":     "{{input.store_name}} total {{nodes.fetch.data.total}}",
		"first_item":  "{{input.items.0}}",
		"nested":      map[string]interface{}{"value": "{{nodes.fetch.data}}"},
		"literal_num": 7,
	}
	resolved, err := ResolveParams(ctx, params)
	if err != nil {
		t.Fatalf("ResolveParams failed: %v", err)
	}
	if resolved["title"] != "Daily Report" || resolved["threshold"] != 10 {
		t.Fatalf("expected exact expressions to preserve values, got %#v", resolved)
	}
	if resolved["summary"] != "Demo Store total 42" || resolved["first_item"] != "first" {
		t.Fatalf("unexpected interpolated strings: %#v", resolved)
	}
	expectedNested := map[string]interface{}{"value": map[string]interface{}{"total": 42}}
	if !reflect.DeepEqual(resolved["nested"], expectedNested) {
		t.Fatalf("unexpected nested value: %#v", resolved["nested"])
	}
}

func TestResolveParamsRejectsUnsupportedOrMissingExpressions(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "credential blocked", value: "{{credentials.telegram.token}}", want: "unsupported expression"},
		{name: "env blocked", value: "{{env.HOME}}", want: "unsupported expression"},
		{name: "function blocked", value: "{{nodes.fetch.data.join()}}", want: "invalid expression"},
		{name: "missing input", value: "{{input.missing}}", want: "missing expression path"},
		{name: "missing node", value: "{{nodes.fetch.data.total}}", want: "missing expression path"},
		{name: "missing config", value: "{{pack.config.missing}}", want: "missing expression path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResolveParams(ResolveContext{Input: map[string]interface{}{}, Nodes: map[string]interface{}{}, PackConfig: map[string]interface{}{}}, map[string]interface{}{"value": tt.value})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q error, got %v", tt.want, err)
			}
		})
	}
}

func TestResolveParamsDoesNotEchoResolvedValuesOnError(t *testing.T) {
	secretLike := "sk-secret-value"
	_, err := ResolveParams(ResolveContext{
		Input: map[string]interface{}{"present": secretLike},
	}, map[string]interface{}{
		"value": "{{input.present.missing}}",
	})
	if err == nil {
		t.Fatalf("expected missing path error")
	}
	if strings.Contains(err.Error(), secretLike) {
		t.Fatalf("error echoed data value: %v", err)
	}
}
