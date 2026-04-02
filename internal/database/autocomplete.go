package database

import (
	"strings"
)

// TableRef holds a schema-qualified table reference parsed from a FROM/JOIN clause.
// Schema may be empty for unqualified table names.
type TableRef struct {
	Schema string
	Table  string
}

// ExtractFromTableRefs returns all table references found in FROM/JOIN clauses of sql.
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
			// skip whitespace
			j := i + 1
			for j < len(tokens) && tokens[j].Type == TokenWhitespace {
				j++
			}
			if j >= len(tokens) {
				break
			}
			first := tokens[j]
			if first.Type != TokenIdentifier && first.Type != TokenQuotedIdentifier {
				i++
				continue
			}
			// check for schema.table
			if j+2 < len(tokens) && tokens[j+1].Type == TokenDot {
				second := tokens[j+2]
				if second.Type == TokenIdentifier || second.Type == TokenQuotedIdentifier {
					refs = append(refs, TableRef{Schema: first.Value, Table: second.Value})
					i = j + 3
					continue
				}
			}
			refs = append(refs, TableRef{Table: first.Value})
			i = j + 1
			continue
		}
		i++
	}
	return refs
}

// HasLimitClause reports whether sql contains a LIMIT keyword.
func HasLimitClause(sql string) bool {
	for _, tok := range Tokenize(sql) {
		if tok.Type == TokenKeyword && strings.ToUpper(tok.Value) == "LIMIT" {
			return true
		}
	}
	return false
}

// AutocompleteEntry is a suggestion returned by BuildSQLAutocomplete.
// Main is the text to insert; Secondary is a hint shown alongside it.
type AutocompleteEntry struct {
	Main      string
	Secondary string
}

// BuildSQLAutocomplete returns autocomplete suggestions for a SQL text at the
// given cursor byte position, using the supplied schema and column lists.
func BuildSQLAutocomplete(
	text string,
	cursorBytePos int,
	schemas []SchemaWithTables,
	columns []string,
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
