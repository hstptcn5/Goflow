package jsoncontract

import (
	"strings"
	"testing"
)

func TestValidateDailyOpsContract(t *testing.T) {
	contract := dailyOpsContract(t)
	value, err := Decode([]byte(`{
		"report_date":"2026-08-09",
		"timezone":"Asia/Bangkok",
		"revenue":48250.75,
		"order_count":314,
		"cancelled_refunded_count":7,
		"low_stock_summary":"3 SKUs below threshold",
		"comparison_summary":"Revenue up 12.4% vs prior day",
		"future_field":{"allowed":true}
	}`))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	summary, err := Validate(value, contract)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if summary.RequiredFields != 7 || summary.ReportDate != "2026-08-09" {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestValidateDailyOpsContractRejectsMissingAndWrongTypes(t *testing.T) {
	contract := dailyOpsContract(t)
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "missing", body: `{"report_date":"2026-08-09"}`, want: "missing required field"},
		{name: "wrong number", body: validBody(`"revenue"`), want: `field "revenue" must be a JSON number`},
		{name: "negative integer", body: validBody(`-1`), want: `field "order_count" must be at least`},
		{name: "fractional integer", body: validBody(`1.5`), want: `field "order_count" must be an integer`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := Decode([]byte(tt.body))
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			_, err = Validate(value, contract)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, err)
			}
		})
	}
}

func dailyOpsContract(t *testing.T) Contract {
	t.Helper()
	contract, err := Parse(map[string]interface{}{"required": map[string]interface{}{
		"report_date":              map[string]interface{}{"type": "string", "non_empty": true},
		"timezone":                 map[string]interface{}{"type": "string", "non_empty": true},
		"revenue":                  map[string]interface{}{"type": "number"},
		"order_count":              map[string]interface{}{"type": "integer", "minimum": 0},
		"cancelled_refunded_count": map[string]interface{}{"type": "integer", "minimum": 0},
		"low_stock_summary":        map[string]interface{}{"type": "string"},
		"comparison_summary":       map[string]interface{}{"type": "string"},
	}})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return contract
}

func validBody(orderCount string) string {
	return `{
		"report_date":"2026-08-09",
		"timezone":"Asia/Bangkok",
		"revenue":` + func() string {
		if orderCount == `"revenue"` {
			return orderCount
		}
		return "48250.75"
	}() + `,
		"order_count":` + func() string {
		if orderCount == `"revenue"` {
			return "314"
		}
		return orderCount
	}() + `,
		"cancelled_refunded_count":7,
		"low_stock_summary":"low",
		"comparison_summary":"up"
	}`
}
