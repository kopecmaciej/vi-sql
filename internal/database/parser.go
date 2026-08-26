package database

import (
	"fmt"
	"strconv"
	"strings"
)

// ExtractLimitValue returns the row-count integer of the LIMIT clause. Both
// dialect forms are handled: "LIMIT n" and MySQL-style "LIMIT offset, n" —
// in the comma form the second number is the count, not the first.
func ExtractLimitValue(sql string) (int64, bool) {
	limit, _, ok := extractLimitClause(sql)
	return limit, ok
}

// extractLimitClause parses a LIMIT clause into (count, offset). It supports
// "LIMIT n [OFFSET m]" as well as "LIMIT m, n". ok is false when no valid
// clause is present.
func extractLimitClause(sql string) (count, offset int64, ok bool) {
	upper := strings.ToUpper(sql)
	idx := strings.Index(upper, " LIMIT ")
	if idx < 0 {
		return 0, 0, false
	}
	rest := strings.TrimSpace(sql[idx+len(" LIMIT "):])
	// The clause ends at a statement terminator.
	if end := strings.IndexAny(rest, ";)"); end >= 0 {
		rest = strings.TrimSpace(rest[:end])
	}
	first, restAfterFirst := splitLeadingInt(rest)
	if !first.ok {
		return 0, 0, false
	}
	restAfterFirst = strings.TrimSpace(restAfterFirst)
	if strings.HasPrefix(restAfterFirst, ",") {
		// MySQL form: LIMIT offset, count
		second, _ := splitLeadingInt(strings.TrimSpace(restAfterFirst[1:]))
		return second.n, first.n, second.ok
	}
	if len(restAfterFirst) >= 7 && strings.EqualFold(restAfterFirst[:7], "OFFSET ") {
		off, _ := splitLeadingInt(strings.TrimSpace(restAfterFirst[7:]))
		if off.ok {
			return first.n, off.n, true
		}
	}
	return first.n, 0, true
}

// splitLeadingInt reads an optional sign-free integer at the start of s.
type intPart struct {
	n  int64
	ok bool
}

func splitLeadingInt(s string) (intPart, string) {
	s = strings.TrimSpace(s)
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 {
		return intPart{}, s
	}
	n, err := strconv.ParseInt(s[:end], 10, 64)
	if err != nil || n <= 0 {
		return intPart{}, s[end:]
	}
	return intPart{n: n, ok: true}, s[end:]
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
	if limit, offset, ok := extractLimitClause(sql); ok {
		result += fmt.Sprintf(" LIMIT %d", limit)
		if offset > 0 {
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
