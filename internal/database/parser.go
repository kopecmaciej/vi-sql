package database

import (
	"fmt"
	"strconv"
	"strings"
)

// extractClauseInt finds " KEYWORD N" in sql (case-insensitive) and returns N.
// N must be > 0 to be considered present.
func extractClauseInt(sql, keyword string) (int64, bool) {
	upper := strings.ToUpper(sql)
	marker := " " + strings.ToUpper(keyword) + " "
	idx := strings.Index(upper, marker)
	if idx < 0 {
		return 0, false
	}
	rest := strings.TrimSpace(sql[idx+len(marker):])
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, false
	}
	n, err := strconv.ParseInt(rest[:end], 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// ExtractLimitValue returns the LIMIT N integer from a SQL string, if present.
func ExtractLimitValue(sql string) (int64, bool) {
	return extractClauseInt(sql, "LIMIT")
}

// RebuildSelectSQL replaces the WHERE and ORDER BY clauses in a SELECT,
// then re-appends any LIMIT / OFFSET from the original query.
// Pass empty strings to omit a clause.
func RebuildSelectSQL(sql, newWhere, newOrderBy string) string {
	upper := strings.ToUpper(sql)

	cutPos := len(sql)
	for _, kw := range []string{" WHERE ", " ORDER BY ", " LIMIT "} {
		if idx := strings.Index(upper, kw); idx >= 0 && idx < cutPos {
			cutPos = idx
		}
	}

	result := strings.TrimSpace(sql[:cutPos])
	if newWhere != "" {
		result += " WHERE " + newWhere
	}
	if newOrderBy != "" {
		result += " ORDER BY " + newOrderBy
	}
	if limit, ok := extractClauseInt(sql, "LIMIT"); ok {
		result += fmt.Sprintf(" LIMIT %d", limit)
		if offset, ok := extractClauseInt(sql, "OFFSET"); ok {
			result += fmt.Sprintf(" OFFSET %d", offset)
		}
	}
	return result
}

// SanitizeWhereClause performs basic sanity checks on user-provided WHERE input.
// It rejects obvious DDL/DML injection attempts in a filter context.
func SanitizeWhereClause(where string) error {
	if where == "" {
		return nil
	}

	upper := strings.ToUpper(strings.TrimSpace(where))
	forbidden := []string{
		"DROP ", "DELETE ", "INSERT ", "UPDATE ", "ALTER ",
		"CREATE ", "TRUNCATE ", "GRANT ", "REVOKE ",
		"EXEC ", "EXECUTE ",
	}
	for _, f := range forbidden {
		if strings.Contains(upper, f) {
			return fmt.Errorf("WHERE clause must not contain %s statements", strings.TrimSpace(f))
		}
	}
	return nil
}
