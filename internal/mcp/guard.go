package mcp

import (
	"fmt"
	"strings"
)

// allowedFirstKeywords are the only SQL verbs that start a read-only statement.
var allowedFirstKeywords = map[string]bool{
	"SELECT":  true,
	"WITH":    true, // CTEs — further checked below
	"EXPLAIN": true,
	"SHOW":    true,
	"TABLE":   true,  // PostgreSQL shorthand for SELECT * FROM table
	"VALUES":  true,
}

// writeKeywords are SQL verbs that mutate data or schema.
var writeKeywords = []string{
	"INSERT", "UPDATE", "DELETE", "DROP", "ALTER",
	"CREATE", "TRUNCATE", "GRANT", "REVOKE",
	"COPY", "MERGE", "CALL", "EXECUTE",
}

// assertReadOnly returns an error if query contains any mutation statement.
// It strips comments and string literals before analysis, and tracks
// parenthesis depth so write keywords inside CTE subqueries are caught too.
func assertReadOnly(query string) error {
	clean := stripSQL(query)
	upper := strings.ToUpper(clean)

	first := firstKeyword(upper)
	if first == "" {
		return fmt.Errorf("empty query")
	}
	if !allowedFirstKeywords[first] {
		return fmt.Errorf("read-only mode: %s statements are not allowed", first)
	}

	// Walk tokens at depth 0 looking for write keywords.
	// Depth > 0 means we're inside a subquery or CTE body — still check those.
	for _, kw := range writeKeywords {
		if containsKeyword(upper, kw) {
			return fmt.Errorf("read-only mode: query contains a %s statement", kw)
		}
	}

	return nil
}

// stripSQL removes line comments, block comments, and string literals so that
// keywords inside them don't trigger false positives.
func stripSQL(q string) string {
	var b strings.Builder
	b.Grow(len(q))
	i := 0
	for i < len(q) {
		// Line comment
		if i+1 < len(q) && q[i] == '-' && q[i+1] == '-' {
			for i < len(q) && q[i] != '\n' {
				i++
			}
			b.WriteByte(' ')
			continue
		}
		// Block comment
		if i+1 < len(q) && q[i] == '/' && q[i+1] == '*' {
			i += 2
			for i+1 < len(q) && !(q[i] == '*' && q[i+1] == '/') {
				i++
			}
			i += 2
			b.WriteByte(' ')
			continue
		}
		// Single-quoted string literal (handles '' escape)
		if q[i] == '\'' {
			i++
			for i < len(q) {
				if q[i] == '\'' {
					i++
					if i < len(q) && q[i] == '\'' {
						i++ // escaped quote
						continue
					}
					break
				}
				i++
			}
			b.WriteString("''")
			continue
		}
		// Dollar-quoted string ($$...$$  or $tag$...$tag$)
		if q[i] == '$' {
			end := strings.Index(q[i+1:], "$")
			if end >= 0 {
				tag := q[i : i+end+2] // e.g. "$$" or "$body$"
				closeAt := strings.Index(q[i+len(tag):], tag)
				if closeAt >= 0 {
					i += len(tag) + closeAt + len(tag)
					b.WriteString("''")
					continue
				}
			}
		}
		b.WriteByte(q[i])
		i++
	}
	return b.String()
}

// firstKeyword returns the first whitespace-delimited uppercase token.
func firstKeyword(upper string) string {
	upper = strings.TrimSpace(upper)
	end := strings.IndexAny(upper, " \t\n\r(")
	if end < 0 {
		return upper
	}
	return upper[:end]
}

// containsKeyword reports whether kw appears as a whole word in upper.
func containsKeyword(upper, kw string) bool {
	idx := 0
	for {
		pos := strings.Index(upper[idx:], kw)
		if pos < 0 {
			return false
		}
		abs := idx + pos
		before := abs == 0 || !isIdentChar(upper[abs-1])
		after := abs+len(kw) >= len(upper) || !isIdentChar(upper[abs+len(kw)])
		if before && after {
			return true
		}
		idx = abs + 1
	}
}

func isIdentChar(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') || c == '_'
}
