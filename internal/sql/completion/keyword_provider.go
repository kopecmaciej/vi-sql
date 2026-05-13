package completion

import (
	"strings"

	sql "github.com/kopecmaciej/vi-sql/internal/sql"
)

// KeywordProvider suggests SQL keywords appropriate for the current clause
// and the cursor's position within an expression. It uses ctx.PrecedingTokenType
// + ctx.PrecedingTokenValue to distinguish "expression-start" positions (after a
// clause keyword, comma, operator) from "post-value" positions (after an
// identifier, literal, or close paren), which need very different keyword sets.
type KeywordProvider struct{}

var keywordContexts = map[sql.ContextType]bool{
	sql.CtxStatementStart: true,
	sql.CtxAfterSelect:    true,
	sql.CtxAfterWhere:     true,
	sql.CtxAfterHaving:    true,
	sql.CtxAfterOn:        true,
	sql.CtxAfterOrderBy:   true,
	sql.CtxAfterGroupBy:   true,
	sql.CtxAfterSet:       true,
	sql.CtxAfterFrom:      true,
	sql.CtxAfterJoin:      true,
	sql.CtxAfterInto:      true,
	sql.CtxAfterDDLVerb:   true,
}

func (KeywordProvider) Applicable(ctx sql.ContextType, _ *QueryScope) bool {
	return keywordContexts[ctx]
}

func (KeywordProvider) Suggest(ctx sql.CompletionContext, _ *QueryScope, partial string, _ Context) []Symbol {
	pre := strings.ToUpper(ctx.PrecedingTokenValue)

	// After AS the user types a free alias name — suppress every keyword.
	if pre == "AS" {
		return nil
	}

	atValue := isAtValuePosition(ctx)

	pool := keywordPool(ctx, atValue, pre)
	if pool == nil {
		return nil
	}

	// Empty partial → only show suggestions for contexts where users expect them.
	// In FROM/JOIN we let TableProvider lead — unless we're past a complete table
	// reference (value position), in which case clause keywords should surface.
	if partial == "" && shouldHideOnEmpty(ctx.Type) && !atValue {
		return nil
	}

	out := make([]Symbol, 0, len(pool))
	for _, kw := range pool {
		if matchesPartial(strings.ToLower(kw), partial) {
			out = append(out, Symbol{Kind: KindKeyword, Name: kw, Priority: 10})
		}
	}
	return out
}

// isAtValuePosition reports whether the cursor sits immediately after a
// completed value (identifier, literal, NULL/TRUE/FALSE, or ")"). In that
// position the user can only continue with operators, AS, or a clause keyword
// — not with an expression-starter like CASE.
func isAtValuePosition(ctx sql.CompletionContext) bool {
	switch ctx.PrecedingTokenType {
	case sql.TokenIdentifier, sql.TokenQuotedIdentifier,
		sql.TokenString, sql.TokenNumber:
		return true
	case sql.TokenPunctuation:
		return ctx.PrecedingTokenValue == ")"
	case sql.TokenKeyword:
		switch strings.ToUpper(ctx.PrecedingTokenValue) {
		case "NULL", "TRUE", "FALSE", "END":
			return true
		}
	}
	return false
}

// shouldHideOnEmpty reports whether to suppress suggestions when the user has
// typed no partial yet. FROM/JOIN are the obvious case — tables are primary,
// and dumping clause keywords there is just noise.
func shouldHideOnEmpty(t sql.ContextType) bool {
	return t == sql.CtxAfterFrom || t == sql.CtxAfterJoin
}

// keywordPool picks the relevant keyword slices for the current context and
// expression position. Returning nil means "no keywords to offer here".
func keywordPool(ctx sql.CompletionContext, atValue bool, pre string) []string {
	switch ctx.Type {
	case sql.CtxStatementStart:
		// Only suggest top-level statement openers when the user is typing.
		if ctx.PartialWord == "" {
			return nil
		}
		return concat(sql.DMLKeywords, sql.DDLVerbKeywords)

	case sql.CtxAfterDDLVerb:
		// Object keywords (TABLE/VIEW/...) handled by DDLObjectProvider.
		return nil

	case sql.CtxAfterFrom, sql.CtxAfterJoin:
		// After a table name: offer JOIN family, WHERE, ORDER BY, etc.
		return sql.ClauseKeywords

	case sql.CtxAfterInto:
		return sql.ClauseKeywords

	case sql.CtxAfterSelect:
		if atValue {
			// "SELECT col |" → AS, comma is implicit, FROM closes the list.
			return []string{"AS", "FROM"}
		}
		// Start of column expression. Include DISTINCT only right after the
		// SELECT keyword itself (no commas yet).
		if pre == "SELECT" {
			return concat(sql.ExpressionStarters, sql.FunctionKeywords, []string{"DISTINCT"})
		}
		return concat(sql.ExpressionStarters, sql.FunctionKeywords)

	case sql.CtxAfterWhere, sql.CtxAfterHaving, sql.CtxAfterOn:
		if atValue {
			return concat(sql.BinaryOperators, sql.PostPredicateKeywords)
		}
		// After any operator (=, <, >, !=, +, …) the RHS is a scalar expression —
		// NOT/EXISTS are predicate prefixes, not values.
		if ctx.PrecedingTokenType == sql.TokenOperator {
			return concat(sql.ExpressionStarters, sql.FunctionKeywords)
		}
		return concat(sql.UnaryOperators, sql.ExpressionStarters, sql.FunctionKeywords)

	case sql.CtxAfterOrderBy:
		if atValue {
			return sql.OrderKeywords
		}
		return sql.FunctionKeywords

	case sql.CtxAfterGroupBy:
		if atValue {
			return nil // post-column the user moves to HAVING/ORDER BY (clause-level)
		}
		return sql.FunctionKeywords

	case sql.CtxAfterSet:
		if atValue {
			return []string{"WHERE", "RETURNING"}
		}
		return concat(sql.ExpressionStarters, sql.FunctionKeywords)
	}
	return nil
}

func concat(slices ...[]string) []string {
	total := 0
	for _, s := range slices {
		total += len(s)
	}
	out := make([]string, 0, total)
	for _, s := range slices {
		out = append(out, s...)
	}
	return out
}
