package mysql

import (
	"fmt"
	"strings"

	"github.com/kopecmaciej/vi-sql/internal/database"
)

// Formatter provides MySQL-specific value formatting for the TUI layer.
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

// quoteMySQLIdent backtick-quotes a MySQL identifier to prevent injection.
func quoteMySQLIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// quoteMySQLTable returns a fully-qualified `schema`.`table` identifier.
func quoteMySQLTable(schema, table string) string {
	return quoteMySQLIdent(schema) + "." + quoteMySQLIdent(table)
}

// buildPKWhere creates WHERE clause parts and args from a PrimaryKey using ? placeholders.
func buildPKWhere(pk database.PrimaryKey) ([]string, []any) {
	parts := make([]string, 0, len(pk.Columns))
	args := make([]any, 0, len(pk.Columns))
	for col, val := range pk.Columns {
		parts = append(parts, fmt.Sprintf("%s = ?", quoteMySQLIdent(col)))
		args = append(args, val)
	}
	return parts, args
}
