package sql

import (
	"slices"
	"sort"
	"strings"
)

const formatIndent = "    "

// clausePhrases are ClauseKeywords split into words and sorted longest-first,
// so a greedy match at a keyword token prefers "LEFT JOIN" over "JOIN".
var clausePhrases = func() [][]string {
	phrases := make([][]string, 0, len(ClauseKeywords))
	for _, kw := range ClauseKeywords {
		phrases = append(phrases, strings.Fields(strings.ToUpper(kw)))
	}
	sort.Slice(phrases, func(a, b int) bool { return len(phrases[a]) > len(phrases[b]) })
	return phrases
}()

// Format returns a pretty-printed copy of sql: each top-level clause keyword
// (FROM, WHERE, JOIN, GROUP BY, ...) starts a new line, parenthesized
// subqueries indent one level deeper, and semicolon-separated statements are
// formatted independently. Column lists and expressions are left inline.
func Format(sql string) string {
	statements, trailingSemicolon := splitStatements(dropWhitespaceTokens(Tokenize(sql)))

	formatted := make([]string, 0, len(statements))
	for _, stmt := range statements {
		if s := formatStatement(stmt); s != "" {
			formatted = append(formatted, s)
		}
	}
	result := strings.Join(formatted, ";\n\n")
	if trailingSemicolon && result != "" {
		result += ";"
	}
	return result
}

func dropWhitespaceTokens(tokens []Token) []Token {
	out := tokens[:0]
	for _, t := range tokens {
		if t.Type != TokenWhitespace {
			out = append(out, t)
		}
	}
	return out
}

// splitStatements breaks tokens on top-level (paren depth 0) semicolons.
// trailingSemicolon reports whether tokens ended with such a semicolon, so
func splitStatements(tokens []Token) (statements [][]Token, trailingSemicolon bool) {
	depth := 0
	start := 0
	for i, t := range tokens {
		if t.Type != TokenPunctuation {
			continue
		}
		switch t.Value {
		case "(":
			depth++
		case ")":
			if depth > 0 {
				depth--
			}
		case ";":
			if depth == 0 {
				if i > start {
					statements = append(statements, tokens[start:i])
				}
				start = i + 1
			}
		}
	}
	if start < len(tokens) {
		statements = append(statements, tokens[start:])
	} else {
		trailingSemicolon = len(tokens) > 0
	}
	return statements, trailingSemicolon
}

func formatStatement(tokens []Token) string {
	if len(tokens) == 0 {
		return ""
	}
	clauseBreak := clauseBreaks(tokens)

	var out strings.Builder
	depth := 0
	var subqueryParen []bool
	pendingBreak := false
	var prev *Token

	writeBreak := func(d int) {
		out.WriteString("\n")
		out.WriteString(strings.Repeat(formatIndent, d))
	}

	for i := range tokens {
		tok := tokens[i]

		if tok.Type == TokenPunctuation && tok.Value == ")" && depth > 0 {
			depth--
			isSubquery := false
			if n := len(subqueryParen); n > 0 {
				isSubquery = subqueryParen[n-1]
				subqueryParen = subqueryParen[:n-1]
			}
			if isSubquery {
				writeBreak(depth)
				out.WriteString(")")
				pendingBreak = false
				prev = &tokens[i]
				continue
			}
		}

		switch {
		case pendingBreak:
			writeBreak(depth)
		case i == 0:
			// no separator before the first token
		case clauseBreak[i]:
			writeBreak(depth)
		case needsSpace(prev, &tok):
			out.WriteString(" ")
		}
		pendingBreak = false

		out.WriteString(tok.Value)

		switch {
		case tok.Type == TokenPunctuation && tok.Value == "(":
			depth++
			nested := i+1 < len(tokens) && isDMLStart(tokens[i+1])
			subqueryParen = append(subqueryParen, nested)
			if nested {
				pendingBreak = true
			}
		case tok.Type == TokenComment && strings.HasPrefix(tok.Value, "--"):
			pendingBreak = true
		}

		prev = &tokens[i]
	}
	return out.String()
}

// clauseBreaks marks, for each token index, whether a clause phrase (e.g.
// "WHERE", "GROUP BY", "LEFT JOIN") starts there — at any paren depth, so
// subquery bodies get the same clause-level breaks as the outer statement.
// The formatter's own depth tracking supplies the indent for each break.
func clauseBreaks(tokens []Token) []bool {
	breaks := make([]bool, len(tokens))
	for i := 0; i < len(tokens); {
		t := tokens[i]
		if t.Type == TokenKeyword {
			if n := matchClausePhrase(tokens, i); n > 0 {
				breaks[i] = true
				i += n
				continue
			}
		}
		i++
	}
	return breaks
}

func matchClausePhrase(tokens []Token, i int) int {
	for _, phrase := range clausePhrases {
		if i+len(phrase) > len(tokens) {
			continue
		}
		matched := true
		for j, word := range phrase {
			tk := tokens[i+j]
			if tk.Type != TokenKeyword || strings.ToUpper(tk.Value) != word {
				matched = false
				break
			}
		}
		if matched {
			return len(phrase)
		}
	}
	return 0
}

func isDMLStart(t Token) bool {
	if t.Type != TokenKeyword {
		return false
	}
	v := strings.ToUpper(t.Value)
	return slices.Contains(DMLKeywords, v)
}

func isFunctionKeyword(t Token) bool {
	if t.Type != TokenKeyword {
		return false
	}
	v := strings.ToUpper(t.Value)
	return slices.Contains(FunctionKeywords, v)
}

// needsSpace decides whether a space belongs between prev and cur when
// neither a structural newline nor a "no separator" rule applies.
func needsSpace(prev, cur *Token) bool {
	if prev == nil {
		return false
	}
	if prev.Type == TokenPunctuation && prev.Value == "(" {
		return false
	}
	if prev.Type == TokenDot || prev.Type == TokenTypecast {
		return false
	}
	if cur.Type == TokenDot || cur.Type == TokenTypecast {
		return false
	}
	if cur.Type == TokenPunctuation {
		switch cur.Value {
		case ",", ")", ";":
			return false
		case "(":
			if prev.Type == TokenIdentifier || prev.Type == TokenQuotedIdentifier || isFunctionKeyword(*prev) {
				return false
			}
		}
	}
	return true
}
