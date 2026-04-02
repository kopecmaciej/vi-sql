package database

import "strings"

// ContextType describes what the SQL editor expects at the cursor position.
type ContextType int

const (
	CtxStatementStart ContextType = iota // beginning of a statement or after DML keyword
	CtxAfterSelect                       // column / expression list
	CtxAfterFrom                         // table / schema reference
	CtxAfterJoin                         // table after a JOIN keyword
	CtxAfterWhere                        // predicate expression
	CtxAfterOn                           // JOIN … ON condition
	CtxAfterOrderBy                      // column reference in ORDER BY
	CtxAfterGroupBy                      // column reference in GROUP BY
	CtxAfterSet                          // column = value in UPDATE SET
	CtxAfterInto                         // table name in INSERT INTO
	CtxAfterValues                       // value list after VALUES
	CtxAfterDot                          // schema.‹table› or table.‹column›
	CtxGeneral                           // fallback — no useful clause found
)

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

	// Capture the type of the last non-whitespace token before the cursor.
	precedingType := tokens[lookFrom].Type

	// ── dot-qualified context ─────────────────────────────────────────────────
	// e.g.  users.‹cursor›  or  public.users.‹cursor›
	if tokens[lookFrom].Type == TokenDot {
		qualifier := qualifierBeforeDot(tokens, lookFrom)
		return CompletionContext{
			Type:               CtxAfterDot,
			TableName:          qualifier,
			PartialWord:        partial,
			PrecedingTokenType: precedingType,
		}
	}

	// ── walk backwards for nearest clause keyword ─────────────────────────────
	return walkForContext(tokens, lookFrom, partial, precedingType)
}

// extractPartialAndLookFrom returns the partial word being typed at the cursor
// and the index of the last token that is fully before (or at) the cursor.
//
//   "SELECT * FROM ta|ble"  →  partial="ta", lookFrom = index of token before "ta"
//   "SELECT * FROM |"       →  partial="",   lookFrom = index of the space token
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
			if tok.Type == TokenIdentifier || tok.Type == TokenKeyword {
				// Word token: cursor is still editing it — treat as partial.
				partial = tok.Value[:cursorBytePos-tok.Start]
				lookFrom = i - 1
			} else {
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
	if t.Type == TokenIdentifier || t.Type == TokenQuotedIdentifier {
		return t.Value
	}
	return ""
}

// walkForContext scans backwards from startIdx to find the nearest SQL clause
// keyword and returns the corresponding CompletionContext.
func walkForContext(tokens []Token, startIdx int, partial string, precedingType TokenType) CompletionContext {
	for i := startIdx; i >= 0; i-- {
		tok := tokens[i]
		if tok.Type != TokenKeyword {
			continue
		}
		switch strings.ToUpper(tok.Value) {
		case "SELECT":
			return CompletionContext{Type: CtxAfterSelect, PartialWord: partial, PrecedingTokenType: precedingType}

		case "FROM":
			return CompletionContext{Type: CtxAfterFrom, PartialWord: partial, PrecedingTokenType: precedingType}

		case "JOIN":
			return CompletionContext{Type: CtxAfterJoin, PartialWord: partial, PrecedingTokenType: precedingType}

		case "ON":
			return CompletionContext{Type: CtxAfterOn, PartialWord: partial, PrecedingTokenType: precedingType}

		case "WHERE", "HAVING":
			return CompletionContext{Type: CtxAfterWhere, PartialWord: partial, PrecedingTokenType: precedingType}

		case "BY":
			// Distinguish ORDER BY vs GROUP BY by looking at the preceding keyword.
			for j := i - 1; j >= 0; j-- {
				if tokens[j].Type == TokenWhitespace {
					continue
				}
				if tokens[j].Type == TokenKeyword {
					switch strings.ToUpper(tokens[j].Value) {
					case "ORDER":
						return CompletionContext{Type: CtxAfterOrderBy, PartialWord: partial, PrecedingTokenType: precedingType}
					case "GROUP":
						return CompletionContext{Type: CtxAfterGroupBy, PartialWord: partial, PrecedingTokenType: precedingType}
					}
				}
				break
			}

		case "SET":
			return CompletionContext{Type: CtxAfterSet, PartialWord: partial, PrecedingTokenType: precedingType}

		case "INTO":
			return CompletionContext{Type: CtxAfterInto, PartialWord: partial, PrecedingTokenType: precedingType}

		case "VALUES":
			return CompletionContext{Type: CtxAfterValues, PartialWord: partial, PrecedingTokenType: precedingType}

		case "LIMIT", "OFFSET":
			return CompletionContext{Type: CtxGeneral, PartialWord: partial, PrecedingTokenType: precedingType}

		case "RETURNING":
			return CompletionContext{Type: CtxAfterSelect, PartialWord: partial, PrecedingTokenType: precedingType}

		case "UNION", "INTERSECT", "EXCEPT":
			return CompletionContext{Type: CtxStatementStart, PartialWord: partial, PrecedingTokenType: precedingType}

		case "INSERT", "UPDATE", "DELETE", "WITH",
			"CREATE", "DROP", "ALTER", "TRUNCATE":
			return CompletionContext{Type: CtxStatementStart, PartialWord: partial, PrecedingTokenType: precedingType}
		}
	}

	return CompletionContext{Type: CtxStatementStart, PartialWord: partial, PrecedingTokenType: precedingType}
}
