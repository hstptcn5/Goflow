package tablefile

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
)

func ReadXLSX(data []byte) (Table, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return Table{}, fmt.Errorf("open XLSX: %w", err)
	}
	files := map[string]*zip.File{}
	for _, file := range reader.File {
		files[file.Name] = file
	}
	shared, err := readSharedStrings(files["xl/sharedStrings.xml"])
	if err != nil {
		return Table{}, err
	}
	sheet := files["xl/worksheets/sheet1.xml"]
	if sheet == nil {
		return Table{}, fmt.Errorf("XLSX first worksheet xl/worksheets/sheet1.xml is missing")
	}
	rc, err := sheet.Open()
	if err != nil {
		return Table{}, err
	}
	defer rc.Close()
	rows, err := readSheetRows(rc, shared)
	if err != nil {
		return Table{}, err
	}
	if len(rows) == 0 {
		return Table{Columns: []string{}, Rows: []map[string]interface{}{}}, nil
	}
	width := 0
	for _, row := range rows {
		if len(row) > width {
			width = len(row)
		}
	}
	if width > MaxColumns {
		return Table{}, fmt.Errorf("XLSX exceeds %d column limit", MaxColumns)
	}
	header := make([]string, width)
	for i := range header {
		if i < len(rows[0]) && rows[0][i] != nil {
			header[i] = fmt.Sprint(rows[0][i])
		}
	}
	columns := normalizeColumns(header, width)
	resultRows := make([]map[string]interface{}, 0, len(rows)-1)
	for _, values := range rows[1:] {
		row := make(map[string]interface{}, width)
		for i, column := range columns {
			if i < len(values) {
				row[column] = values[i]
			} else {
				row[column] = nil
			}
		}
		resultRows = append(resultRows, row)
		if len(resultRows) > MaxRows {
			return Table{}, fmt.Errorf("XLSX exceeds %d row limit", MaxRows)
		}
	}
	return Table{Columns: columns, Rows: resultRows}, nil
}

func WriteXLSX(table Table) ([]byte, error) {
	columns := table.Columns
	if len(columns) == 0 {
		columns = deriveColumns(table.Rows)
	}
	if len(columns) > MaxColumns {
		return nil, fmt.Errorf("XLSX exceeds %d column limit", MaxColumns)
	}
	if len(table.Rows) > MaxRows {
		return nil, fmt.Errorf("XLSX exceeds %d row limit", MaxRows)
	}

	var buffer bytes.Buffer
	zw := zip.NewWriter(&buffer)
	files := map[string]string{
		"[Content_Types].xml":        `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`,
		"_rels/.rels":                `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`,
	}
	for name, content := range files {
		entry, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := io.WriteString(entry, content); err != nil {
			return nil, err
		}
	}

	sheetEntry, err := zw.Create("xl/worksheets/sheet1.xml")
	if err != nil {
		return nil, err
	}
	if _, err := io.WriteString(sheetEntry, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`); err != nil {
		return nil, err
	}
	writeRow := func(rowNumber int, values []interface{}) error {
		if _, err := fmt.Fprintf(sheetEntry, `<row r="%d">`, rowNumber); err != nil {
			return err
		}
		for i, value := range values {
			ref := columnName(i+1) + strconv.Itoa(rowNumber)
			if err := writeXLSXCell(sheetEntry, ref, value); err != nil {
				return err
			}
		}
		_, err := io.WriteString(sheetEntry, `</row>`)
		return err
	}
	headerValues := make([]interface{}, len(columns))
	for i, column := range columns {
		headerValues[i] = column
	}
	if err := writeRow(1, headerValues); err != nil {
		return nil, err
	}
	for rowIndex, row := range table.Rows {
		values := make([]interface{}, len(columns))
		for i, column := range columns {
			values[i] = row[column]
		}
		if err := writeRow(rowIndex+2, values); err != nil {
			return nil, err
		}
	}
	if _, err := io.WriteString(sheetEntry, `</sheetData></worksheet>`); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func readSharedStrings(file *zip.File) ([]string, error) {
	if file == nil {
		return nil, nil
	}
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	decoder := xml.NewDecoder(rc)
	var result []string
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "si" {
			continue
		}
		var parts []string
		for {
			token, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			switch typed := token.(type) {
			case xml.StartElement:
				if typed.Name.Local == "t" {
					var text string
					if err := decoder.DecodeElement(&text, &typed); err != nil {
						return nil, err
					}
					parts = append(parts, text)
				}
			case xml.EndElement:
				if typed.Name.Local == "si" {
					result = append(result, strings.Join(parts, ""))
					goto nextSharedString
				}
			}
		}
	nextSharedString:
	}
	return result, nil
}

func readSheetRows(reader io.Reader, shared []string) ([][]interface{}, error) {
	decoder := xml.NewDecoder(reader)
	var rows [][]interface{}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "row" {
			continue
		}
		row := []interface{}{}
		for {
			token, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			switch typed := token.(type) {
			case xml.StartElement:
				if typed.Name.Local != "c" {
					continue
				}
				var cell struct {
					Ref    string `xml:"r,attr"`
					Type   string `xml:"t,attr"`
					Value  string `xml:"v"`
					Inline string `xml:"is>t"`
				}
				if err := decoder.DecodeElement(&cell, &typed); err != nil {
					return nil, err
				}
				index := columnIndex(cell.Ref)
				if index < 0 || index >= MaxColumns {
					return nil, fmt.Errorf("XLSX cell reference %q exceeds supported column range", cell.Ref)
				}
				for len(row) <= index {
					row = append(row, nil)
				}
				row[index] = decodeXLSXCell(cell.Type, cell.Value, cell.Inline, shared)
			case xml.EndElement:
				if typed.Name.Local == "row" {
					rows = append(rows, row)
					if len(rows) > MaxRows+1 {
						return nil, fmt.Errorf("XLSX exceeds %d row limit", MaxRows)
					}
					goto nextRow
				}
			}
		}
	nextRow:
	}
	return rows, nil
}

func decodeXLSXCell(cellType, value, inline string, shared []string) interface{} {
	switch cellType {
	case "s":
		index, err := strconv.Atoi(strings.TrimSpace(value))
		if err == nil && index >= 0 && index < len(shared) {
			return shared[index]
		}
		return value
	case "inlineStr", "str":
		if inline != "" {
			return inline
		}
		return value
	case "b":
		return strings.TrimSpace(value) == "1" || strings.EqualFold(strings.TrimSpace(value), "true")
	default:
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil
		}
		if number, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return number
		}
		return value
	}
}

func writeXLSXCell(writer io.Writer, ref string, value interface{}) error {
	if value == nil {
		_, err := fmt.Fprintf(writer, `<c r="%s"/>`, ref)
		return err
	}
	switch typed := value.(type) {
	case bool:
		bit := "0"
		if typed {
			bit = "1"
		}
		_, err := fmt.Fprintf(writer, `<c r="%s" t="b"><v>%s</v></c>`, ref, bit)
		return err
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		_, err := fmt.Fprintf(writer, `<c r="%s"><v>%v</v></c>`, ref, typed)
		return err
	default:
		var escaped bytes.Buffer
		if err := xml.EscapeText(&escaped, []byte(fmt.Sprint(value))); err != nil {
			return err
		}
		_, err := fmt.Fprintf(writer, `<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, ref, escaped.String())
		return err
	}
}

func columnIndex(ref string) int {
	letters := ""
	for _, r := range ref {
		if unicode.IsLetter(r) {
			letters += string(unicode.ToUpper(r))
		} else {
			break
		}
	}
	if letters == "" {
		return -1
	}
	value := 0
	for _, r := range letters {
		value = value*26 + int(r-'A'+1)
	}
	return value - 1
}

func columnName(index int) string {
	if index <= 0 {
		return "A"
	}
	name := ""
	for index > 0 {
		index--
		name = string(rune('A'+index%26)) + name
		index /= 26
	}
	return name
}
