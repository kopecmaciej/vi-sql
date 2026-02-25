package database

// SQLKeywords provides SQL keywords for autocomplete.
var SQLKeywords = []string{
	// Clauses
	"AND", "OR", "NOT", "IN", "BETWEEN", "LIKE", "ILIKE",
	"IS", "IS NOT", "IS NULL", "IS NOT NULL",
	"EXISTS", "ANY", "ALL",
	// Comparisons
	"=", "!=", "<>", "<", ">", "<=", ">=",
	// Functions
	"COUNT", "SUM", "AVG", "MIN", "MAX",
	"COALESCE", "NULLIF", "CAST",
	"LOWER", "UPPER", "TRIM", "LENGTH",
	"NOW", "CURRENT_TIMESTAMP", "CURRENT_DATE",
	// Order
	"ASC", "DESC", "NULLS FIRST", "NULLS LAST",
	// Values
	"TRUE", "FALSE", "NULL",
	// Type casts
	"::text", "::int", "::bigint", "::numeric", "::boolean",
	"::date", "::timestamp", "::jsonb",
}

// SQLAutocomplete provides autocomplete entries combining SQL keywords
// and table-specific column names.
type SQLAutocomplete struct {
	keywords []string
	columns  []string
}

func NewSQLAutocomplete() *SQLAutocomplete {
	return &SQLAutocomplete{
		keywords: SQLKeywords,
	}
}

func (a *SQLAutocomplete) SetColumns(columns []string) {
	a.columns = columns
}

func (a *SQLAutocomplete) GetSuggestions(prefix string) []string {
	var suggestions []string

	// Columns first, then keywords
	for _, col := range a.columns {
		suggestions = append(suggestions, col)
	}
	for _, kw := range a.keywords {
		suggestions = append(suggestions, kw)
	}

	return suggestions
}
