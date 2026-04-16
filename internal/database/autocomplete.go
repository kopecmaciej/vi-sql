package database

import (
	"strings"
)

// TableRef holds a schema-qualified table reference parsed from a FROM/JOIN clause.
// Schema may be empty for unqualified table names; Alias may be empty if none is given.
type TableRef struct {
	Schema string
	Table  string
	Alias  string
}

// tableTerminators are keywords that end a table reference and cannot be an alias.
var tableTerminators = map[string]bool{
	"WHERE": true, "JOIN": true, "LEFT": true, "RIGHT": true, "INNER": true,
	"FULL": true, "CROSS": true, "NATURAL": true, "ON": true, "USING": true,
	"GROUP": true, "ORDER": true, "HAVING": true, "LIMIT": true, "OFFSET": true,
	"UNION": true, "INTERSECT": true, "EXCEPT": true, "SET": true,
	"RETURNING": true, "INTO": true, "VALUES": true, "SELECT": true, "FROM": true,
	"WITH": true, "FETCH": true, "FOR": true,
}

// skipWS returns the index of the first non-whitespace token at or after start.
func skipWS(tokens []Token, start int) int {
	for start < len(tokens) && tokens[start].Type == TokenWhitespace {
		start++
	}
	return start
}

// ExtractFromTableRefs returns all table references found in FROM/JOIN clauses of sql,
// including any alias declared with or without AS.
func ExtractFromTableRefs(sql string) []TableRef {
	tokens := Tokenize(sql)
	var refs []TableRef
	fromKeywords := map[string]bool{
		"FROM": true, "JOIN": true, "INTO": true,
	}
	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		if tok.Type == TokenKeyword && fromKeywords[strings.ToUpper(tok.Value)] {
			j := skipWS(tokens, i+1)
			if j >= len(tokens) {
				break
			}
			first := tokens[j]
			if first.Type != TokenIdentifier && first.Type != TokenQuotedIdentifier {
				i++
				continue
			}

			ref := TableRef{}
			// check for schema.table
			if j+2 < len(tokens) && tokens[j+1].Type == TokenDot {
				second := tokens[j+2]
				if second.Type == TokenIdentifier || second.Type == TokenQuotedIdentifier {
					ref.Schema = first.Value
					ref.Table = second.Value
					j = j + 3
				} else {
					ref.Table = first.Value
					j = j + 1
				}
			} else {
				ref.Table = first.Value
				j = j + 1
			}

			// Look for optional alias: [AS] identifier
			k := skipWS(tokens, j)
			if k < len(tokens) {
				next := tokens[k]
				if next.Type == TokenKeyword && strings.ToUpper(next.Value) == "AS" {
					k = skipWS(tokens, k+1)
					if k < len(tokens) && (tokens[k].Type == TokenIdentifier || tokens[k].Type == TokenQuotedIdentifier) {
						ref.Alias = tokens[k].Value
					}
				} else if next.Type == TokenIdentifier && !tableTerminators[strings.ToUpper(next.Value)] {
					ref.Alias = next.Value
				}
			}

			refs = append(refs, ref)
			i = j
			continue
		}
		i++
	}
	return refs
}

// ExtractCTENames returns the names of all CTEs declared in a WITH clause.
func ExtractCTENames(sql string) []string {
	tokens := Tokenize(sql)
	var names []string
	for i, tok := range tokens {
		if tok.Type != TokenKeyword || strings.ToUpper(tok.Value) != "WITH" {
			continue
		}
		j := skipWS(tokens, i+1)
		for j < len(tokens) {
			if tokens[j].Type != TokenIdentifier {
				break
			}
			cteName := tokens[j].Value
			j = skipWS(tokens, j+1)
			if j >= len(tokens) || strings.ToUpper(tokens[j].Value) != "AS" {
				break
			}
			names = append(names, cteName)
			j = skipWS(tokens, j+1)
			if j >= len(tokens) || tokens[j].Value != "(" {
				break
			}
			// skip the CTE body tracking parenthesis depth
			depth := 0
			for j < len(tokens) {
				if tokens[j].Type == TokenPunctuation {
					switch tokens[j].Value {
					case "(":
						depth++
					case ")":
						depth--
						if depth == 0 {
							j++
							goto afterBody
						}
					}
				}
				j++
			}
		afterBody:
			j = skipWS(tokens, j)
			// comma → another CTE follows
			if j < len(tokens) && tokens[j].Type == TokenPunctuation && tokens[j].Value == "," {
				j = skipWS(tokens, j+1)
				continue
			}
			break
		}
	}
	return names
}

// IsExplainQuery reports whether sql starts with the EXPLAIN keyword.
func IsExplainQuery(sql string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "EXPLAIN")
}

// AutocompleteEntry is a suggestion returned by BuildSQLAutocomplete.
// Main is the text to insert; Secondary is a hint shown alongside it.
type AutocompleteEntry struct {
	Main      string
	Secondary string
}

// BuildSQLAutocomplete returns autocomplete suggestions for a SQL text at the
// given cursor byte position, using the supplied schema, column, and variable lists.
// variables holds dynamically extracted identifiers such as CTE names and table aliases.
func BuildSQLAutocomplete(
	text string,
	cursorBytePos int,
	schemas []Schema,
	columns []string,
	variables []string,
) []AutocompleteEntry {
	tokens := Tokenize(text)
	ctx := DetectContext(tokens, cursorBytePos)
	partial := strings.ToLower(ctx.PartialWord)

	var entries []AutocompleteEntry

	switch ctx.Type {
	case CtxStatementStart:
		if partial == "" {
			return nil
		}
		for _, kw := range append(DMLKeywords, ClauseKeywords...) {
			if strings.HasPrefix(strings.ToLower(kw), partial) {
				entries = append(entries, AutocompleteEntry{Main: kw})
			}
		}

	case CtxAfterFrom, CtxAfterJoin, CtxAfterInto:
		// After a complete table reference (not after a comma), show clause keywords.
		if partial == "" && ctx.PrecedingTokenType == TokenIdentifier {
			for _, kw := range ClauseKeywords {
				entries = append(entries, AutocompleteEntry{Main: kw})
			}
			return entries
		}
		for _, v := range variables {
			if partial == "" || strings.HasPrefix(strings.ToLower(v), partial) {
				entries = append(entries, AutocompleteEntry{Main: v, Secondary: "cte"})
			}
		}
		for _, schema := range schemas {
			for _, table := range schema.Tables {
				qualified := schema.Schema + "." + table
				if partial == "" || strings.HasPrefix(strings.ToLower(qualified), partial) ||
					strings.HasPrefix(strings.ToLower(table), partial) {
					entries = append(entries, AutocompleteEntry{
						Main: qualified,
					})
				}
			}
		}
		for _, kw := range ClauseKeywords {
			if partial != "" && strings.HasPrefix(strings.ToLower(kw), partial) {
				entries = append(entries, AutocompleteEntry{Main: kw})
			}
		}

	case CtxAfterSelect, CtxAfterWhere, CtxAfterOn,
		CtxAfterOrderBy, CtxAfterGroupBy, CtxAfterSet:
		// Cursor is inside or just after a literal value — no suggestions.
		if partial == "" && (ctx.PrecedingTokenType == TokenNumber || ctx.PrecedingTokenType == TokenString) {
			return nil
		}
		// After a complete column name (not after a comma), show clause keywords.
		if partial == "" && ctx.PrecedingTokenType == TokenIdentifier {
			for _, kw := range ClauseKeywords {
				entries = append(entries, AutocompleteEntry{Main: kw})
			}
			return entries
		}
		for _, col := range columns {
			if partial == "" || strings.HasPrefix(strings.ToLower(col), partial) {
				entries = append(entries, AutocompleteEntry{Main: col})
			}
		}
		for _, kw := range SQLKeywords {
			if partial != "" && strings.HasPrefix(strings.ToLower(kw), partial) {
				entries = append(entries, AutocompleteEntry{Main: kw})
			}
		}
		for _, kw := range ClauseKeywords {
			if partial != "" && strings.HasPrefix(strings.ToLower(kw), partial) {
				entries = append(entries, AutocompleteEntry{Main: kw})
			}
		}

	case CtxAfterDot:
		// If qualifier is a schema name, show its tables instead of columns.
		for _, schema := range schemas {
			if strings.EqualFold(ctx.TableName, schema.Schema) {
				for _, table := range schema.Tables {
					if partial == "" || strings.HasPrefix(strings.ToLower(table), partial) {
						entries = append(entries, AutocompleteEntry{
							Main: table,
						})
					}
				}
				return entries
			}
		}
		for _, col := range columns {
			if partial == "" || strings.HasPrefix(strings.ToLower(col), partial) {
				entries = append(entries, AutocompleteEntry{Main: col})
			}
		}

	default:
		if partial == "" {
			return nil
		}
		for _, kw := range SQLKeywords {
			if strings.HasPrefix(strings.ToLower(kw), partial) {
				entries = append(entries, AutocompleteEntry{Main: kw})
			}
		}
	}

	return entries
}
