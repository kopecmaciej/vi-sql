package completion

import (
	sql "github.com/kopecmaciej/vi-sql/internal/sql"
)

// Provider contributes autocomplete candidates for a specific context.
type Provider interface {
	// Applicable reports whether this provider contributes to ctx.
	Applicable(ctx sql.ContextType, scope *QueryScope) bool
	// Suggest returns candidates. partial is already lowercased.
	Suggest(ctx sql.CompletionContext, scope *QueryScope, partial string, cfg Context) []Symbol
}
