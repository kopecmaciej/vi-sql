package postgres

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/kopecmaciej/vi-sql/internal/database"
)

// buildPKWhere creates WHERE clause parts and args from a PrimaryKey.
func buildPKWhere(pk database.PrimaryKey) ([]string, []any) {
	parts := make([]string, 0, len(pk.Columns))
	args := make([]any, 0, len(pk.Columns))
	i := 1
	for col, val := range pk.Columns {
		parts = append(parts, fmt.Sprintf("%s = $%d", pgx.Identifier{col}.Sanitize(), i))
		args = append(args, val)
		i++
	}
	return parts, args
}

// Formatter provides PostgreSQL-specific value formatting for the TUI layer.
// All database values arrive as strings (PostgreSQL text wire format), so
// formatting is straightforward quoting.
type Formatter struct{}

// SQLLiteral formats a value as a SQL literal for use in a generated UPDATE template.
func (f *Formatter) SQLLiteral(val any) string {
	if val == nil {
		return "NULL"
	}
	s := database.StringifyValue(val)
	if s == "NULL" {
		return "NULL"
	}
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// EditableString returns the value formatted for display in the inline edit input.
func (f *Formatter) EditableString(val any) string {
	if val == nil {
		return ""
	}
	return database.StringifyValue(val)
}
