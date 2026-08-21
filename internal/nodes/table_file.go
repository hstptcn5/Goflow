package nodes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"goflow/internal/fileref"
	"goflow/internal/tablefile"
)

type TableFileExecutor struct {
	store *fileref.Store
}

func NewTableFileExecutor() *TableFileExecutor { return &TableFileExecutor{store: fileref.DefaultStore()} }
func NewTableFileExecutorWithStore(store *fileref.Store) *TableFileExecutor {
	if store == nil {
		store = fileref.DefaultStore()
	}
	return &TableFileExecutor{store: store}
}

func normalizeTableFileOperation(raw interface{}) (string, error) {
	op := strings.ToUpper(strings.TrimSpace(conditionValueString(raw)))
	if op == "" {
		op = "READ"
	}
	if op != "READ" && op != "WRITE" {
		return "", fmt.Errorf("table file operation must be READ or WRITE")
	}
	return op, nil
}

func normalizeTableFormat(raw interface{}, name string) (string, error) {
	format := strings.ToUpper(strings.TrimSpace(conditionValueString(raw)))
	if format == "" || format == "AUTO" {
		ext := strings.ToLower(filepath.Ext(name))
		switch ext {
		case ".csv":
			format = "CSV"
		case ".xlsx":
			format = "XLSX"
		default:
			return "", fmt.Errorf("table format could not be inferred; choose CSV or XLSX")
		}
	}
	if format != "CSV" && format != "XLSX" {
		return "", fmt.Errorf("table format must be CSV or XLSX")
	}
	return format, nil
}

func tableFromValue(raw interface{}) (tablefile.Table, error) {
	if raw == nil {
		return tablefile.Table{}, fmt.Errorf("table data is required")
	}
	if typed, ok := raw.(tablefile.Table); ok {
		return typed, nil
	}
	if text, ok := raw.(string); ok {
		text = strings.TrimSpace(text)
		if text == "" {
			return tablefile.Table{}, fmt.Errorf("table data is required")
		}
		var table tablefile.Table
		if err := json.Unmarshal([]byte(text), &table); err != nil {
			return tablefile.Table{}, fmt.Errorf("table data must be a Table JSON object: %w", err)
		}
		return table, nil
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return tablefile.Table{}, fmt.Errorf("table data could not be encoded: %w", err)
	}
	var table tablefile.Table
	if err := json.Unmarshal(encoded, &table); err != nil {
		return tablefile.Table{}, fmt.Errorf("table data must contain columns and rows: %w", err)
	}
	return table, nil
}

func csvDelimiter(raw interface{}) (rune, error) {
	text := conditionValueString(raw)
	if text == "" {
		return ',', nil
	}
	runes := []rune(text)
	if len(runes) != 1 {
		return 0, fmt.Errorf("CSV delimiter must be exactly one character")
	}
	return runes[0], nil
}

func (e *TableFileExecutor) Execute(ctx *ExecutionContext, node *Node) (interface{}, error) {
	op, err := normalizeTableFileOperation(node.Params["operation"])
	if err != nil {
		return nil, err
	}
	if op == "READ" {
		ref, err := fileref.Parse(node.Params["file_ref"])
		if err != nil {
			return nil, err
		}
		format, err := normalizeTableFormat(node.Params["format"], ref.Name)
		if err != nil {
			return nil, err
		}
		data, err := e.store.ReadAll(ref)
		if err != nil {
			return nil, err
		}
		switch format {
		case "CSV":
			delimiter, err := csvDelimiter(node.Params["delimiter"])
			if err != nil {
				return nil, err
			}
			hasHeader := true
			if raw, exists := node.Params["has_header"]; exists {
				if typed, ok := raw.(bool); ok {
					hasHeader = typed
				}
			}
			table, err := tablefile.ReadCSV(bytes.NewReader(data), delimiter, hasHeader)
			if err != nil {
				return nil, err
			}
			return table, nil
		case "XLSX":
			return tablefile.ReadXLSX(data)
		}
	}

	table, err := tableFromValue(node.Params["table"])
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(conditionValueString(node.Params["name"]))
	format, err := normalizeTableFormat(node.Params["format"], name)
	if err != nil {
		return nil, err
	}
	if name == "" {
		if format == "CSV" {
			name = "table.csv"
		} else {
			name = "table.xlsx"
		}
	}
	var data []byte
	var mimeType string
	switch format {
	case "CSV":
		delimiter, err := csvDelimiter(node.Params["delimiter"])
		if err != nil {
			return nil, err
		}
		includeHeader := true
		if raw, exists := node.Params["include_header"]; exists {
			if typed, ok := raw.(bool); ok {
				includeHeader = typed
			}
		}
		var buffer bytes.Buffer
		if err := tablefile.WriteCSV(&buffer, table, delimiter, includeHeader); err != nil {
			return nil, err
		}
		data = buffer.Bytes()
		mimeType = "text/csv"
	case "XLSX":
		data, err = tablefile.WriteXLSX(table)
		if err != nil {
			return nil, err
		}
		mimeType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}
	return e.store.PutBytes(name, mimeType, data)
}

func (e *TableFileExecutor) Validate(node *Node) error {
	_, err := normalizeTableFileOperation(node.Params["operation"])
	return err
}

func (e *TableFileExecutor) GetDefinition() NodeDefinition {
	return NodeDefinition{
		Type: TypeTableFile, Name: "Table File", Description: "Reads and writes structured CSV/XLSX tables through managed FileRefs", Icon: "Table", Category: "DATA", Retryable: false,
		Params: []ParamDefinition{
			{Name: "operation", Label: "Operation", Type: "select", Default: "READ", Options: []string{"READ", "WRITE"}, Required: true},
			{Name: "format", Label: "Format", Type: "select", Default: "AUTO", Options: []string{"AUTO", "CSV", "XLSX"}, Required: true},
			{Name: "file_ref", Label: "Input FileRef", Type: "json", Default: "", Required: false, Description: "Managed CSV/XLSX FileRef for READ"},
			{Name: "table", Label: "Table", Type: "json", Default: "", Required: false, Description: "Table object with columns and rows for WRITE"},
			{Name: "name", Label: "Output Name", Type: "text", Default: "", Required: false, Description: "Filename for WRITE"},
			{Name: "delimiter", Label: "CSV Delimiter", Type: "text", Default: ",", Required: false},
			{Name: "has_header", Label: "CSV Has Header", Type: "boolean", Default: true, Required: false},
			{Name: "include_header", Label: "CSV Include Header", Type: "boolean", Default: true, Required: false},
		},
	}
}
