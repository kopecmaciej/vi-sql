package completion

import (
	sql "github.com/kopecmaciej/vi-sql/internal/sql"
)

// TableProvider suggests schema-qualified table names.
type TableProvider struct{}

var tableContexts = map[sql.ContextType]bool{
	sql.CtxAfterFrom:      true,
	sql.CtxAfterJoin:      true,
	sql.CtxAfterInto:      true,
	sql.CtxExistingObject: true,
}

func (TableProvider) Applicable(ctx sql.ContextType, _ *QueryScope) bool {
	return tableContexts[ctx]
}

func (TableProvider) Suggest(ctx sql.CompletionContext, scope *QueryScope, partial string, cfg Context) []Symbol {
	// After a complete table reference (e.g. "FROM users |"), the user is past
	// the table name — suppress the table list so clause keywords can take over.
	if partial == "" && isAtValuePosition(ctx) {
		return nil
	}
	var out []Symbol
	for _, it := range cfg.Index.all {
		if matchesPartial(it.lowerQualified, partial) || matchesPartial(it.lowerTable, partial) {
			prio := 30
			if scope != nil && scope.HasTable(it.table) {
				prio += 10
			}
			out = append(out, Symbol{
				Kind:      KindTable,
				Name:      it.qualified,
				Qualifier: it.schema,
				Priority:  prio,
			})
		}
	}
	return out
}
