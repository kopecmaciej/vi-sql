package completion

import (
	"strings"

	"github.com/kopecmaciej/vi-sql/internal/database"
	sql "github.com/kopecmaciej/vi-sql/internal/sql"
)

// Column is a minimal column descriptor used by the completion engine.
type Column struct {
	Name     string
	TypeHint string // e.g. "integer", "text", "timestamp"
	IsPK     bool
}

// Context carries the runtime data the engine needs to produce suggestions.
type Context struct {
	Schemas       []database.Schema
	ColumnFetcher func(schema, table string) ([]Column, error)
	ColumnCache   map[string][]Column // caller-owned; engine reads and writes it
}

// Engine orchestrates providers and the ranker to produce autocomplete symbols.
type Engine struct {
	providers []Provider
}

// NewEngine creates an engine backed by the given providers.
func NewEngine(ps ...Provider) *Engine {
	return &Engine{providers: ps}
}

// NewDefaultEngine creates an engine with the standard set of providers.
func NewDefaultEngine() *Engine {
	return NewEngine(
		KeywordProvider{},
		TableProvider{},
		ColumnProvider{},
		CTEProvider{},
		AliasProvider{},
		DDLObjectProvider{},
	)
}

// scopeRequired reports whether BuildScope needs to run for the given context.
// For contexts where no provider reads scope fields, we skip the scope build.
func scopeRequired(t sql.ContextType) bool {
	switch t {
	case sql.CtxStatementStart, sql.CtxAfterDDLVerb,
		sql.CtxAfterValues, sql.CtxGeneral, sql.CtxCreateObject:
		return false
	}
	return true
}

// Suggest returns ranked autocomplete symbols for the cursor position in text.
func (e *Engine) Suggest(text string, cursorPos int, cfg Context) []Symbol {
	if sql.SuppressRaw(text, cursorPos) {
		return nil
	}
	return e.SuggestTokens(sql.Tokenize(text), text, cursorPos, cfg)
}

// SuggestTokens is like Suggest but accepts pre-tokenized input.
// Use it when the caller already holds a token slice for the same text (e.g.
// syntax highlighting) to avoid a redundant tokenization on every keystroke.
func (e *Engine) SuggestTokens(tokens []sql.Token, text string, cursorPos int, cfg Context) []Symbol {
	if sql.SuppressRaw(text, cursorPos) {
		return nil
	}
	if cfg.ColumnCache == nil {
		cfg.ColumnCache = make(map[string][]Column)
	}

	sqlCtx := sql.DetectContext(tokens, cursorPos)
	var scope *QueryScope
	if scopeRequired(sqlCtx.Type) {
		scope = BuildScope(tokens)
	}
	partial := strings.ToLower(sqlCtx.PartialWord)

	var candidates []Symbol
	for _, p := range e.providers {
		if p.Applicable(sqlCtx.Type, scope) {
			candidates = append(candidates, p.Suggest(sqlCtx, scope, partial, cfg)...)
		}
	}
	ranked := Rank(candidates, partial, scope)
	replaceStart := cursorPos - len(sqlCtx.PartialWord)
	for i := range ranked {
		ranked[i].Replace.Start = replaceStart
		ranked[i].Replace.End = cursorPos
	}
	return ranked
}
