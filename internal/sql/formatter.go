package sql

import (
	"slices"
	"sort"
	"strings"
)

const (
	formatIndent         = "    " //4 x space
	maxInlineSelectWidth = 80     // TODO : move this to config (possibly)
)

// clausePhrasesByFirstWord are ClauseKeywords split into words and group by
// their first word, so it can be quickly match with tokens eg: phrase:
// [UNION]:[UNION][ALL] can be quickly paired with token [UNION].
// We sort, so longer pairs like "UNION ALL" can be match first, before "UNION"
var clausePhrasesByFirstWord = func() map[string][][]string {
	phrases := make([][]string, 0, len(ClauseKeywords))
	for _, kw := range ClauseKeywords {
		phrases = append(phrases, strings.Fields(strings.ToUpper(kw)))
	}
	sort.Slice(phrases, func(a, b int) bool { return len(phrases[a]) > len(phrases[b]) })

	byFirstWord := make(map[string][][]string)
	for _, p := range phrases {
		byFirstWord[p[0]] = append(byFirstWord[p[0]], p)
	}
	return byFirstWord
}()

// Format modify sql query to pretty-printed state. Each top-level clause keyword
// starts new line, parenthesized subqueries indent one level deeper, and
// semicolon-separated statements are formatted independently. SELECT lists are split
// if their length exceeds maxInlineSelectWidth
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

// splitStatements breaks tokens into top-level statements
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
	selectCommaBreak := selectCommaBreaks(tokens)

	var out strings.Builder
	depth := 0
	var subqueryParen []bool
	pendingBreakDepth := -1
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
				pendingBreakDepth = -1
				prev = &tokens[i]
				continue
			}
		}

		switch {
		case pendingBreakDepth >= 0:
			writeBreak(pendingBreakDepth)
		case i == 0:
			// no separator before the first token
		case clauseBreak[i]:
			writeBreak(depth)
		case needsSpace(prev, &tok):
			out.WriteString(" ")
		}
		pendingBreakDepth = -1

		out.WriteString(tok.Value)

		switch {
		case tok.Type == TokenPunctuation && tok.Value == "(":
			depth++
			nested := i+1 < len(tokens) && isDMLStart(tokens[i+1])
			subqueryParen = append(subqueryParen, nested)
			if nested {
				pendingBreakDepth = depth
			}
		case tok.Type == TokenComment && strings.HasPrefix(tok.Value, "--"):
			pendingBreakDepth = depth
		case tok.Type == TokenPunctuation && tok.Value == "," && selectCommaBreak[i]:
			pendingBreakDepth = depth + 1
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

// selectCommaBreaks marks, for each token index, whether it is a top-level
// comma in a SELECT column list that should be followed by a line break.
func selectCommaBreaks(tokens []Token) []bool {
	breaks := make([]bool, len(tokens))
	depth := 0
	for i, t := range tokens {
		if t.Type == TokenPunctuation {
			switch t.Value {
			case "(":
				depth++
			case ")":
				if depth > 0 {
					depth--
				}
			}
		}
		if t.Type != TokenKeyword || strings.ToUpper(t.Value) != "SELECT" {
			continue
		}
		start := i + 1
		if start < len(tokens) && tokens[start].Type == TokenKeyword && strings.ToUpper(tokens[start].Value) == "DISTINCT" {
			start++
		}
		commas, end := selectListRange(tokens, start, depth)
		if len(commas) == 0 {
			continue
		}
		if depth*len(formatIndent)+inlineWidth(tokens[i:end]) <= maxInlineSelectWidth {
			continue
		}
		for _, ci := range commas {
			breaks[ci] = true
		}
	}
	return breaks
}

// selectListRange returns the indexes of top-level (paren depth baseDepth)
// commas between start and end, plus end itself — the index of the next
// clause keyword or closing paren at that depth, marking the list's end.
func selectListRange(tokens []Token, start, baseDepth int) (commas []int, end int) {
	depth := baseDepth
	for i := start; i < len(tokens); i++ {
		t := tokens[i]
		if t.Type == TokenPunctuation {
			switch t.Value {
			case "(":
				depth++
				continue
			case ")":
				if depth == baseDepth {
					return commas, i
				}
				depth--
				continue
			case ",":
				if depth == baseDepth {
					commas = append(commas, i)
				}
				continue
			}
		}
		if depth == baseDepth && t.Type == TokenKeyword && matchClausePhrase(tokens, i) > 0 {
			return commas, i
		}
	}
	return commas, len(tokens)
}

// inlineWidth returns the rendered character width of tokens if placed on one
// line with no breaks, using the same spacing rule as the main formatting pass.
func inlineWidth(tokens []Token) int {
	width := 0
	var prev *Token
	for i := range tokens {
		if needsSpace(prev, &tokens[i]) {
			width++
		}
		width += len(tokens[i].Value)
		prev = &tokens[i]
	}
	return width
}

func matchClausePhrase(tokens []Token, i int) int {
	if tokens[i].Type != TokenKeyword {
		return 0
	}
	for _, phrase := range clausePhrasesByFirstWord[strings.ToUpper(tokens[i].Value)] {
		if i+len(phrase) > len(tokens) {
			continue
		}
		matched := true
		for j := 1; j < len(phrase); j++ {
			tk := tokens[i+j]
			if tk.Type != TokenKeyword || strings.ToUpper(tk.Value) != phrase[j] {
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
