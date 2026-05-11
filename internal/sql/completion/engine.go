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

// Suggest returns ranked autocomplete symbols for the cursor position in text.
func (e *Engine) Suggest(text string, cursorPos int, cfg Context) []Symbol {
	if cfg.ColumnCache == nil {
		cfg.ColumnCache = make(map[string][]Column)
	}

	tokens := sql.Tokenize(text)
	ctx := sql.DetectContext(tokens, cursorPos)
	scope := BuildScope(text)
	partial := strings.ToLower(ctx.PartialWord)

	var candidates []Symbol
	for _, p := range e.providers {
		if p.Applicable(ctx.Type, scope) {
			candidates = append(candidates, p.Suggest(ctx, scope, partial, cfg)...)
		}
	}
	return Rank(candidates, partial, scope)
}
