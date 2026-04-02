package database

import "strings"

// TokenType identifies the category of a SQL token.
type TokenType int

const (
	TokenKeyword         TokenType = iota // SQL reserved word (case-insensitive)
	TokenIdentifier                       // unquoted identifier
	TokenQuotedIdentifier                 // "double-quoted" identifier
	TokenString                           // 'single-quoted string' with '' escape
	TokenNumber                           // numeric literal
	TokenComment                          // -- line comment or /* block comment */
	TokenOperator                         // =, !=, <>, <, >, <=, >=, +, -, /, %
	TokenPunctuation                      // ( ) , ;
	TokenWhitespace                       // spaces, tabs, newlines
	TokenDot                              // .
	TokenStar                             // *
	TokenTypecast                         // ::
)

// Token is a single lexical unit of a SQL string.
type Token struct {
	Type  TokenType
	Value string
	Start int // byte offset of first character (inclusive)
	End   int // byte offset past last character (exclusive)
}

// Tokenize splits sql into a flat slice of Tokens covering every byte.
// It never returns an error; unrecognised bytes are emitted as TokenPunctuation.
func Tokenize(sql string) []Token {
	tokens := make([]Token, 0, len(sql)/4)
	i := 0
	n := len(sql)

	for i < n {
		start := i
		ch := sql[i]

		// ── whitespace ───────────────────────────────────────────────────────
		if isSpace(ch) {
			for i < n && isSpace(sql[i]) {
				i++
			}
			tokens = append(tokens, Token{TokenWhitespace, sql[start:i], start, i})
			continue
		}

		// ── line comment  -- ─────────────────────────────────────────────────
		if ch == '-' && i+1 < n && sql[i+1] == '-' {
			for i < n && sql[i] != '\n' {
				i++
			}
			tokens = append(tokens, Token{TokenComment, sql[start:i], start, i})
			continue
		}

		// ── block comment  /* … */ ────────────────────────────────────────────
		if ch == '/' && i+1 < n && sql[i+1] == '*' {
			i += 2
			for i+1 < n && !(sql[i] == '*' && sql[i+1] == '/') {
				i++
			}
			if i+1 < n {
				i += 2 // consume */
			}
			tokens = append(tokens, Token{TokenComment, sql[start:i], start, i})
			continue
		}

		// ── typecast  :: ──────────────────────────────────────────────────────
		if ch == ':' && i+1 < n && sql[i+1] == ':' {
			i += 2
			tokens = append(tokens, Token{TokenTypecast, "::", start, i})
			continue
		}

		// ── single-quoted string  '…' ('' = escaped quote) ───────────────────
		if ch == '\'' {
			i++
			for i < n {
				if sql[i] == '\'' {
					i++
					if i < n && sql[i] == '\'' { // escaped single quote
						i++
						continue
					}
					break
				}
				i++
			}
			tokens = append(tokens, Token{TokenString, sql[start:i], start, i})
			continue
		}

		// ── double-quoted identifier  "…" ─────────────────────────────────────
		if ch == '"' {
			i++
			for i < n && sql[i] != '"' {
				i++
			}
			if i < n {
				i++ // consume closing "
			}
			tokens = append(tokens, Token{TokenQuotedIdentifier, sql[start:i], start, i})
			continue
		}

		// ── dot ───────────────────────────────────────────────────────────────
		if ch == '.' {
			i++
			tokens = append(tokens, Token{TokenDot, ".", start, i})
			continue
		}

		// ── star ──────────────────────────────────────────────────────────────
		if ch == '*' {
			i++
			tokens = append(tokens, Token{TokenStar, "*", start, i})
			continue
		}

		// ── two-character operators ───────────────────────────────────────────
		if i+1 < n {
			two := sql[i : i+2]
			switch two {
			case "!=", "<>", "<=", ">=":
				i += 2
				tokens = append(tokens, Token{TokenOperator, two, start, i})
				continue
			}
		}

		// ── single-character operators ────────────────────────────────────────
		if ch == '=' || ch == '<' || ch == '>' || ch == '+' || ch == '-' || ch == '/' || ch == '%' {
			i++
			tokens = append(tokens, Token{TokenOperator, string(ch), start, i})
			continue
		}

		// ── punctuation ───────────────────────────────────────────────────────
		if ch == '(' || ch == ')' || ch == ',' || ch == ';' {
			i++
			tokens = append(tokens, Token{TokenPunctuation, string(ch), start, i})
			continue
		}

		// ── numeric literal ───────────────────────────────────────────────────
		if ch >= '0' && ch <= '9' {
			for i < n && isNumContinue(sql[i]) {
				i++
			}
			tokens = append(tokens, Token{TokenNumber, sql[start:i], start, i})
			continue
		}

		// ── parameter placeholder  $1, $2, … ─────────────────────────────────
		if ch == '$' && i+1 < n && sql[i+1] >= '0' && sql[i+1] <= '9' {
			i++
			for i < n && sql[i] >= '0' && sql[i] <= '9' {
				i++
			}
			tokens = append(tokens, Token{TokenIdentifier, sql[start:i], start, i})
			continue
		}

		// ── keyword / identifier ──────────────────────────────────────────────
		if isIdentStart(ch) {
			for i < n && isIdentContinue(sql[i]) {
				i++
			}
			word := sql[start:i]
			typ := TokenIdentifier
			if sqlKeywordSet[strings.ToUpper(word)] {
				typ = TokenKeyword
			}
			tokens = append(tokens, Token{typ, word, start, i})
			continue
		}

		// ── fallback: unknown byte ────────────────────────────────────────────
		i++
		tokens = append(tokens, Token{TokenPunctuation, string(ch), start, i})
	}

	return tokens
}

// ─── character helpers ────────────────────────────────────────────────────────

func isSpace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentContinue(ch byte) bool {
	return isIdentStart(ch) || (ch >= '0' && ch <= '9') || ch == '$'
}

func isNumContinue(ch byte) bool {
	return (ch >= '0' && ch <= '9') || ch == '.' || ch == '_' ||
		ch == 'e' || ch == 'E' || ch == '+' || ch == '-'
}

// sqlKeywordSet is the full set of SQL reserved words recognised by the lexer.
// Keys are upper-case; lookup is always done with strings.ToUpper.
var sqlKeywordSet = func() map[string]bool {
	words := []string{
		// DML
		"SELECT", "INSERT", "UPDATE", "DELETE", "MERGE", "WITH",
		// DDL
		"CREATE", "DROP", "ALTER", "TRUNCATE", "RENAME",
		// Clauses
		"FROM", "WHERE", "HAVING", "GROUP", "ORDER", "BY",
		"JOIN", "LEFT", "RIGHT", "INNER", "OUTER", "FULL", "CROSS", "NATURAL",
		"ON", "USING", "INTO", "VALUES", "SET", "RETURNING",
		"LIMIT", "OFFSET", "FETCH", "NEXT", "ROWS", "ONLY", "FOR",
		"UNION", "INTERSECT", "EXCEPT", "ALL", "DISTINCT",
		// Boolean logic
		"AND", "OR", "NOT",
		// Predicates
		"IN", "BETWEEN", "LIKE", "ILIKE", "SIMILAR", "IS",
		"EXISTS", "ANY", "SOME", "UNIQUE",
		// Values
		"NULL", "TRUE", "FALSE", "DEFAULT",
		// Table/column keywords
		"AS", "CASE", "WHEN", "THEN", "ELSE", "END",
		"CONSTRAINT", "PRIMARY", "KEY", "FOREIGN", "REFERENCES", "CHECK",
		"TABLE", "VIEW", "INDEX", "SCHEMA", "DATABASE", "SEQUENCE",
		"FUNCTION", "PROCEDURE", "TRIGGER", "COLUMN", "ROW", "TYPE",
		// Order
		"ASC", "DESC", "NULLS", "FIRST", "LAST",
		// Functions (common)
		"CAST", "COALESCE", "NULLIF", "COUNT", "SUM", "AVG", "MIN", "MAX",
		"NOW", "CURRENT_TIMESTAMP", "CURRENT_DATE",
		"LOWER", "UPPER", "TRIM", "LENGTH",
		// Misc
		"DO", "BEGIN", "COMMIT", "ROLLBACK", "SAVEPOINT",
		"GRANT", "REVOKE", "EXPLAIN", "ANALYZE", "VACUUM",
	}
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}()
