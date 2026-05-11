package completion

import (
	"strings"

	sql "github.com/kopecmaciej/vi-sql/internal/sql"
)

// AliasProvider handles dot-qualified completions (table.column or schema.table).
// For schema qualifiers it returns the schema's tables; for table/alias qualifiers
// it returns the columns of the resolved table.
type AliasProvider struct{}

func (AliasProvider) Applicable(ctx sql.ContextType, _ *QueryScope) bool {
	return ctx == sql.CtxAfterDot
}

func (AliasProvider) Suggest(ctx sql.CompletionContext, scope *QueryScope, partial string, cfg Context) []Symbol {
	qualifier := ctx.TableName

	// If qualifier matches a schema name, return its tables.
	for _, schema := range cfg.Schemas {
		if strings.EqualFold(qualifier, schema.Schema) {
			var out []Symbol
			for _, table := range schema.Tables {
				if partial == "" || strings.HasPrefix(strings.ToLower(table), partial) {
					out = append(out, Symbol{
						Kind:      KindTable,
						Name:      table,
						Qualifier: schema.Schema,
						Priority:  50,
					})
				}
			}
			return out
		}
	}

	// Resolve alias to real table name.
	realTable := qualifier
	if scope != nil {
		realTable = scope.ResolveTable(qualifier)
	}

	// Find schema for the real table.
	schema := ""
	for _, s := range cfg.Schemas {
		for _, t := range s.Tables {
			if strings.EqualFold(t, realTable) {
				schema = s.Schema
				break
			}
		}
		if schema != "" {
			break
		}
	}

	cols := fetchColumns(schema, realTable, cfg)
	var out []Symbol
	for _, col := range cols {
		if partial == "" || strings.HasPrefix(strings.ToLower(col.Name), partial) {
			out = append(out, Symbol{
				Kind:      KindColumn,
				Name:      col.Name,
				TypeHint:  col.TypeHint,
				IsPK:      col.IsPK,
				Qualifier: realTable,
				Priority:  60,
			})
		}
	}
	return out
}
