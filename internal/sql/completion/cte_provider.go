package completion

import (
	"strings"

	sql "github.com/kopecmaciej/vi-sql/internal/sql"
)

// CTEProvider suggests CTE names as table references.
type CTEProvider struct{}

func (CTEProvider) Applicable(ctx sql.ContextType, _ *QueryScope) bool {
	return ctx == sql.CtxAfterFrom || ctx == sql.CtxAfterJoin
}

func (CTEProvider) Suggest(_ sql.CompletionContext, scope *QueryScope, partial string, _ Context) []Symbol {
	if scope == nil {
		return nil
	}
	var out []Symbol
	for _, name := range scope.CTENames {
		if matchesPartial(strings.ToLower(name), partial) {
			out = append(out, Symbol{Kind: KindCTE, Name: name, Priority: 40})
		}
	}
	return out
}
