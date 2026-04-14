package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsExplainQuery(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want bool
	}{
		{"EXPLAIN keyword", "EXPLAIN SELECT * FROM t", true},
		{"EXPLAIN ANALYZE", "EXPLAIN ANALYZE SELECT * FROM t", true},
		{"lowercase explain", "explain select * from t", true},
		{"leading whitespace", "  EXPLAIN SELECT 1", true},
		{"plain SELECT", "SELECT * FROM t", false},
		{"empty string", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsExplainQuery(tc.sql))
		})
	}
}

func TestExtractFromTableRefs(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want []TableRef
	}{
		{
			name: "simple FROM",
			sql:  "SELECT * FROM users",
			want: []TableRef{{Table: "users"}},
		},
		{
			name: "schema-qualified table",
			sql:  "SELECT * FROM public.users",
			want: []TableRef{{Schema: "public", Table: "users"}},
		},
		{
			name: "table with AS alias",
			sql:  "SELECT * FROM users AS u",
			want: []TableRef{{Table: "users", Alias: "u"}},
		},
		{
			name: "table with implicit alias",
			sql:  "SELECT * FROM users u",
			want: []TableRef{{Table: "users", Alias: "u"}},
		},
		{
			name: "JOIN clause",
			sql:  "SELECT * FROM users u JOIN orders o ON u.id = o.user_id",
			want: []TableRef{{Table: "users", Alias: "u"}, {Table: "orders", Alias: "o"}},
		},
		{
			name: "schema-qualified with alias",
			sql:  "SELECT * FROM public.users u",
			want: []TableRef{{Schema: "public", Table: "users", Alias: "u"}},
		},
		{
			name: "INSERT INTO",
			sql:  "INSERT INTO users (id) VALUES (1)",
			want: []TableRef{{Table: "users"}},
		},
		{
			name: "no table reference",
			sql:  "SELECT 1",
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractFromTableRefs(tc.sql)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExtractCTENames(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want []string
	}{
		{
			name: "WITH name AS (...) returns the CTE name",
			sql:  "WITH cte AS (SELECT 1) SELECT * FROM cte",
			want: []string{"cte"},
		},
		{
			name: "WITH a AS (...), b AS (...) returns both CTE names in order",
			sql:  "WITH a AS (SELECT 1), b AS (SELECT 2) SELECT * FROM a JOIN b ON true",
			want: []string{"a", "b"},
		},
		{
			name: "query without WITH clause returns nil",
			sql:  "SELECT * FROM t",
			want: nil,
		},
		{
			name: "function call inside CTE body does not confuse paren depth tracking",
			sql:  "WITH users AS (SELECT max(id) FROM t) SELECT * FROM users",
			want: []string{"users"},
		},
		{
			name: "subquery inside CTE body does not confuse paren depth tracking",
			sql:  "WITH cte AS (SELECT id FROM (SELECT id FROM t) sub) SELECT * FROM cte",
			want: []string{"cte"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractCTENames(tc.sql)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestBuildSQLAutocomplete(t *testing.T) {
	schemas := []SchemaWithTables{
		{Schema: "public", Tables: []string{"users", "orders"}},
	}
	columns := []string{"id", "email", "status"}

	t.Run("suggest tables after FROM", func(t *testing.T) {
		sql := "SELECT * FROM "
		entries := BuildSQLAutocomplete(sql, len(sql), schemas, columns, nil)
		mains := extractMains(entries)
		assert.Contains(t, mains, "public.users")
		assert.Contains(t, mains, "public.orders")
	})

	t.Run("filter tables by partial after FROM", func(t *testing.T) {
		sql := "SELECT * FROM public.us"
		entries := BuildSQLAutocomplete(sql, len(sql), schemas, columns, nil)
		mains := extractMains(entries)
		assert.Contains(t, mains, "users")
		assert.NotContains(t, mains, "orders")
	})

	t.Run("suggest columns after WHERE", func(t *testing.T) {
		sql := "SELECT * FROM users WHERE "
		entries := BuildSQLAutocomplete(sql, len(sql), schemas, columns, nil)
		mains := extractMains(entries)
		assert.Contains(t, mains, "id")
		assert.Contains(t, mains, "email")
	})

	t.Run("filter columns by partial", func(t *testing.T) {
		sql := "SELECT * FROM users WHERE em"
		entries := BuildSQLAutocomplete(sql, len(sql), schemas, columns, nil)
		mains := extractMains(entries)
		assert.Contains(t, mains, "email")
		assert.NotContains(t, mains, "id")
	})

	t.Run("CTE names shown as variables after FROM", func(t *testing.T) {
		sql := "WITH my_cte AS (SELECT 1) SELECT * FROM "
		entries := BuildSQLAutocomplete(sql, len(sql), schemas, columns, []string{"my_cte"})
		mains := extractMains(entries)
		assert.Contains(t, mains, "my_cte")
	})
}

func extractMains(entries []AutocompleteEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Main
	}
	return out
}
