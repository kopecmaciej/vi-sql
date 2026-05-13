package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIdent(t *testing.T) {
	tests := []struct {
		name   string
		quoter Quoter
		input  string
		want   string
	}{
		{"ansi wraps name in double quotes", ANSIQuoter, "users", `"users"`},
		{"backtick wraps name in backticks", BacktickQuoter, "users", "`users`"},
		{"ansi quotes a reserved word", ANSIQuoter, "order", `"order"`},
		{"backtick quotes a reserved word", BacktickQuoter, "key", "`key`"},
		{"ansi doubles embedded double quotes", ANSIQuoter, `we"ird`, `"we""ird"`},
		{"backtick doubles embedded backticks", BacktickQuoter, "we`ird", "`we``ird`"},
		{"ansi leaves embedded backticks unescaped", ANSIQuoter, "ab`c", "\"ab`c\""},
		{"backtick leaves embedded double quotes unescaped", BacktickQuoter, `ab"c`, "`ab\"c`"},
		{"empty name still gets quoted", ANSIQuoter, "", `""`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.quoter.Ident(tt.input))
		})
	}
}

func TestTable_QuotesBothParts(t *testing.T) {
	assert.Equal(t, `"public"."users"`, ANSIQuoter.Table("public", "users"))
	assert.Equal(t, "`auth`.`users`", BacktickQuoter.Table("auth", "users"))
}

func TestTable_QuotesReservedWords(t *testing.T) {
	assert.Equal(t, `"public"."order"`, ANSIQuoter.Table("public", "order"))
}

func TestTable_EscapesEmbeddedQuotesInBothParts(t *testing.T) {
	assert.Equal(t, `"we""ird"."col""umn"`, ANSIQuoter.Table(`we"ird`, `col"umn`))
}

func TestWhereEqAnon_SingleColumn(t *testing.T) {
	parts, args := ANSIQuoter.WhereEqAnon(map[string]any{"id": 42})
	assert.Equal(t, []string{`"id" = ?`}, parts)
	assert.Equal(t, []any{42}, args)
}

func TestWhereEqAnon_AppliesDialectQuoting(t *testing.T) {
	parts, _ := BacktickQuoter.WhereEqAnon(map[string]any{"key": "abc"})
	assert.Equal(t, []string{"`key` = ?"}, parts)
}

func TestWhereEqAnon_CompositeKey_OneClausePerColumn(t *testing.T) {
	parts, args := ANSIQuoter.WhereEqAnon(map[string]any{"tenant_id": 1, "user_id": 2})
	assert.ElementsMatch(t, []string{`"tenant_id" = ?`, `"user_id" = ?`}, parts)
	assert.ElementsMatch(t, []any{1, 2}, args)
}

func TestWhereEqAnon_EmptyMap(t *testing.T) {
	parts, args := ANSIQuoter.WhereEqAnon(map[string]any{})
	assert.Empty(t, parts)
	assert.Empty(t, args)
}

func TestWhereEqIndexed_SingleColumnGetsDollarOne(t *testing.T) {
	parts, args := ANSIQuoter.WhereEqIndexed(map[string]any{"id": 42})
	assert.Equal(t, []string{`"id" = $1`}, parts)
	assert.Equal(t, []any{42}, args)
}

func TestWhereEqIndexed_PlaceholdersAreSequential(t *testing.T) {
	parts, _ := ANSIQuoter.WhereEqIndexed(map[string]any{"a": 1, "b": 2, "c": 3})

	suffixes := make([]string, len(parts))
	for i, p := range parts {
		suffixes[i] = p[len(p)-2:]
	}
	assert.Equal(t, []string{"$1", "$2", "$3"}, suffixes)
}

func TestWhereEqIndexed_EmptyMap(t *testing.T) {
	parts, args := ANSIQuoter.WhereEqIndexed(map[string]any{})
	assert.Empty(t, parts)
	assert.Empty(t, args)
}
