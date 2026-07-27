package nodes

import (
	"encoding/json"
	"math"
	"testing"
)

func TestParseDelaySecondsAcceptsIntegerInputs(t *testing.T) {
	cases := []struct {
		name  string
		value interface{}
		want  int
	}{
		{name: "string", value: "2", want: 2},
		{name: "int", value: int(2), want: 2},
		{name: "int32", value: int32(2), want: 2},
		{name: "int64", value: int64(2), want: 2},
		{name: "float32 integer", value: float32(2), want: 2},
		{name: "float64 integer", value: float64(2), want: 2},
		{name: "json number", value: json.Number("2"), want: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseDelaySeconds(tc.value)
			if err != nil {
				t.Fatalf("parseDelaySeconds returned error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestParseDelaySecondsRejectsInvalidInputs(t *testing.T) {
	cases := []struct {
		name  string
		value interface{}
	}{
		{name: "missing", value: nil},
		{name: "empty string", value: ""},
		{name: "non numeric string", value: "abc"},
		{name: "fractional string", value: "1.5"},
		{name: "fractional float32", value: float32(1.5)},
		{name: "fractional float64", value: float64(1.5)},
		{name: "zero", value: 0},
		{name: "negative", value: -1},
		{name: "too large", value: maxDelaySeconds + 1},
		{name: "nan", value: math.NaN()},
		{name: "infinity", value: math.Inf(1)},
		{name: "json fractional", value: json.Number("1.5")},
		{name: "unsupported type", value: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := parseDelaySeconds(tc.value); err == nil {
				t.Fatalf("parseDelaySeconds(%#v) = %d, want error", tc.value, got)
			}
		})
	}
}
