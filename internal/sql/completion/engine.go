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
	IsFK     bool
}

// indexedTable is a precomputed row built once per schema load.
type indexedTable struct {
	schema         string
	table          string
	qualified      string // "schema.table" original case
	lowerQualified string // lowercase for prefix matching
	lowerTable     string // lowercase for prefix matching
}

// SchemaIndex is a precomputed lookup structure built once from []database.Schema.
// Providers read from it instead of iterating cfg.Schemas on every keystroke.
type SchemaIndex struct {
	all           []indexedTable
	bySchema      map[string][]indexedTable // lowercase schema → tables
	tableToSchema map[string]string         // lowercase table → schema name
}

// BuildSchemaIndex builds a SchemaIndex from a schema list.
// Call once when schemas are loaded or refreshed; pass via Context.Index.
func BuildSchemaIndex(schemas []database.Schema) *SchemaIndex {
	idx := &SchemaIndex{
		bySchema:      make(map[string][]indexedTable, len(schemas)),
		tableToSchema: make(map[string]string),
	}
	for _, s := range schemas {
		lowerSchema := strings.ToLower(s.Schema)
		for _, t := range s.Tables {
			lowerT := strings.ToLower(t)
			it := indexedTable{
				schema:         s.Schema,
				table:          t,
				qualified:      s.Schema + "." + t,
				lowerQualified: lowerSchema + "." + lowerT,
				lowerTable:     lowerT,
			}
			idx.all = append(idx.all, it)
			idx.bySchema[lowerSchema] = append(idx.bySchema[lowerSchema], it)
			idx.tableToSchema[lowerT] = s.Schema
		}
	}
	return idx
}

// Context carries the runtime data the engine needs to produce suggestions.
type Context struct {
	Schemas       []database.Schema
	Index         *SchemaIndex // precomputed index; built lazily if nil
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
	if cfg.Index == nil {
		cfg.Index = BuildSchemaIndex(cfg.Schemas)
	}

	sqlCtx := sql.DetectContext(tokens, cursorPos)
	var scope *QueryScope
	if scopeRequired(sqlCtx.Type) {
		scope = BuildScope(tokens, cursorPos)
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
	if sqlCtx.QuotedPartial {
		replaceStart-- // include the opening quote char in the replaced range
	}
	for i := range ranked {
		ranked[i].Replace.Start = replaceStart
		ranked[i].Replace.End = cursorPos
		ranked[i].Quoted = sqlCtx.QuotedPartial
	}
	return ranked
}
