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
	qualifier := ctx.Qualifier
	lowerQualifier := strings.ToLower(qualifier)

	if tables, ok := cfg.Index.bySchema[lowerQualifier]; ok {
		var out []Symbol
		for _, it := range tables {
			if matchesPartial(it.lowerTable, partial) {
				out = append(out, Symbol{
					Kind:      KindTable,
					Name:      it.table,
					Qualifier: it.schema,
					Priority:  50,
				})
			}
		}
		return out
	}

	// Resolve alias to real table name.
	realTable := qualifier
	if scope != nil {
		realTable = scope.ResolveTable(qualifier)
	}

	// O(1) lookup: find the schema owning realTable.
	schema := cfg.Index.tableToSchema[strings.ToLower(realTable)]

	cols := fetchColumns(schema, realTable, cfg)
	var out []Symbol
	for _, col := range cols {
		if matchesPartial(strings.ToLower(col.Name), partial) {
			out = append(out, Symbol{
				Kind:      KindColumn,
				Name:      col.Name,
				TypeHint:  col.TypeHint,
				IsPK:      col.IsPK,
				IsFK:      col.IsFK,
				Qualifier: realTable,
				Priority:  60,
			})
		}
	}
	return out
}
