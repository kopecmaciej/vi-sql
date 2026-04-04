package database

// DMLKeywords are the top-level statement-opening keywords.
var DMLKeywords = []string{
	"SELECT", "INSERT", "UPDATE", "DELETE", "WITH", "MERGE",
}

// DDLKeywords are DDL statement and object type keywords.
var DDLKeywords = []string{
	"CREATE", "DROP", "ALTER", "TRUNCATE", "RENAME",
	"TABLE", "VIEW", "INDEX", "SCHEMA", "DATABASE", "SEQUENCE",
	"FUNCTION", "PROCEDURE", "TRIGGER", "COLUMN", "ROW", "TYPE",
	"CONSTRAINT", "PRIMARY", "KEY", "FOREIGN", "REFERENCES", "CHECK",
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

// ExpressionKeywords are expression-level and query-structure keywords.
var ExpressionKeywords = []string{
	"AS", "DISTINCT",
	"CASE", "WHEN", "THEN", "ELSE", "END",
	"NULL", "TRUE", "FALSE", "DEFAULT",
	"FETCH", "NEXT", "ROWS", "ONLY", "FOR",
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

// TransactionKeywords are transaction control keywords.
var TransactionKeywords = []string{
	"DO", "BEGIN", "COMMIT", "ROLLBACK", "SAVEPOINT",
}

// AdminKeywords are database administration keywords.
var AdminKeywords = []string{
	"GRANT", "REVOKE", "EXPLAIN", "ANALYZE", "VACUUM",
}

// SQLKeywords is a flat list of expression-level keywords for autocomplete
// consumers that don't use the context-aware engine (e.g. InputBar).
// Built from the specific slices to avoid duplication.
var SQLKeywords = func() []string {
	out := make([]string, 0,
		len(OperatorKeywords)+len(FunctionKeywords)+len(OrderKeywords)+len(ExpressionKeywords)+9)
	out = append(out, OperatorKeywords...)
	out = append(out, FunctionKeywords...)
	out = append(out, OrderKeywords...)
	out = append(out, ExpressionKeywords...)
	out = append(out,
		"::text", "::int", "::bigint", "::numeric", "::boolean",
		"::date", "::timestamp", "::jsonb",
	)
	return out
}()

// allKeywordWords is used exclusively by the lexer to build its keyword
// recognition set. It covers every group — including DDL, transaction, and
// admin keywords — so all SQL reserved words are syntax-highlighted even
// when they are not offered as autocomplete suggestions.
var allKeywordWords = func() []string {
	groups := [][]string{
		DMLKeywords, DDLKeywords, ClauseKeywords, ExpressionKeywords,
		FunctionKeywords, OperatorKeywords, OrderKeywords,
		TransactionKeywords, AdminKeywords,
	}
	total := 0
	for _, g := range groups {
		total += len(g)
	}
	out := make([]string, 0, total)
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}()
