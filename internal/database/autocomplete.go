package database

import (
	"strings"
)

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
