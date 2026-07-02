package completion

// SymbolKind classifies what a completion symbol represents.
type SymbolKind int

const (
	KindKeyword SymbolKind = iota
	KindSchema
	KindTable
	KindColumn
	KindCTE       // common table expression name
	KindAlias     // table alias
	KindFunction  // built-in or user-defined function
	KindDDLObject // DDL object-type keyword (TABLE, VIEW, INDEX, …)
)

// Symbol is a single autocomplete candidate returned by the engine.
type Symbol struct {
	Kind      SymbolKind
	Name      string
	Qualifier string // schema for Table/Schema, table for Column
	TypeHint  string // optional type info ("integer", "text", …)
	IsPK      bool
	IsFK      bool
	Quoted    bool // partial was typed inside an opening quote → emit a fully-quoted identifier
	Priority  int
	Replace   struct{ Start, End int } // byte range to replace in the source text
}
