package completion

import (
	"strings"

	sql "github.com/kopecmaciej/vi-sql/internal/sql"
)

// DDLObjectProvider suggests DDL object-type keywords (TABLE, VIEW, INDEX, …)
// after a DDL verb.
type DDLObjectProvider struct{}

func (DDLObjectProvider) Applicable(ctx sql.ContextType, _ *QueryScope) bool {
	return ctx == sql.CtxAfterDDLVerb
}

func (DDLObjectProvider) Suggest(_ sql.CompletionContext, _ *QueryScope, partial string, _ Context) []Symbol {
	var out []Symbol
	for _, kw := range sql.DDLObjectKeywords {
		if partial == "" || strings.HasPrefix(strings.ToLower(kw), partial) {
			out = append(out, Symbol{Kind: KindDDLObject, Name: kw, Priority: 50})
		}
	}
	return out
}
