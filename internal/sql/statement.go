package sql

import "strings"

// DestructiveStatementInfo is returned by HasDestructiveStatement when sql
// contains a DELETE, UPDATE, TRUNCATE, or DROP operation.
type DestructiveStatementInfo struct {
	Operation string
	Table     string
	HasWhere  bool
}

// HasDestructiveStatement returns info when sql is a DELETE / UPDATE / TRUNCATE /
// DROP statement, also reports whether a WHERE is present.
func HasDestructiveStatement(sql string) *DestructiveStatementInfo {
	tokens := Tokenize(sql)

	sig := make([]Token, 0, len(tokens))
	for _, t := range tokens {
		if t.Type != TokenWhitespace && t.Type != TokenComment {
			sig = append(sig, t)
		}
	}
	if len(sig) == 0 {
		return nil
	}

	first := strings.ToUpper(sig[0].Value)
	switch first {
	case "DELETE":
		info := &DestructiveStatementInfo{Operation: "DELETE"}
		idx := 1
		if idx < len(sig) && strings.ToUpper(sig[idx].Value) == "FROM" {
			idx++
		}
		info.Table = extractTableName(sig, idx)
		info.HasWhere = topLevelWhere(sig)
		return info

	case "UPDATE":
		info := &DestructiveStatementInfo{Operation: "UPDATE"}
		info.Table = extractTableName(sig, 1)
		info.HasWhere = topLevelWhere(sig)
		return info

	case "TRUNCATE":
		info := &DestructiveStatementInfo{Operation: "TRUNCATE"}
		idx := 1
		if idx < len(sig) && strings.ToUpper(sig[idx].Value) == "TABLE" {
			idx++
		}
		info.Table = extractTableName(sig, idx)
		return info

	case "DROP":
		info := &DestructiveStatementInfo{Operation: "DROP"}
		if len(sig) > 1 {
			switch strings.ToUpper(sig[1].Value) {
			case "TABLE", "VIEW", "INDEX", "SCHEMA", "DATABASE", "TRIGGER", "FUNCTION", "PROCEDURE", "SEQUENCE":
				idx := 2
				if idx+1 < len(sig) && strings.ToUpper(sig[idx].Value) == "IF" && strings.ToUpper(sig[idx+1].Value) == "EXISTS" {
					idx += 2
				}
				info.Table = extractTableName(sig, idx)
			}
		}
		return info
	}

	return nil
}

// IsExplainQuery reports whether sql starts with EXPLAIN.
func IsExplainQuery(sql string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "EXPLAIN")
}

// IsResultCommand reports whether sql is a SHOW/DESC-style command that
// returns a result set directly. Such commands cannot be wrapped into a
// paginated subquery (SELECT * FROM (cmd) t), so they must run unwrapped.
func IsResultCommand(sql string) bool {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(upper, "SHOW") ||
		strings.HasPrefix(upper, "DESC") // covers DESC and DESCRIBE
}

// IsReturningDML checks for INSERT/UPDATE/DELETE with a RETURNING clause.
func IsReturningDML(sql string) bool {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	hasDML := strings.HasPrefix(upper, "INSERT") ||
		strings.HasPrefix(upper, "UPDATE") ||
		strings.HasPrefix(upper, "DELETE")
	return hasDML && strings.Contains(upper, "RETURNING")
}

// extractTableName returns the table name at sig[idx], including schema prefix if present.
func extractTableName(sig []Token, idx int) string {
	if idx >= len(sig) {
		return ""
	}
	t := sig[idx]
	if t.Type != TokenIdentifier && t.Type != TokenQuotedIdentifier {
		return ""
	}
	name := t.Value
	if idx+2 < len(sig) && sig[idx+1].Type == TokenDot {
		name = name + "." + sig[idx+2].Value
	}
	return name
}

// topLevelWhere reports whether sig contains a WHERE keyword at the top level.
func topLevelWhere(sig []Token) bool {
	depth := 0
	for _, t := range sig {
		switch t.Value {
		case "(":
			depth++
		case ")":
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 && t.Type == TokenKeyword && strings.ToUpper(t.Value) == "WHERE" {
			return true
		}
	}
	return false
}
