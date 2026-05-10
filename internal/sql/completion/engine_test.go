package completion

import (
	"testing"

	"github.com/kopecmaciej/vi-sql/internal/database"
	isql "github.com/kopecmaciej/vi-sql/internal/sql"
)

var testSchemas = []database.Schema{
	{Schema: "public", Tables: []string{"users", "orders"}},
}

func symbolNames(syms []Symbol) []string {
	out := make([]string, len(syms))
	for i, s := range syms {
		out[i] = s.Name
	}
	return out
}

func contains(names []string, name string) bool {
	for _, n := range names {
		if n == name {
			return true
		}
	}
	return false
}

func cfg() Context {
	return Context{Schemas: testSchemas}
}

func TestEngine_TablesAfterFrom(t *testing.T) {
	e := NewDefaultEngine()
	sql := "SELECT * FROM "
	syms := e.Suggest(sql, len(sql), cfg())
	names := symbolNames(syms)
	if !contains(names, "public.users") {
		t.Errorf("expected public.users in suggestions: %v", names)
	}
	if !contains(names, "public.orders") {
		t.Errorf("expected public.orders in suggestions: %v", names)
	}
}

func TestEngine_TablesFilteredByPartialAfterFrom(t *testing.T) {
	e := NewDefaultEngine()
	sql := "SELECT * FROM us"
	syms := e.Suggest(sql, len(sql), cfg())
	names := symbolNames(syms)
	if !contains(names, "public.users") {
		t.Errorf("expected public.users: %v", names)
	}
	if contains(names, "public.orders") {
		t.Errorf("unexpected public.orders in partial match: %v", names)
	}
}

func TestEngine_ColumnsAfterWhere(t *testing.T) {
	cols := []string{"id", "email", "status"}
	c := Context{
		Schemas: testSchemas,
		ColumnFetcher: func(_, _ string) ([]string, error) {
			return cols, nil
		},
		ColumnCache: make(map[string][]string),
	}
	e := NewDefaultEngine()
	sql := "SELECT * FROM users WHERE "
	syms := e.Suggest(sql, len(sql), c)
	names := symbolNames(syms)
	if !contains(names, "id") {
		t.Errorf("expected id in suggestions: %v", names)
	}
	if !contains(names, "email") {
		t.Errorf("expected email in suggestions: %v", names)
	}
}

func TestEngine_ColumnsFilteredByPartial(t *testing.T) {
	cols := []string{"id", "email", "status"}
	c := Context{
		Schemas: testSchemas,
		ColumnFetcher: func(_, _ string) ([]string, error) {
			return cols, nil
		},
		ColumnCache: make(map[string][]string),
	}
	e := NewDefaultEngine()
	sql := "SELECT * FROM users WHERE em"
	syms := e.Suggest(sql, len(sql), c)
	names := symbolNames(syms)
	if !contains(names, "email") {
		t.Errorf("expected email: %v", names)
	}
	if contains(names, "id") {
		t.Errorf("unexpected id in partial 'em' match: %v", names)
	}
}

func TestEngine_CTENamesAfterFrom(t *testing.T) {
	e := NewDefaultEngine()
	sql := "WITH my_cte AS (SELECT 1) SELECT * FROM "
	syms := e.Suggest(sql, len(sql), cfg())
	names := symbolNames(syms)
	if !contains(names, "my_cte") {
		t.Errorf("expected my_cte in suggestions: %v", names)
	}
}

func TestEngine_DDLObjectKeywordsAfterCreate(t *testing.T) {
	e := NewDefaultEngine()
	sql := "CREATE "
	syms := e.Suggest(sql, len(sql), cfg())
	names := symbolNames(syms)
	for _, kw := range []string{"TABLE", "VIEW", "INDEX"} {
		if !contains(names, kw) {
			t.Errorf("expected %s in DDL verb suggestions: %v", kw, names)
		}
	}
}

func TestEngine_DDLObjectFilteredByPartial(t *testing.T) {
	e := NewDefaultEngine()
	sql := "CREATE ta"
	syms := e.Suggest(sql, len(sql), cfg())
	names := symbolNames(syms)
	if !contains(names, "TABLE") {
		t.Errorf("expected TABLE: %v", names)
	}
	if contains(names, "VIEW") {
		t.Errorf("unexpected VIEW in 'ta' partial: %v", names)
	}
}

// TestEngine_CreateObject verifies that CtxCreateObject does NOT suggest existing tables
// (you're naming a new object, not picking an existing one).
func TestEngine_CreateObject_NoExistingTables(t *testing.T) {
	e := NewDefaultEngine()
	sql := "CREATE TABLE "
	syms := e.Suggest(sql, len(sql), cfg())
	names := symbolNames(syms)
	if contains(names, "TABLE") || contains(names, "VIEW") || contains(names, "INDEX") {
		t.Errorf("DDL object keywords must not appear after CREATE TABLE: %v", names)
	}
	// Tables are also not suggested for a new-name context.
	if contains(names, "public.users") {
		t.Errorf("existing tables must not appear in CtxCreateObject: %v", names)
	}
}

// TestEngine_ExistingObject verifies DROP TABLE does suggest existing tables.
func TestEngine_ExistingObject_SuggestsExistingTables(t *testing.T) {
	e := NewDefaultEngine()
	sql := "DROP TABLE us"
	syms := e.Suggest(sql, len(sql), cfg())
	names := symbolNames(syms)
	if !contains(names, "public.users") {
		t.Errorf("expected public.users after DROP TABLE: %v", names)
	}
	if contains(names, "public.orders") {
		t.Errorf("unexpected public.orders when partial is 'us': %v", names)
	}
}

// --- Position-aware keyword tests ---

func TestEngine_SelectStart_OffersStartersAndDistinct(t *testing.T) {
	e := NewDefaultEngine()
	sql := "SELECT "
	names := symbolNames(e.Suggest(sql, len(sql), cfg()))

	for _, kw := range []string{"DISTINCT", "CASE", "NULL", "COUNT"} {
		if !contains(names, kw) {
			t.Errorf("expected %s after 'SELECT ': %v", kw, names)
		}
	}
	for _, bad := range []string{"AS", "AND", "OR", "WHEN", "THEN", "ELSE", "END", "IN", "BETWEEN", "LIKE", "IS NULL", "FETCH"} {
		if contains(names, bad) {
			t.Errorf("must not appear at SELECT expression start: %s in %v", bad, names)
		}
	}
}

func TestEngine_SelectAfterColumn_OffersAS(t *testing.T) {
	e := NewDefaultEngine()
	sql := "SELECT id "
	names := symbolNames(e.Suggest(sql, len(sql), cfg()))

	if !contains(names, "AS") {
		t.Errorf("expected AS after 'SELECT id ': %v", names)
	}
	if !contains(names, "FROM") {
		t.Errorf("expected FROM after 'SELECT id ': %v", names)
	}
	for _, bad := range []string{"CASE", "NULL", "TRUE", "FALSE", "COUNT", "DISTINCT"} {
		if contains(names, bad) {
			t.Errorf("must not appear after a column expression: %s in %v", bad, names)
		}
	}
}

// TestEngine_SelectAfterCompletedExpression verifies that after a completed
// SELECT expression (identifier, wildcard *, literal, aggregate) the engine
// offers AS and FROM — and nothing from the expression-starter or function pools.
func TestEngine_SelectAfterCompletedExpression(t *testing.T) {
	cases := []struct {
		sql  string
		desc string
	}{
		{"SELECT * ", "wildcard star"},
		{"SELECT id, name ", "trailing identifier after comma"},
		{"SELECT COUNT(*) ", "aggregate function call"},
		{"SELECT 'literal' ", "string literal"},
		{"SELECT 1 ", "numeric literal"},
	}

	e := NewDefaultEngine()

	for _, tc := range cases {
		names := symbolNames(e.Suggest(tc.sql, len(tc.sql), cfg()))

		if !contains(names, "FROM") {
			t.Errorf("[%s] expected FROM in suggestions: %v", tc.desc, names)
		}
		if !contains(names, "AS") {
			t.Errorf("[%s] expected AS in suggestions: %v", tc.desc, names)
		}
		for _, kw := range isql.FunctionKeywords {
			if contains(names, kw) {
				t.Errorf("[%s] function keyword %q must not appear after completed expression: %v", tc.desc, kw, names)
			}
		}
		for _, kw := range isql.ExpressionStarters {
			if contains(names, kw) {
				t.Errorf("[%s] expression starter %q must not appear after completed expression: %v", tc.desc, kw, names)
			}
		}
	}
}

func TestEngine_SelectAfterAS_SuppressesAllKeywords(t *testing.T) {
	e := NewDefaultEngine()
	sql := "SELECT id AS "
	syms := e.Suggest(sql, len(sql), cfg())
	for _, s := range syms {
		if s.Kind == KindKeyword {
			t.Errorf("must not suggest keywords after AS (free alias name): got %q", s.Name)
		}
	}
}

func TestEngine_SelectSecondColumn_NoDistinct(t *testing.T) {
	e := NewDefaultEngine()
	sql := "SELECT id, "
	names := symbolNames(e.Suggest(sql, len(sql), cfg()))

	if contains(names, "DISTINCT") {
		t.Errorf("DISTINCT must only appear right after SELECT keyword, not after a comma: %v", names)
	}
	if !contains(names, "CASE") {
		t.Errorf("expected CASE at start of next column: %v", names)
	}
}

func TestEngine_WhereStart_OffersStarters(t *testing.T) {
	e := NewDefaultEngine()
	sql := "SELECT * FROM users WHERE "
	names := symbolNames(e.Suggest(sql, len(sql), cfg()))

	for _, kw := range []string{"NOT", "EXISTS", "CASE", "COUNT"} {
		if !contains(names, kw) {
			t.Errorf("expected %s at WHERE start: %v", kw, names)
		}
	}
	for _, bad := range []string{"AND", "OR", "IN", "BETWEEN", "LIKE", "IS NULL", "AS"} {
		if contains(names, bad) {
			t.Errorf("binary operator %s must not start a WHERE expression: %v", bad, names)
		}
	}
}

func TestEngine_WhereAfterColumn_OffersOperators(t *testing.T) {
	e := NewDefaultEngine()
	sql := "SELECT * FROM users WHERE id "
	names := symbolNames(e.Suggest(sql, len(sql), cfg()))

	for _, kw := range []string{"AND", "OR", "IN", "BETWEEN", "LIKE", "IS NULL", "IS NOT NULL"} {
		if !contains(names, kw) {
			t.Errorf("expected operator %s after WHERE column: %v", kw, names)
		}
	}
	for _, bad := range []string{"CASE", "COUNT", "NULL", "NOT", "EXISTS"} {
		if contains(names, bad) {
			t.Errorf("expression starter %s must not follow a WHERE column: %v", bad, names)
		}
	}
}

func TestEngine_OrderByAfterColumn_OffersAscDesc(t *testing.T) {
	e := NewDefaultEngine()
	sql := "SELECT * FROM users ORDER BY id "
	names := symbolNames(e.Suggest(sql, len(sql), cfg()))

	for _, kw := range []string{"ASC", "DESC"} {
		if !contains(names, kw) {
			t.Errorf("expected %s after ORDER BY column: %v", kw, names)
		}
	}
	if contains(names, "COUNT") {
		t.Errorf("function names must not appear after an ORDER BY column: %v", names)
	}
}

// TestEngine_FromAfterCompleteTableRef verifies that after a fully-typed table
// reference (qualified or not) the engine offers clause keywords, not more tables.
func TestEngine_FromAfterCompleteTableRef(t *testing.T) {
	cases := []struct {
		sql  string
		desc string
	}{
		{"SELECT * FROM users ", "unqualified table"},
		{"SELECT * FROM public.users ", "schema-qualified table"},
		{"SELECT * FROM public.users u ", "table with alias"},
	}

	e := NewDefaultEngine()

	for _, tc := range cases {
		syms := e.Suggest(tc.sql, len(tc.sql), cfg())
		names := symbolNames(syms)

		for _, s := range syms {
			if s.Kind == KindTable {
				t.Errorf("[%s] table %q must not appear after complete table ref", tc.desc, s.Name)
				break
			}
		}
		for _, kw := range []string{"WHERE", "JOIN", "ORDER BY"} {
			if !contains(names, kw) {
				t.Errorf("[%s] expected clause keyword %q in suggestions: %v", tc.desc, kw, names)
			}
		}
	}
}

// --- BuildScope tests ---

func TestBuildScope_SimpleFrom(t *testing.T) {
	scope := BuildScope("SELECT * FROM users")
	if len(scope.TableRefs) != 1 || scope.TableRefs[0].Table != "users" {
		t.Errorf("got %+v", scope.TableRefs)
	}
}

func TestBuildScope_SchemaQualified(t *testing.T) {
	scope := BuildScope("SELECT * FROM public.users")
	if len(scope.TableRefs) != 1 || scope.TableRefs[0].Schema != "public" || scope.TableRefs[0].Table != "users" {
		t.Errorf("got %+v", scope.TableRefs)
	}
}

func TestBuildScope_AliasAS(t *testing.T) {
	scope := BuildScope("SELECT * FROM users AS u")
	if len(scope.TableRefs) != 1 || scope.TableRefs[0].Alias != "u" {
		t.Errorf("got %+v", scope.TableRefs)
	}
}

func TestBuildScope_ImplicitAlias(t *testing.T) {
	scope := BuildScope("SELECT * FROM users u")
	if len(scope.TableRefs) != 1 || scope.TableRefs[0].Alias != "u" {
		t.Errorf("got %+v", scope.TableRefs)
	}
}

func TestBuildScope_JoinClause(t *testing.T) {
	scope := BuildScope("SELECT * FROM users u JOIN orders o ON u.id = o.user_id")
	if len(scope.TableRefs) != 2 {
		t.Fatalf("got %d refs, want 2: %+v", len(scope.TableRefs), scope.TableRefs)
	}
	if scope.TableRefs[0].Table != "users" || scope.TableRefs[0].Alias != "u" {
		t.Errorf("ref[0]=%+v", scope.TableRefs[0])
	}
	if scope.TableRefs[1].Table != "orders" || scope.TableRefs[1].Alias != "o" {
		t.Errorf("ref[1]=%+v", scope.TableRefs[1])
	}
}

func TestBuildScope_CTENames(t *testing.T) {
	scope := BuildScope("WITH cte AS (SELECT 1) SELECT * FROM cte")
	if len(scope.CTENames) != 1 || scope.CTENames[0] != "cte" {
		t.Errorf("got CTENames=%v", scope.CTENames)
	}
}

func TestBuildScope_MultiCTE(t *testing.T) {
	scope := BuildScope("WITH a AS (SELECT 1), b AS (SELECT 2) SELECT * FROM a JOIN b ON true")
	if len(scope.CTENames) != 2 || scope.CTENames[0] != "a" || scope.CTENames[1] != "b" {
		t.Errorf("got CTENames=%v", scope.CTENames)
	}
}

func TestBuildScope_WithRecursive(t *testing.T) {
	sql := "WITH RECURSIVE tree AS (SELECT id FROM nodes WHERE parent IS NULL UNION ALL SELECT n.id FROM nodes n JOIN tree t ON n.parent = t.id) SELECT * FROM tree"
	scope := BuildScope(sql)
	if len(scope.CTENames) != 1 || scope.CTENames[0] != "tree" {
		t.Errorf("WITH RECURSIVE: got CTENames=%v", scope.CTENames)
	}
}

func TestBuildScope_MultiStatement_StopsAtSemicolon(t *testing.T) {
	// stopAtSemicolon returns the last statement — 'a' from the first must not appear.
	scope := BuildScope("SELECT * FROM a; SELECT * FROM b")
	for _, ref := range scope.TableRefs {
		if ref.Table == "a" {
			t.Errorf("first statement table 'a' leaked into scope from second statement: %+v", scope.TableRefs)
		}
	}
}

func TestBuildScope_SubqueryInFrom(t *testing.T) {
	// A subquery in FROM must not promote its inner tables to the outer scope.
	scope := BuildScope("SELECT * FROM (SELECT id FROM inner_table) sub")
	for _, ref := range scope.TableRefs {
		if ref.Table == "inner_table" {
			t.Errorf("inner_table from subquery leaked into outer scope: %+v", scope.TableRefs)
		}
	}
}
