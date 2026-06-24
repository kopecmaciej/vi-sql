package sql

import "strings"

// ContextType describes what the SQL editor expects at the cursor position.
type ContextType int

const (
	CtxStatementStart ContextType = iota // beginning of a statement or after DML keyword
	CtxAfterSelect                       // column / expression list
	CtxAfterFrom                         // table / schema reference
	CtxAfterJoin                         // table after a JOIN keyword
	CtxAfterWhere                        // predicate expression
	CtxAfterHaving                       // HAVING clause (aggregate predicates)
	CtxAfterOn                           // JOIN … ON condition
	CtxAfterOrderBy                      // column reference in ORDER BY
	CtxAfterGroupBy                      // column reference in GROUP BY
	CtxAfterSet                          // column = value in UPDATE SET
	CtxAfterInto                         // table name in INSERT INTO
	CtxAfterValues                       // value list after VALUES
	CtxAfterDot                          // schema.‹table› or table.‹column›
	CtxAfterDDLVerb                      // after CREATE / DROP / ALTER / TRUNCATE / RENAME
	CtxCreateObject                      // after CREATE TABLE/VIEW/… (new name, suppress existing-table list)
	CtxExistingObject                    // after DROP/ALTER/TRUNCATE TABLE/VIEW/… (pick existing name)
	CtxGeneral                           // fallback — no useful clause found
)

// CtxAfterDDLObject is an alias for CtxExistingObject kept for compatibility.
const CtxAfterDDLObject = CtxExistingObject

// CompletionContext carries everything the autocomplete engine needs to rank
// and filter candidates at a given cursor position.
type CompletionContext struct {
	Type ContextType

	// TableName holds the qualifier before a dot (CtxAfterDot):
	//   "users"  in  users.‹cursor›
	//   "public" in  public.‹cursor›
	TableName string

	// PartialWord is the text already typed at the cursor (may be empty).
	PartialWord string

	// PrecedingTokenType is the type of the last non-whitespace token before
	// the cursor (or the partial word being typed). Used to distinguish
	// "SELECT col |" (TokenIdentifier → next is a clause keyword) from
	// "SELECT col, |" (TokenPunctuation → next is another column).
	PrecedingTokenType TokenType

	// PrecedingTokenValue is the raw text of that token. Needed because type
	// alone can't distinguish AS vs SELECT (both keyword) or "," vs "(" vs ")"
	// (all punctuation) — these matter for expression-position-aware filtering.
	PrecedingTokenValue string
}

// SuppressRaw reports whether autocompletion should be suppressed at cursorPos
// using only the raw text, without tokenizing. It covers:
//   - partial word starting with a digit (inside a number literal, e.g. "123|", "col = 1|")
//   - char immediately before the partial is a digit or operator (e.g. "1a|", ">=x|")
//   - char immediately before the partial is a quote (e.g. "'|", "'hello'x|")
//   - cursor inside a string literal detected via odd single-quote count (e.g. "'hello world|")
func SuppressRaw(text string, cursorPos int) bool {
	if cursorPos == 0 {
		return false
	}
	pos := cursorPos
	for pos > 0 && isIdentContinue(text[pos-1]) {
		pos--
	}
	// Partial word starts with a digit → number literal.
	if pos < cursorPos && text[pos] >= '0' && text[pos] <= '9' {
		return true
	}
	if pos == 0 {
		return false
	}
	ch := text[pos-1]
	if (ch >= '0' && ch <= '9') ||
		ch == '=' || ch == '<' || ch == '>' || ch == '!' ||
		ch == '+' || ch == '-' || ch == '/' || ch == '%' ||
		ch == '\'' {
		return true
	}
	// Odd single-quote count → inside a string literal (handles spaces inside strings).
	quotes := 0
	for i := 0; i < pos; i++ {
		if text[i] == '\'' {
			quotes++
		}
	}
	return quotes%2 == 1
}

// DetectContext returns the completion context for cursorBytePos inside sql.
//
// It tokenizes the input, locates the cursor, extracts any partial word, and
// then walks backwards through the token list to find the nearest clause
// keyword that determines what kind of identifiers are expected.
func DetectContext(tokens []Token, cursorBytePos int) CompletionContext {
	if len(tokens) == 0 {
		return CompletionContext{Type: CtxStatementStart}
	}

	partial, lookFrom := extractPartialAndLookFrom(tokens, cursorBytePos)

	// Skip trailing whitespace backwards.
	for lookFrom >= 0 && tokens[lookFrom].Type == TokenWhitespace {
		lookFrom--
	}

	if lookFrom < 0 {
		return CompletionContext{Type: CtxStatementStart, PartialWord: partial}
	}

	// Capture the type and value of the last non-whitespace token before the cursor.
	precedingType := tokens[lookFrom].Type
	precedingValue := tokens[lookFrom].Value

	// SELECT * | vs SELECT a * | — resolve whether * is a wildcard or operator.
	// A * preceded by an identifier/number/string/")": multiplication (not a value).
	// A * preceded by anything else (keyword, comma, "("): SELECT wildcard — treat as identifier.
	if precedingType == TokenStar {
		precedingType, precedingValue = resolveStarPrecedingType(tokens, lookFrom)
	}

	// ── dot-qualified context ─────────────────────────────────────────────────
	// e.g.  users.‹cursor›  or  public.users.‹cursor›
	if tokens[lookFrom].Type == TokenDot {
		qualifier := qualifierBeforeDot(tokens, lookFrom)
		return CompletionContext{
			Type:                CtxAfterDot,
			TableName:           qualifier,
			PartialWord:         partial,
			PrecedingTokenType:  precedingType,
			PrecedingTokenValue: precedingValue,
		}
	}

	// ── walk backwards for nearest clause keyword ─────────────────────────────
	return walkForContext(tokens, lookFrom, partial, precedingType, precedingValue)
}

// extractPartialAndLookFrom returns the partial word being typed at the cursor
// and the index of the last token that is fully before (or at) the cursor.
//
//	"SELECT * FROM ta|ble"  →  partial="ta", lookFrom = index of token before "ta"
//	"SELECT * FROM |"       →  partial="",   lookFrom = index of the space token
func extractPartialAndLookFrom(tokens []Token, cursorBytePos int) (partial string, lookFrom int) {
	lookFrom = -1

	for i, tok := range tokens {
		if tok.Start >= cursorBytePos {
			// Token starts at or after cursor — stop.
			lookFrom = i - 1
			return
		}
		if tok.End >= cursorBytePos && tok.Start < cursorBytePos {
			// Cursor is inside or at the end of this token.
			switch tok.Type {
			case TokenIdentifier, TokenKeyword:
				// Word token: cursor is still editing it — treat as partial.
				partial = tok.Value[:cursorBytePos-tok.Start]
				lookFrom = i - 1
			case TokenQuotedIdentifier:
				// Cursor editing inside a quoted identifier (e.g. "tab): treat the
				// text after the opening quote as the partial word so completion
				// still filters and the preceding dot is seen for qualification.
				partial = strings.TrimPrefix(tok.Value[:cursorBytePos-tok.Start], `"`)
				lookFrom = i - 1
			default:
				// Non-word token (dot, operator, punctuation, whitespace):
				// cursor is past the token boundary — include it in lookFrom.
				lookFrom = i
			}
			return
		}
		// Token is fully before cursor.
		lookFrom = i
	}
	// Cursor is past all tokens.
	return
}

// unquoteIdent strips the surrounding double quotes from a "quoted" identifier
// and collapses the "" escape, yielding the raw name as stored in the schema
// index. A trailing quote may be absent if the identifier is still being typed.
func unquoteIdent(s string) string {
	s = strings.TrimPrefix(s, `"`)
	s = strings.TrimSuffix(s, `"`)
	return strings.ReplaceAll(s, `""`, `"`)
}

// qualifierBeforeDot returns the identifier immediately before the dot at dotIdx.
func qualifierBeforeDot(tokens []Token, dotIdx int) string {
	i := dotIdx - 1
	for i >= 0 && tokens[i].Type == TokenWhitespace {
		i--
	}
	if i < 0 {
		return ""
	}
	t := tokens[i]
	if t.Type == TokenIdentifier {
		return t.Value
	}
	if t.Type == TokenQuotedIdentifier {
		return unquoteIdent(t.Value)
	}
	return ""
}

// walkForContext scans backwards from startIdx to find the nearest SQL clause
// keyword and returns the corresponding CompletionContext.
//
// Parenthesis depth is tracked: keywords inside balanced paren groups are
// skipped so they don't leak context outward. When an unmatched "(" is
// encountered (depth < 0) the cursor is inside a paren group; contextInsideParen
// is called to decide the appropriate context.
func walkForContext(tokens []Token, startIdx int, partial string, precedingType TokenType, precedingValue string) CompletionContext {
	depth := 0
	for i := startIdx; i >= 0; i-- {
		tok := tokens[i]

		if tok.Type == TokenPunctuation {
			switch tok.Value {
			case ")":
				depth++
			case "(":
				depth--
				if depth < 0 {
					return contextInsideParen(tokens, i, partial, precedingType, precedingValue)
				}
			}
			continue
		}

		// Skip non-keywords and any keyword buried inside balanced parens.
		if tok.Type != TokenKeyword || depth > 0 {
			continue
		}

		switch strings.ToUpper(tok.Value) {
		case "SELECT":
			return CompletionContext{Type: CtxAfterSelect, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}

		case "FROM":
			return CompletionContext{Type: CtxAfterFrom, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}

		case "JOIN":
			return CompletionContext{Type: CtxAfterJoin, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}

		case "ON":
			return CompletionContext{Type: CtxAfterOn, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}

		case "WHERE":
			return CompletionContext{Type: CtxAfterWhere, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}

		case "HAVING":
			return CompletionContext{Type: CtxAfterHaving, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}

		case "BY":
			// Distinguish ORDER BY vs GROUP BY by looking at the preceding keyword.
			for j := i - 1; j >= 0; j-- {
				if tokens[j].Type == TokenWhitespace {
					continue
				}
				if tokens[j].Type == TokenKeyword {
					switch strings.ToUpper(tokens[j].Value) {
					case "ORDER":
						return CompletionContext{Type: CtxAfterOrderBy, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}
					case "GROUP":
						return CompletionContext{Type: CtxAfterGroupBy, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}
					}
				}
				break
			}

		case "SET":
			return CompletionContext{Type: CtxAfterSet, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}

		case "INTO":
			return CompletionContext{Type: CtxAfterInto, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}

		case "VALUES":
			return CompletionContext{Type: CtxAfterValues, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}

		case "LIMIT", "OFFSET":
			return CompletionContext{Type: CtxGeneral, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}

		case "RETURNING":
			return CompletionContext{Type: CtxAfterSelect, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}

		case "UNION", "INTERSECT", "EXCEPT":
			return CompletionContext{Type: CtxStatementStart, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}

		case "INSERT", "UPDATE", "DELETE", "WITH":
			return CompletionContext{Type: CtxStatementStart, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}

		case "TABLE", "VIEW", "INDEX", "SCHEMA", "DATABASE",
			"SEQUENCE", "FUNCTION", "PROCEDURE", "TRIGGER", "TYPE":
			// Look back for the DDL verb to distinguish CREATE from DROP/ALTER/TRUNCATE/RENAME.
			for j := i - 1; j >= 0; j-- {
				if tokens[j].Type == TokenWhitespace {
					continue
				}
				if tokens[j].Type == TokenKeyword {
					switch strings.ToUpper(tokens[j].Value) {
					case "CREATE":
						return CompletionContext{Type: CtxCreateObject, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}
					case "DROP", "ALTER", "TRUNCATE", "RENAME":
						return CompletionContext{Type: CtxExistingObject, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}
					}
				}
				break
			}
			return CompletionContext{Type: CtxExistingObject, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}

		case "CREATE", "DROP", "ALTER", "TRUNCATE", "RENAME":
			return CompletionContext{Type: CtxAfterDDLVerb, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}
		}
	}

	return CompletionContext{Type: CtxStatementStart, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}
}

// resolveStarPrecedingType returns the effective TokenType/Value for a `*` token
// at starIdx. If `*` is preceded by a value (identifier, literal, ")") it is a
// multiplication operator and stays TokenStar. Otherwise it is a SELECT wildcard
// and is treated as TokenIdentifier so isAtValuePosition fires correctly.
func resolveStarPrecedingType(tokens []Token, starIdx int) (TokenType, string) {
	i := starIdx - 1
	for i >= 0 && tokens[i].Type == TokenWhitespace {
		i--
	}
	if i < 0 {
		return TokenIdentifier, "*"
	}
	switch tokens[i].Type {
	case TokenIdentifier, TokenQuotedIdentifier, TokenNumber, TokenString:
		return TokenStar, "*" // multiplication
	case TokenPunctuation:
		if tokens[i].Value == ")" {
			return TokenStar, "*" // (expr) * …
		}
	}
	return TokenIdentifier, "*" // wildcard
}

// contextInsideParen decides the completion context when the cursor is inside
// a paren group whose opening "(" is at parenIdx. It looks at the token
// immediately before the "(" to distinguish the different cases:
//
//   - identifier before "("  → column definition or function args (CtxGeneral)
//   - IN / ANY / ALL / SOME / EXISTS before "("  → subquery or value list (CtxStatementStart)
//   - VALUES before "("  → value list (CtxAfterValues)
//   - anything else  → CtxGeneral (no schema/table suggestions)
func contextInsideParen(tokens []Token, parenIdx int, partial string, precedingType TokenType, precedingValue string) CompletionContext {
	i := parenIdx - 1
	for i >= 0 && tokens[i].Type == TokenWhitespace {
		i--
	}
	if i >= 0 && tokens[i].Type == TokenKeyword {
		switch strings.ToUpper(tokens[i].Value) {
		case "IN", "ANY", "ALL", "SOME", "EXISTS":
			return CompletionContext{Type: CtxStatementStart, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}
		case "VALUES":
			return CompletionContext{Type: CtxAfterValues, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}
		}
	}
	return CompletionContext{Type: CtxGeneral, PartialWord: partial, PrecedingTokenType: precedingType, PrecedingTokenValue: precedingValue}
}
