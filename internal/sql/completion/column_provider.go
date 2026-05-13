package completion

import (
	"strings"

	sql "github.com/kopecmaciej/vi-sql/internal/sql"
)

// ColumnProvider suggests column names based on tables in scope.
type ColumnProvider struct{}

var columnContexts = map[sql.ContextType]bool{
	sql.CtxAfterSelect:  true,
	sql.CtxAfterWhere:   true,
	sql.CtxAfterHaving:  true,
	sql.CtxAfterOn:      true,
	sql.CtxAfterOrderBy: true,
	sql.CtxAfterGroupBy: true,
	sql.CtxAfterSet:     true,
}

func (ColumnProvider) Applicable(ctx sql.ContextType, _ *QueryScope) bool {
	return columnContexts[ctx]
}

func (ColumnProvider) Suggest(_ sql.CompletionContext, scope *QueryScope, partial string, cfg Context) []Symbol {
	cols := resolveColumns(scope, cfg)

	var out []Symbol
	for _, col := range cols {
		if matchesPartial(strings.ToLower(col.Name), partial) {
			out = append(out, Symbol{Kind: KindColumn, Name: col.Name, TypeHint: col.TypeHint, IsPK: col.IsPK, IsFK: col.IsFK, Priority: 50})
		}
	}
	return out
}

// resolveColumns fetches and merges columns from all in-scope tables.
func resolveColumns(scope *QueryScope, cfg Context) []Column {
	if scope == nil || len(scope.TableRefs) == 0 {
		return nil
	}

	seen := make(map[string]bool)
	var cols []Column

	for _, ref := range scope.TableRefs {
		for _, c := range fetchColumns(ref.Schema, ref.Table, cfg) {
			if !seen[c.Name] {
				seen[c.Name] = true
				cols = append(cols, c)
			}
		}
	}
	return cols
}

// fetchColumns returns columns for schema.table, using the cache when available.
func fetchColumns(schema, table string, cfg Context) []Column {
	qualifiedKey := schema + "." + table
	if cols, ok := cfg.ColumnCache[qualifiedKey]; ok {
		return cols
	}
	if cols, ok := cfg.ColumnCache[table]; ok {
		return cols
	}

	if cfg.ColumnFetcher != nil {
		if cols, err := cfg.ColumnFetcher(schema, table); err == nil {
			cfg.ColumnCache[qualifiedKey] = cols
			return cols
		}
		if schema != "" {
			if cols, err := cfg.ColumnFetcher("", table); err == nil {
				cfg.ColumnCache[table] = cols
				return cols
			}
		}
	}
	return nil
}
