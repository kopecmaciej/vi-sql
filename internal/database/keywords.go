package database

// DMLKeywords are the top-level statement-opening keywords.
var DMLKeywords = []string{
	"SELECT", "INSERT", "UPDATE", "DELETE", "WITH", "MERGE",
}

// ClauseKeywords are mid-statement structural keywords.
var ClauseKeywords = []string{
	"FROM", "WHERE", "JOIN", "LEFT JOIN", "RIGHT JOIN", "INNER JOIN",
	"FULL OUTER JOIN", "CROSS JOIN",
	"ON", "USING",
	"GROUP BY", "ORDER BY", "HAVING",
	"LIMIT", "OFFSET",
	"SET", "INTO", "VALUES", "RETURNING",
	"UNION", "UNION ALL", "INTERSECT", "EXCEPT",
}

// FunctionKeywords are built-in SQL functions.
var FunctionKeywords = []string{
	"COUNT", "SUM", "AVG", "MIN", "MAX",
	"COALESCE", "NULLIF", "CAST",
	"LOWER", "UPPER", "TRIM", "LENGTH", "SUBSTRING", "REPLACE", "CONCAT",
	"NOW", "CURRENT_TIMESTAMP", "CURRENT_DATE", "DATE_TRUNC", "EXTRACT",
	"ARRAY_AGG", "STRING_AGG", "JSON_AGG", "JSONB_AGG",
	"ROW_NUMBER", "RANK", "DENSE_RANK", "LAG", "LEAD",
}

// OperatorKeywords are SQL boolean / comparison keywords used in expressions.
var OperatorKeywords = []string{
	"AND", "OR", "NOT",
	"IN", "NOT IN", "BETWEEN", "NOT BETWEEN",
	"LIKE", "NOT LIKE", "ILIKE", "NOT ILIKE",
	"IS NULL", "IS NOT NULL",
	"EXISTS", "NOT EXISTS",
	"ANY", "ALL", "SOME",
}

// OrderKeywords are used after ORDER BY.
var OrderKeywords = []string{
	"ASC", "DESC", "NULLS FIRST", "NULLS LAST",
}

// SQLKeywords provides SQL keywords for autocomplete.
// Kept for backward compatibility with InputBar and other consumers.
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
