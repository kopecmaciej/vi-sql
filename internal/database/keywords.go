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

// SQLKeywords is a flat list of expression-level keywords for autocomplete
// consumers that don't use the context-aware engine (e.g. InputBar).
// Built from the specific slices to avoid duplication.
var SQLKeywords = func() []string {
	out := make([]string, 0, len(OperatorKeywords)+len(FunctionKeywords)+len(OrderKeywords)+12)
	out = append(out, OperatorKeywords...)
	out = append(out, FunctionKeywords...)
	out = append(out, OrderKeywords...)
	out = append(out,
		"TRUE", "FALSE", "NULL",
		"::text", "::int", "::bigint", "::numeric", "::boolean",
		"::date", "::timestamp", "::jsonb",
	)
	return out
}()
