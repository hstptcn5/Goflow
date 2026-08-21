package tablefile

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strings"
)

const (
	MaxRows    = 100000
	MaxColumns = 512
)

type Table struct {
	Columns []string                 `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
}

func ReadCSV(reader io.Reader, delimiter rune, hasHeader bool) (Table, error) {
	if delimiter == 0 {
		delimiter = ','
	}
	r := csv.NewReader(reader)
	r.Comma = delimiter
	r.FieldsPerRecord = -1
	var records [][]string
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Table{}, err
		}
		if len(record) > MaxColumns {
			return Table{}, fmt.Errorf("CSV row exceeds %d column limit", MaxColumns)
		}
		records = append(records, record)
		if len(records) > MaxRows+1 {
			return Table{}, fmt.Errorf("CSV exceeds %d row limit", MaxRows)
		}
	}
	if len(records) == 0 {
		return Table{Columns: []string{}, Rows: []map[string]interface{}{}}, nil
	}
	width := 0
	for _, record := range records {
		if len(record) > width {
			width = len(record)
		}
	}
	columns := make([]string, width)
	start := 0
	if hasHeader {
		columns = normalizeColumns(records[0], width)
		start = 1
	} else {
		for i := range columns {
			columns[i] = fmt.Sprintf("column_%d", i+1)
		}
	}
	rows := make([]map[string]interface{}, 0, len(records)-start)
	for _, record := range records[start:] {
		row := make(map[string]interface{}, width)
		for i, column := range columns {
			value := ""
			if i < len(record) {
				value = record[i]
			}
			row[column] = value
		}
		rows = append(rows, row)
	}
	return Table{Columns: columns, Rows: rows}, nil
}

func WriteCSV(writer io.Writer, table Table, delimiter rune, includeHeader bool) error {
	if delimiter == 0 {
		delimiter = ','
	}
	columns := table.Columns
	if len(columns) == 0 {
		columns = deriveColumns(table.Rows)
	}
	if len(columns) > MaxColumns {
		return fmt.Errorf("CSV exceeds %d column limit", MaxColumns)
	}
	if len(table.Rows) > MaxRows {
		return fmt.Errorf("CSV exceeds %d row limit", MaxRows)
	}
	w := csv.NewWriter(writer)
	w.Comma = delimiter
	if includeHeader && len(columns) > 0 {
		if err := w.Write(columns); err != nil {
			return err
		}
	}
	for _, row := range table.Rows {
		record := make([]string, len(columns))
		for i, column := range columns {
			value := row[column]
			if value != nil {
				record[i] = fmt.Sprint(value)
			}
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func normalizeColumns(header []string, width int) []string {
	columns := make([]string, width)
	seen := map[string]int{}
	for i := 0; i < width; i++ {
		name := ""
		if i < len(header) {
			name = strings.TrimSpace(header[i])
		}
		if name == "" {
			name = fmt.Sprintf("column_%d", i+1)
		}
		base := name
		seen[base]++
		if seen[base] > 1 {
			name = fmt.Sprintf("%s_%d", base, seen[base])
		}
		columns[i] = name
	}
	return columns
}

func deriveColumns(rows []map[string]interface{}) []string {
	set := map[string]struct{}{}
	for _, row := range rows {
		for key := range row {
			set[key] = struct{}{}
		}
	}
	columns := make([]string, 0, len(set))
	for key := range set {
		columns = append(columns, key)
	}
	sort.Strings(columns)
	return columns
}
