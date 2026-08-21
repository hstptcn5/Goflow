package nodes

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseSQLParameters(t *testing.T) {
	tests := []struct {
		name    string
		raw     interface{}
		want    []interface{}
		wantErr string
	}{
		{name: "missing", raw: nil, want: nil},
		{name: "structured", raw: []interface{}{"a", float64(2), true}, want: []interface{}{"a", float64(2), true}},
		{name: "json", raw: `["a",2,true,null]`, want: []interface{}{"a", float64(2), true, nil}},
		{name: "empty", raw: "  ", want: nil},
		{name: "object rejected", raw: `{"value":1}`, wantErr: "JSON array"},
		{name: "scalar rejected", raw: 42, wantErr: "must be an array"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSQLParameters(tt.raw)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSQLParameters() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseSQLParameters() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestSQLDefinitionExposesBoundParameters(t *testing.T) {
	for _, def := range []NodeDefinition{NewPostgresQueryExecutor().GetDefinition(), NewMySQLQueryExecutor().GetDefinition()} {
		found := false
		for _, param := range def.Params {
			if param.Name == "parameters" {
				found = true
				if param.Type != "json" || param.Required {
					t.Fatalf("%s parameters definition = %#v", def.Type, param)
				}
			}
		}
		if !found {
			t.Fatalf("%s does not expose parameters", def.Type)
		}
	}
}
