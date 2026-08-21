package nodes

import (
	"path/filepath"
	"reflect"
	"testing"

	"goflow/internal/fileref"
	"goflow/internal/tablefile"
)

func TestTableFileCSVReadWrite(t *testing.T) {
	store := fileref.NewStore(filepath.Join(t.TempDir(), "store"))
	executor := NewTableFileExecutorWithStore(store)
	input := tablefile.Table{
		Columns: []string{"sku", "qty"},
		Rows: []map[string]interface{}{{"sku": "A", "qty": 2}, {"sku": "B", "qty": 3}},
	}
	written, err := executor.Execute(NewExecutionContext("wf", "exec"), &Node{Params: map[string]interface{}{
		"operation": "WRITE", "format": "CSV", "name": "orders.csv", "table": input,
	}})
	if err != nil {
		t.Fatal(err)
	}
	ref, ok := written.(fileref.Ref)
	if !ok || ref.Name != "orders.csv" {
		t.Fatalf("write output = %#v", written)
	}
	read, err := executor.Execute(NewExecutionContext("wf", "exec"), &Node{Params: map[string]interface{}{
		"operation": "READ", "format": "AUTO", "file_ref": ref,
	}})
	if err != nil {
		t.Fatal(err)
	}
	table := read.(tablefile.Table)
	if !reflect.DeepEqual(table.Columns, []string{"sku", "qty"}) || len(table.Rows) != 2 || table.Rows[0]["sku"] != "A" || table.Rows[0]["qty"] != "2" {
		t.Fatalf("table = %#v", table)
	}
}

func TestTableFileXLSXReadWrite(t *testing.T) {
	store := fileref.NewStore(filepath.Join(t.TempDir(), "store"))
	executor := NewTableFileExecutorWithStore(store)
	input := tablefile.Table{
		Columns: []string{"sku", "qty", "active"},
		Rows: []map[string]interface{}{{"sku": "A", "qty": 2.5, "active": true}},
	}
	written, err := executor.Execute(NewExecutionContext("wf", "exec"), &Node{Params: map[string]interface{}{
		"operation": "WRITE", "format": "XLSX", "name": "orders.xlsx", "table": input,
	}})
	if err != nil {
		t.Fatal(err)
	}
	ref := written.(fileref.Ref)
	read, err := executor.Execute(NewExecutionContext("wf", "exec"), &Node{Params: map[string]interface{}{
		"operation": "READ", "format": "AUTO", "file_ref": ref,
	}})
	if err != nil {
		t.Fatal(err)
	}
	table := read.(tablefile.Table)
	if table.Rows[0]["sku"] != "A" || table.Rows[0]["qty"] != 2.5 || table.Rows[0]["active"] != true {
		t.Fatalf("table = %#v", table)
	}
}
