package tablefile

import (
	"archive/zip"
	"bytes"
	"io"
	"reflect"
	"testing"
)

func TestXLSXWriteReadRoundTrip(t *testing.T) {
	input := Table{
		Columns: []string{"sku", "qty", "active", "note"},
		Rows: []map[string]interface{}{
			{"sku": "A", "qty": 2, "active": true, "note": "hello & <world>"},
			{"sku": "B", "qty": 3.5, "active": false, "note": ""},
		},
	}
	data, err := WriteXLSX(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty XLSX output")
	}
	got, err := ReadXLSX(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Columns, input.Columns) {
		t.Fatalf("columns = %#v, want %#v", got.Columns, input.Columns)
	}
	if len(got.Rows) != 2 {
		t.Fatalf("rows = %#v", got.Rows)
	}
	if got.Rows[0]["sku"] != "A" || got.Rows[0]["qty"] != float64(2) || got.Rows[0]["active"] != true || got.Rows[0]["note"] != "hello & <world>" {
		t.Fatalf("first row = %#v", got.Rows[0])
	}
	if got.Rows[1]["qty"] != 3.5 || got.Rows[1]["active"] != false {
		t.Fatalf("second row = %#v", got.Rows[1])
	}
}

func TestXLSXReadSharedStrings(t *testing.T) {
	var buffer bytes.Buffer
	zw := zip.NewWriter(&buffer)
	files := map[string]string{
		"xl/sharedStrings.xml":     `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><si><t>sku</t></si><si><t>A</t></si></sst>`,
		"xl/worksheets/sheet1.xml": `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="inlineStr"><is><t>qty</t></is></c></row><row r="2"><c r="A2" t="s"><v>1</v></c><c r="B2"><v>4</v></c></row></sheetData></worksheet>`,
	}
	for name, content := range files {
		entry, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(entry, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	table, err := ReadXLSX(buffer.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(table.Columns, []string{"sku", "qty"}) || table.Rows[0]["sku"] != "A" || table.Rows[0]["qty"] != float64(4) {
		t.Fatalf("table = %#v", table)
	}
}

func TestXLSXRejectsInvalidArchive(t *testing.T) {
	if _, err := ReadXLSX([]byte("not a zip")); err == nil {
		t.Fatal("invalid XLSX accepted")
	}
}
