package completion

// SymbolKind classifies what a completion symbol represents.
type SymbolKind int

const (
	KindKeyword   SymbolKind = iota
	KindTable                // schema-qualified table
	KindColumn               // column belonging to a table
	KindCTE                  // common table expression name
	KindAlias                // table alias
	KindFunction             // built-in or user-defined function
	KindSchema               // schema name
	KindDDLObject            // DDL object-type keyword (TABLE, VIEW, INDEX, …)
)

// Symbol is a single autocomplete candidate returned by the engine.
type Symbol struct {
	Kind      SymbolKind
	Name      string
	Qualifier string // schema for Table/Schema, table for Column
	TypeHint  string // optional type info ("integer", "text", …)
	IsPK      bool   // true for primary key columns
	Priority  int
	Replace   struct{ Start, End int } // byte range to replace in the source text
}
