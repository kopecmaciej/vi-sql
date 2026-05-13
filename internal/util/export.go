package util

import (
	"compress/gzip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/tealeg/xlsx"
)

type ExportFormat string

const (
	ExportCSV       ExportFormat = "CSV"
	ExportJSON      ExportFormat = "JSON"
	ExportSQLInsert ExportFormat = "SQL INSERT"
	ExportMarkdown  ExportFormat = "MARKDOWN"
	ExportText      ExportFormat = "TEXT"
	ExportXLSX      ExportFormat = "XLSX"
)

// ExportOptions controls optional behaviour for each format.
type ExportOptions struct {
	IncludeHeaders bool   // CSV / Markdown: emit the header row
	PrettyPrint    bool   // JSON: use indented output
	Compress       bool   // all formats: wrap output in gzip
	SheetName      string // XLSX: worksheet name (defaults to "Sheet1")
}

// ExportRows writes rows to w in the requested format.
// columns controls field order; schema and table are used for SQL INSERT only.
// rows is []map[string]any (same underlying type as database.Row).
func ExportRows(w io.Writer, format ExportFormat, columns []string, rows []map[string]any, schema, table string, opts ExportOptions) error {
	out := w
	if opts.Compress {
		gz := gzip.NewWriter(w)
		defer func() { _ = gz.Close() }()
		out = gz
	}

	switch format {
	case ExportCSV:
		return exportCSV(out, columns, rows, opts.IncludeHeaders)
	case ExportJSON:
		return exportJSON(out, columns, rows, opts.PrettyPrint)
	case ExportSQLInsert:
		return exportSQLInsert(out, columns, rows, schema, table)
	case ExportMarkdown:
		return exportMarkdown(out, columns, rows, opts.IncludeHeaders)
	case ExportText:
		return exportText(out, columns, rows, opts.IncludeHeaders)
	case ExportXLSX:
		return exportXLSX(out, columns, rows, opts.SheetName)
	default:
		return fmt.Errorf("unknown export format: %s", format)
	}
}

func exportCSV(w io.Writer, columns []string, rows []map[string]any, includeHeaders bool) error {
	cw := csv.NewWriter(w)
	if includeHeaders {
		if err := cw.Write(columns); err != nil {
			return err
		}
	}
	record := make([]string, len(columns))
	for _, row := range rows {
		for i, col := range columns {
			v := row[col]
			if v == nil {
				record[i] = ""
			} else {
				record[i] = stringifyValue(v)
			}
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func exportJSON(w io.Writer, columns []string, rows []map[string]any, pretty bool) error {
	out := make([]map[string]any, len(rows))
	for i, row := range rows {
		obj := make(map[string]any, len(columns))
		for _, col := range columns {
			obj[col] = AsJSONValue(row[col])
		}
		out[i] = obj
	}

	var enc []byte
	var err error
	if pretty {
		enc, err = json.MarshalIndent(out, "", "  ")
	} else {
		enc, err = json.Marshal(out)
	}
	if err != nil {
		return err
	}
	_, err = w.Write(enc)
	return err
}

func exportSQLInsert(w io.Writer, columns []string, rows []map[string]any, schema, table string) error {
	quotedCols := make([]string, len(columns))
	for i, col := range columns {
		quotedCols[i] = fmt.Sprintf(`"%s"`, col)
	}
	colList := strings.Join(quotedCols, ", ")

	var target string
	if schema != "" {
		target = fmt.Sprintf(`"%s"."%s"`, schema, table)
	} else {
		target = fmt.Sprintf(`"%s"`, table)
	}

	for _, row := range rows {
		vals := make([]string, len(columns))
		for i, col := range columns {
			v := row[col]
			vals[i] = sqlLiteral(v)
		}
		_, err := fmt.Fprintf(w, "INSERT INTO %s (%s) VALUES (%s);\n",
			target, colList, strings.Join(vals, ", "))
		if err != nil {
			return err
		}
	}
	return nil
}

func exportMarkdown(w io.Writer, columns []string, rows []map[string]any, includeHeaders bool) error {
	colWidths := make([]int, len(columns))
	for i, col := range columns {
		colWidths[i] = len(col)
	}
	for _, row := range rows {
		for i, col := range columns {
			if n := len(stringifyValue(row[col])); n > colWidths[i] {
				colWidths[i] = n
			}
		}
	}

	pad := func(s string, width int) string {
		if len(s) >= width {
			return s
		}
		return s + strings.Repeat(" ", width-len(s))
	}

	if includeHeaders {
		parts := make([]string, len(columns))
		for i, col := range columns {
			parts[i] = pad(col, colWidths[i])
		}
		if _, err := fmt.Fprintf(w, "| %s |\n", strings.Join(parts, " | ")); err != nil {
			return err
		}
		seps := make([]string, len(columns))
		for i, w := range colWidths {
			seps[i] = strings.Repeat("-", w)
		}
		if _, err := fmt.Fprintf(w, "| %s |\n", strings.Join(seps, " | ")); err != nil {
			return err
		}
	}

	for _, row := range rows {
		parts := make([]string, len(columns))
		for i, col := range columns {
			parts[i] = pad(stringifyValue(row[col]), colWidths[i])
		}
		if _, err := fmt.Fprintf(w, "| %s |\n", strings.Join(parts, " | ")); err != nil {
			return err
		}
	}
	return nil
}

func exportText(w io.Writer, columns []string, rows []map[string]any, includeHeaders bool) error {
	colWidths := make([]int, len(columns))
	for i, col := range columns {
		colWidths[i] = len(col)
	}
	for _, row := range rows {
		for i, col := range columns {
			if n := len(stringifyValue(row[col])); n > colWidths[i] {
				colWidths[i] = n
			}
		}
	}

	pad := func(s string, width int) string {
		if len(s) >= width {
			return s
		}
		return s + strings.Repeat(" ", width-len(s))
	}

	if includeHeaders {
		parts := make([]string, len(columns))
		for i, col := range columns {
			parts[i] = pad(col, colWidths[i])
		}
		if _, err := fmt.Fprintf(w, "%s\n", strings.Join(parts, "  ")); err != nil {
			return err
		}
		seps := make([]string, len(columns))
		for i, width := range colWidths {
			seps[i] = strings.Repeat("-", width)
		}
		if _, err := fmt.Fprintf(w, "%s\n", strings.Join(seps, "  ")); err != nil {
			return err
		}
	}

	for _, row := range rows {
		parts := make([]string, len(columns))
		for i, col := range columns {
			parts[i] = pad(stringifyValue(row[col]), colWidths[i])
		}
		if _, err := fmt.Fprintf(w, "%s\n", strings.Join(parts, "  ")); err != nil {
			return err
		}
	}
	return nil
}

func exportXLSX(w io.Writer, columns []string, rows []map[string]any, sheetName string) error {
	if sheetName == "" {
		sheetName = "Sheet1"
	}
	f := xlsx.NewFile()
	sheet, err := f.AddSheet(sheetName)
	if err != nil {
		return err
	}
	headerRow := sheet.AddRow()
	for _, col := range columns {
		headerRow.AddCell().SetString(col)
	}
	for _, row := range rows {
		r := sheet.AddRow()
		for _, col := range columns {
			r.AddCell().SetString(stringifyValue(row[col]))
		}
	}
	return f.Write(w)
}

// stringifyValue converts any value to its string representation.
// nil returns empty string (for CSV); use sqlLiteral for SQL NULL handling.
func stringifyValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// AsJSONValue returns json.RawMessage for valid JSON object or arrays strings.
func AsJSONValue(v any) any {
	s, ok := v.(string)
	if !ok || len(s) == 0 || (s[0] != '{' && s[0] != '[') {
		return v
	}
	if json.Valid([]byte(s)) {
		return json.RawMessage(s)
	}
	return v
}

// sqlLiteral formats a value as a SQL literal: NULL, unquoted number/bool, or single-quoted string.
func sqlLiteral(v any) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return fmt.Sprintf("%v", val)
	case []byte:
		return "'" + strings.ReplaceAll(string(val), "'", "''") + "'"
	default:
		s := stringifyValue(v)
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}
}
