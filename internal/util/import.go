package util

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
)

// ImportOptions controls CSV import behaviour.
type ImportOptions struct {
	HasHeader bool
	BatchSize int // rows between progressFn calls; default 50
}

// ColumnHint carries the column metadata needed for mapping. It mirrors the
// relevant fields of database.ColumnInfo without creating an import cycle.
type ColumnHint struct {
	Name       string
	IsNullable bool
	HasDefault bool
	IsPK       bool
}

// csvColMap links a CSV column index to a destination table column name.
type csvColMap struct {
	csvIndex   int
	colName    string
	nullable   bool
	hasDefault bool
}

// MappingResult holds the validated column plan and any advisory warnings.
type MappingResult struct {
	Mappings []csvColMap
	Warnings []string // CSV cols skipped, nullable table cols omitted
}

// ParseCSVPreview opens path and reads up to n data rows (n=0 returns headers only).
// headers is non-nil only when hasHeader is true.
func ParseCSVPreview(path string, hasHeader bool, n int) (headers []string, rows [][]string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()

	r := csv.NewReader(f)
	r.LazyQuotes = true
	r.TrimLeadingSpace = true

	if hasHeader {
		headers, err = r.Read()
		if err != nil {
			if err == io.EOF {
				return headers, nil, nil
			}
			return nil, nil, fmt.Errorf("reading CSV header: %w", err)
		}
	}

	for i := 0; n == 0 || i < n; i++ {
		record, readErr := r.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return headers, rows, fmt.Errorf("reading CSV row %d: %w", i+1, readErr)
		}
		rows = append(rows, record)
	}

	return headers, rows, nil
}

// BuildColumnMapping cross-references csvHeaders against tableColumns.
// Returns an error only if a required (non-nullable, no default) table column
// has no matching CSV source. Warnings are advisory.
func BuildColumnMapping(csvHeaders []string, tableColumns []ColumnHint) (MappingResult, error) {
	var result MappingResult

	// Build a lookup: lowercase column name → column hint
	colByName := make(map[string]ColumnHint, len(tableColumns))
	for _, c := range tableColumns {
		colByName[strings.ToLower(c.Name)] = c
	}

	// Map CSV headers to table columns.
	matched := make(map[string]bool)
	for i, h := range csvHeaders {
		key := strings.ToLower(strings.TrimSpace(h))
		col, ok := colByName[key]
		if !ok {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("CSV column %q not found in table, skipping", h))
			continue
		}
		result.Mappings = append(result.Mappings, csvColMap{
			csvIndex:   i,
			colName:    col.Name,
			nullable:   col.IsNullable,
			hasDefault: col.HasDefault,
		})
		matched[strings.ToLower(col.Name)] = true
	}

	// Check for required table columns not covered by any CSV header.
	for _, col := range tableColumns {
		if matched[strings.ToLower(col.Name)] {
			continue
		}
		if col.IsNullable || col.HasDefault || col.IsPK {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("table column %q has no CSV source (nullable/default/pk — will be omitted)", col.Name))
			continue
		}
		return result, fmt.Errorf("required column %q is missing from the CSV and has no default", col.Name)
	}

	return result, nil
}

// countLines does a fast newline count to give ImportRows a total for progress.
// Returns -1 on any error.
func countLines(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return -1
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)
	n := 0
	for scanner.Scan() {
		n++
	}
	if scanner.Err() != nil {
		return -1
	}
	return n
}

// ImportRows streams path through a CSV reader, maps columns via mapping,
// and calls insertFn for each row. Per-row errors are collected without
// aborting the import. progressFn is called every opts.BatchSize rows;
// total is -1 when unknown.
func ImportRows(
	ctx context.Context,
	path string,
	opts ImportOptions,
	mapping MappingResult,
	progressFn func(imported, total int),
	insertFn func(map[string]any) error,
) (int, []error) {
	if opts.BatchSize <= 0 {
		opts.BatchSize = 50
	}

	total := countLines(path)
	if opts.HasHeader && total > 0 {
		total--
	}

	f, err := os.Open(path)
	if err != nil {
		return 0, []error{err}
	}
	defer func() { _ = f.Close() }()

	r := csv.NewReader(f)
	r.LazyQuotes = true
	r.TrimLeadingSpace = true

	if opts.HasHeader {
		if _, err := r.Read(); err != nil && err != io.EOF {
			return 0, []error{fmt.Errorf("reading CSV header: %w", err)}
		}
	}

	var errs []error
	imported := 0
	rowNum := 0

	for ctx.Err() == nil {
		record, readErr := r.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			errs = append(errs, fmt.Errorf("row %d: CSV read error: %w", rowNum+1, readErr))
			rowNum++
			continue
		}
		rowNum++

		row := make(map[string]any, len(mapping.Mappings))
		for _, m := range mapping.Mappings {
			var val any
			if m.csvIndex < len(record) {
				v := record[m.csvIndex]
				if v == "" && (m.nullable || m.hasDefault) {
					val = nil
				} else {
					val = v
				}
			}
			row[m.colName] = val
		}

		if err := insertFn(row); err != nil {
			errs = append(errs, fmt.Errorf("row %d: %w", rowNum, err))
		} else {
			imported++
		}

		if progressFn != nil && imported%opts.BatchSize == 0 {
			progressFn(imported, total)
		}
	}

	if progressFn != nil {
		progressFn(imported, total)
	}

	return imported, errs
}
