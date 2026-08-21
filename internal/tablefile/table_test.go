package tablefile

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestCSVReadWriteRoundTrip(t *testing.T) {
	input := "sku,qty,active\nA,2,true\nB,5,false\n"
	table, err := ReadCSV(strings.NewReader(input), ',', true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(table.Columns, []string{"sku", "qty", "active"}) {
		t.Fatalf("columns = %#v", table.Columns)
	}
	if len(table.Rows) != 2 || table.Rows[0]["sku"] != "A" || table.Rows[1]["qty"] != "5" {
		t.Fatalf("rows = %#v", table.Rows)
	}

	var out bytes.Buffer
	if err := WriteCSV(&out, table, ',', true); err != nil {
		t.Fatal(err)
	}
	roundTrip, err := ReadCSV(strings.NewReader(out.String()), ',', true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(roundTrip, table) {
		t.Fatalf("round trip = %#v, want %#v", roundTrip, table)
	}
}

func TestCSVNormalizesDuplicateAndBlankHeaders(t *testing.T) {
	table, err := ReadCSV(strings.NewReader("name,name,\na,b,c\n"), ',', true)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"name", "name_2", "column_3"}
	if !reflect.DeepEqual(table.Columns, want) {
		t.Fatalf("columns = %#v, want %#v", table.Columns, want)
	}
}

func TestCSVWithoutHeaderGetsStableColumnNames(t *testing.T) {
	table, err := ReadCSV(strings.NewReader("a;b\nc;d\n"), ';', false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(table.Columns, []string{"column_1", "column_2"}) {
		t.Fatalf("columns = %#v", table.Columns)
	}
	if table.Rows[1]["column_2"] != "d" {
		t.Fatalf("rows = %#v", table.Rows)
	}
}
