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

	// Qualifier holds the identifier before a dot (CtxAfterDot). It may resolve
	// to a table/alias or a schema
	Qualifier string

	// PartialWord is the text already typed at the cursor (may be empty).
	PartialWord string

	// QuotedPartial is true when the partial is being typed inside an opening
	// identifier quote ("col, `col, [col). The engine extends the replace range
	// over that opening quote so completion emits a balanced quoted identifier.
	QuotedPartial bool

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

// SuppressRaw is a cheap pre-check whether autocompletion should be suppressed
// at cursorPos so we can avoid tokenizing. It covers cursor inside strings, numbers,
// and right after an operator or quote character.
func SuppressRaw(text string, cursorPos int) bool {
	if cursorPos == 0 {
		return false
	}
	pos := cursorPos
	for pos > 0 && isIdentContinue(text[pos-1]) {
		pos--
	}
	if pos < cursorPos && text[pos] >= '0' && text[pos] <= '9' {
		// Pure numeric partial (e.g. "123") → always suppress.
		pureNum := true
		for k := pos; k < cursorPos; k++ {
			if !isPartOfNumber(text[k]) {
				pureNum = false
				break
			}
		}
		if pureNum {
			return true
		}
		// Mixed digit-start partial (e.g. "2fa"): only suppress when the
		// nearest meaningful preceding char is a value-context marker.
		if pos > 0 {
			chIdx := pos - 1
			for chIdx > 0 && isSpace(text[chIdx]) {
				chIdx--
			}
			ch := text[chIdx]
			if (ch >= '0' && ch <= '9') || ch == '=' || ch == '<' || ch == '>' || ch == '!' ||
				ch == '+' || ch == '-' || ch == '/' || ch == '%' || ch == '\'' {
				return true
			}
		}
		return false
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

	partial, lookFrom, quoted := extractPartialAndLookFrom(tokens, cursorBytePos)

	// Skip trailing whitespace backwards.
	for lookFrom >= 0 && tokens[lookFrom].Type == TokenWhitespace {
		lookFrom--
	}

	if lookFrom < 0 {
		return CompletionContext{Type: CtxStatementStart, PartialWord: partial, QuotedPartial: quoted}
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

	// dot-qualified context e.g. users.‹cursor› or public.users.‹cursor›
	if tokens[lookFrom].Type == TokenDot {
		qualifier := qualifierBeforeDot(tokens, lookFrom)
		return CompletionContext{
			Type:                CtxAfterDot,
			Qualifier:           qualifier,
			PartialWord:         partial,
			QuotedPartial:       quoted,
			PrecedingTokenType:  precedingType,
			PrecedingTokenValue: precedingValue,
		}
	}

	// walk backwards for nearest clause keyword
	ctx := walkForContext(tokens, lookFrom, partial, precedingType, precedingValue)
	ctx.QuotedPartial = quoted
	return ctx
}

// extractPartialAndLookFrom returns the partial word being typed at the cursor
// and the index of the last token that is fully before (or at) the cursor.
//
//	"SELECT * FROM ta|ble"  →  partial="ta", lookFrom = index of token before "ta"
//	"SELECT * FROM |"       →  partial="",   lookFrom = index of the space token
func extractPartialAndLookFrom(tokens []Token, cursorBytePos int) (partial string, lookFrom int, quoted bool) {
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
				// If immediately preceded by a pure-integer number token with no gap
				// (e.g. "2fa" → Number("2") + Identifier("fa")), include the digits
				// so the partial and replace range cover the full column name.
				if lookFrom >= 0 && tokens[lookFrom].Type == TokenNumber && tokens[lookFrom].End == tok.Start {
					numVal := tokens[lookFrom].Value
					allDigits := true
					for k := 0; k < len(numVal); k++ {
						if numVal[k] < '0' || numVal[k] > '9' {
							allDigits = false
							break
						}
					}
					if allDigits {
						partial = numVal + partial
						lookFrom--
					}
				}
			case TokenQuotedIdentifier:
				// Cursor editing inside a quoted identifier (e.g. "tab, `tab, [tab):
				// treat the text after the opening quote as the partial word so
				// completion still filters and the preceding dot is seen for
				// qualification.
				partial = stripOpenQuote(tok.Value[:cursorBytePos-tok.Start])
				quoted = true
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

func isOpenQuote(ch byte) bool {
	return ch == '"' || ch == '`' || ch == '['
}

func stripOpenQuote(s string) string {
	if len(s) > 0 && isOpenQuote(s[0]) {
		return s[1:]
	}
	return s
}

func UnquoteIdent(s string) string {
	if len(s) == 0 || !isOpenQuote(s[0]) {
		return s
	}
	open := s[0]
	close := open
	if open == '[' {
		close = ']'
	}
	s = strings.TrimPrefix(s, string(open))
	s = strings.TrimSuffix(s, string(close))
	return strings.ReplaceAll(s, string(close)+string(close), string(close))
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
		return UnquoteIdent(t.Value)
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
