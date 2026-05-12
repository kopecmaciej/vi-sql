package database

import (
	"fmt"
	"strings"
)

// QuoteIdent quotes a SQL identifier with q, doubling any embedded quote
// characters to prevent injection. Use this for drivers with simple quoting
// rules: ` (MySQL) or " (SQLite). Postgres should keep pgx.Identifier.Sanitize
// because it understands the wire format.
func QuoteIdent(name string, q byte) string {
	qs := string(q)
	return qs + strings.ReplaceAll(name, qs, qs+qs) + qs
}

// QuoteQualified returns `schema`.`table` (or "schema"."table") using q.
func QuoteQualified(schema, table string, q byte) string {
	return QuoteIdent(schema, q) + "." + QuoteIdent(table, q)
}

// BuildPKWhere returns `col = ?` clauses and matching args for every column
// in pk. Use this for drivers with positional ? placeholders (MySQL, SQLite);
// Postgres builds its own with $N indices.
func BuildPKWhere(pk PrimaryKey, quote func(string) string) ([]string, []any) {
	parts := make([]string, 0, len(pk.Columns))
	args := make([]any, 0, len(pk.Columns))
	for col, val := range pk.Columns {
		parts = append(parts, fmt.Sprintf("%s = ?", quote(col)))
		args = append(args, val)
	}
	return parts, args
}
