package util

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTempCSV(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.csv")
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))
	return path
}

func TestParseCSVPreviewWithHeader(t *testing.T) {
	path := writeTempCSV(t, "id,name,age\n1,Alice,30\n2,Bob,25\n")

	headers, rows, err := ParseCSVPreview(path, true, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{"id", "name", "age"}, headers)
	assert.Len(t, rows, 2)
	assert.Equal(t, []string{"1", "Alice", "30"}, rows[0])
}

func TestParseCSVPreviewWithoutHeader(t *testing.T) {
	path := writeTempCSV(t, "1,Alice,30\n2,Bob,25\n")

	headers, rows, err := ParseCSVPreview(path, false, 10)
	require.NoError(t, err)
	assert.Nil(t, headers)
	assert.Len(t, rows, 2)
}

func TestParseCSVPreviewRowLimit(t *testing.T) {
	path := writeTempCSV(t, "id,name\n1,Alice\n2,Bob\n3,Charlie\n")

	_, rows, err := ParseCSVPreview(path, true, 2)
	require.NoError(t, err)
	assert.Len(t, rows, 2)
}

func TestParseCSVPreviewZeroLimitReturnsAll(t *testing.T) {
	path := writeTempCSV(t, "id,name\n1,Alice\n2,Bob\n3,Charlie\n")

	_, rows, err := ParseCSVPreview(path, true, 0)
	require.NoError(t, err)
	assert.Len(t, rows, 3)
}

func TestParseCSVPreviewNonexistentFile(t *testing.T) {
	_, _, err := ParseCSVPreview("/nonexistent/path/data.csv", true, 10)
	assert.Error(t, err)
}

func TestBuildColumnMappingAllColumnsMatch(t *testing.T) {
	headers := []string{"id", "name", "age"}
	columns := []ColumnHint{
		{Name: "id", IsPK: true},
		{Name: "name"},
		{Name: "age", IsNullable: true},
	}

	result, err := BuildColumnMapping(headers, columns)
	require.NoError(t, err)
	assert.Len(t, result.Mappings, 3)
	assert.Empty(t, result.Warnings)
}

func TestBuildColumnMappingExtraCSVColumnProducesWarning(t *testing.T) {
	headers := []string{"id", "name", "extra_col"}
	columns := []ColumnHint{
		{Name: "id", IsPK: true},
		{Name: "name"},
	}

	result, err := BuildColumnMapping(headers, columns)
	require.NoError(t, err)
	assert.Len(t, result.Mappings, 2)
	assert.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "extra_col")
}

func TestBuildColumnMappingMissingNullableColumnProducesWarning(t *testing.T) {
	headers := []string{"id"}
	columns := []ColumnHint{
		{Name: "id", IsPK: true},
		{Name: "description", IsNullable: true},
	}

	result, err := BuildColumnMapping(headers, columns)
	require.NoError(t, err)
	assert.Len(t, result.Mappings, 1)
	assert.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "description")
}

func TestBuildColumnMappingMissingRequiredColumnReturnsError(t *testing.T) {
	headers := []string{"id"}
	columns := []ColumnHint{
		{Name: "id", IsPK: true},
		{Name: "email"},
	}

	_, err := BuildColumnMapping(headers, columns)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email")
}

func TestBuildColumnMappingHeaderMatchingIsCaseInsensitive(t *testing.T) {
	headers := []string{"ID", "Name"}
	columns := []ColumnHint{
		{Name: "id", IsPK: true},
		{Name: "name"},
	}

	result, err := BuildColumnMapping(headers, columns)
	require.NoError(t, err)
	assert.Len(t, result.Mappings, 2)
}

func TestImportRowsSuccessful(t *testing.T) {
	path := writeTempCSV(t, "id,name\n1,Alice\n2,Bob\n")
	headers := []string{"id", "name"}
	columns := []ColumnHint{{Name: "id", IsPK: true}, {Name: "name"}}
	mapping, err := BuildColumnMapping(headers, columns)
	require.NoError(t, err)

	var inserted []map[string]any
	imported, errs := ImportRows(context.Background(), path, ImportOptions{HasHeader: true}, mapping,
		nil,
		func(row map[string]any) error {
			inserted = append(inserted, row)
			return nil
		},
	)

	assert.Empty(t, errs)
	assert.Equal(t, 2, imported)
	assert.Len(t, inserted, 2)
	assert.Equal(t, "Alice", inserted[0]["name"])
}

func TestImportRowsPerRowErrorsCollectedWithoutAborting(t *testing.T) {
	path := writeTempCSV(t, "id,name\n1,Alice\n2,Bob\n3,Charlie\n")
	headers := []string{"id", "name"}
	columns := []ColumnHint{{Name: "id", IsPK: true}, {Name: "name"}}
	mapping, err := BuildColumnMapping(headers, columns)
	require.NoError(t, err)

	callCount := 0
	imported, errs := ImportRows(context.Background(), path, ImportOptions{HasHeader: true}, mapping,
		nil,
		func(row map[string]any) error {
			callCount++
			if row["id"] == "2" {
				return assert.AnError
			}
			return nil
		},
	)

	assert.Equal(t, 3, callCount)
	assert.Equal(t, 2, imported)
	assert.Len(t, errs, 1)
}

func TestImportRowsContextCancellation(t *testing.T) {
	path := writeTempCSV(t, "id,name\n1,Alice\n2,Bob\n3,Charlie\n4,Dave\n")
	headers := []string{"id", "name"}
	columns := []ColumnHint{{Name: "id", IsPK: true}, {Name: "name"}}
	mapping, err := BuildColumnMapping(headers, columns)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	callCount := 0
	ImportRows(ctx, path, ImportOptions{HasHeader: true}, mapping,
		nil,
		func(row map[string]any) error {
			callCount++
			if callCount == 2 {
				cancel()
			}
			return nil
		},
	)

	assert.LessOrEqual(t, callCount, 3)
}

func TestImportRowsNullableEmptyFieldBecomesNil(t *testing.T) {
	path := writeTempCSV(t, "id,description\n1,\n")
	headers := []string{"id", "description"}
	columns := []ColumnHint{{Name: "id", IsPK: true}, {Name: "description", IsNullable: true}}
	mapping, err := BuildColumnMapping(headers, columns)
	require.NoError(t, err)

	var inserted []map[string]any
	ImportRows(context.Background(), path, ImportOptions{HasHeader: true}, mapping,
		nil,
		func(row map[string]any) error {
			inserted = append(inserted, row)
			return nil
		},
	)

	require.Len(t, inserted, 1)
	assert.Nil(t, inserted[0]["description"])
}
