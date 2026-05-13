package util

import (
	"fmt"
	"strings"
)

// Quoter applies a SQL dialect's identifier-quoting rules. Use the
// predefined ANSIQuoter (Postgres, SQLite), BacktickQuoter (MySQL), or
// BracketQuoter (SQL Server).
type Quoter byte

const (
	ANSIQuoter     Quoter = '"' // PostgreSQL, SQLite
	BacktickQuoter Quoter = '`' // MySQL, MariaDB
	BracketQuoter  Quoter = '[' // SQL Server
)

// Ident wraps name in the dialect's quote character, doubling any embedded
// quote characters so reserved words and special characters in identifiers
// are safe to interpolate.
func (q Quoter) Ident(name string) string {
	if q == BracketQuoter {
		// SQL Server bracket quoting: [name], with ] escaped as ]]
		return "[" + strings.ReplaceAll(name, "]", "]]") + "]"
	}
	s := string(q)
	return s + strings.ReplaceAll(name, s, s+s) + s
}

// Table returns a schema-qualified table reference with both parts quoted.
func (q Quoter) Table(schema, table string) string {
	return q.Ident(schema) + "." + q.Ident(table)
}

// WhereEqAnon builds `<col> = ?` clauses for each entry in cols, returning
// the clauses alongside matching args. Use for drivers with anonymous
// placeholders bound by argument order (MySQL, SQLite).
func (q Quoter) WhereEqAnon(cols map[string]any) ([]string, []any) {
	parts := make([]string, 0, len(cols))
	args := make([]any, 0, len(cols))
	for col, val := range cols {
		parts = append(parts, q.Ident(col)+" = ?")
		args = append(args, val)
	}
	return parts, args
}

// WhereEqMSSql builds `<col> = @pN` WHERE clauses for SQL Server, numbering
// parameters starting at startIdx. Returns clauses, matching args, and the
// next available parameter index (startIdx + len(cols)).
func (q Quoter) WhereEqMSSql(cols map[string]any, startIdx int) (parts []string, args []any, nextIdx int) {
	parts = make([]string, 0, len(cols))
	args = make([]any, 0, len(cols))
	i := startIdx
	for col, val := range cols {
		parts = append(parts, fmt.Sprintf("%s = @p%d", q.Ident(col), i))
		args = append(args, val)
		i++
	}
	return parts, args, i
}

// WhereEqIndexed builds `<col> = $N` clauses for each entry in cols,
// returning the clauses alongside matching args. Use for drivers with
// 1-based indexed placeholders (Postgres).
func (q Quoter) WhereEqIndexed(cols map[string]any) ([]string, []any) {
	parts := make([]string, 0, len(cols))
	args := make([]any, 0, len(cols))
	i := 1
	for col, val := range cols {
		parts = append(parts, fmt.Sprintf("%s = $%d", q.Ident(col), i))
		args = append(args, val)
		i++
	}
	return parts, args
}
