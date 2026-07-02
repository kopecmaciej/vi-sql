package completion

import (
	"reflect"
	"slices"
	"testing"

	"github.com/kopecmaciej/vi-sql/internal/database"
	isql "github.com/kopecmaciej/vi-sql/internal/sql"
)

var testSchemas = []database.Schema{
	{Schema: "public", Tables: []string{"users", "orders"}},
	{Schema: "catalog", Tables: []string{"products", "categories"}},
	{Schema: "orders", Tables: []string{"orders", "payments"}},
}

var testColumns = map[string][]Column{
	"users":      {{Name: "id"}, {Name: "name"}, {Name: "email"}, {Name: "status"}, {Name: "created_at"}},
	"orders":     {{Name: "id"}, {Name: "user_id"}, {Name: "method"}, {Name: "amount"}, {Name: "status"}, {Name: "created_at"}},
	"products":   {{Name: "id"}, {Name: "category_id"}, {Name: "name"}, {Name: "price"}, {Name: "stock"}},
	"categories": {{Name: "id"}, {Name: "name"}, {Name: "description"}},
	"payments":   {{Name: "id"}, {Name: "order_id"}, {Name: "amount"}, {Name: "method"}},
}

func symbolNames(syms []Symbol) []string {
	out := make([]string, len(syms))
	for i, s := range syms {
		out[i] = s.Name
	}
	return out
}

func cfg() Context { return Context{Schemas: testSchemas} }

func cfgWithCols() Context {
	return Context{
		Schemas: testSchemas,
		ColumnFetcher: func(_, table string) ([]Column, error) {
			if cols, ok := testColumns[table]; ok {
				return cols, nil
			}
			return nil, nil
		},
		ColumnCache: make(map[string][]Column),
	}
}

// TestEngine_FROM_Tables covers table listing and partial filtering in FROM context,
// including schema-qualified qualifiers and CTE names.
func TestEngine_FROM_Tables(t *testing.T) {
	e := NewDefaultEngine()
	cases := []struct {
		sql    string
		desc   string
		want   []string
		noWant []string
	}{
		{
			sql:  "SELECT * FROM ",
			desc: "all tables from all schemas listed",
			want: []string{"public.users", "public.orders", "catalog.products"},
		},
		{
			sql:    "SELECT * FROM us",
			desc:   "partial 'us' filters to users only",
			want:   []string{"public.users"},
			noWant: []string{"public.orders", "catalog.products"},
		},
		{
			sql:  "SELECT * FROM catalog.",
			desc: "schema dot gives schema's tables",
			want: []string{"products", "categories"},
		},
		{
			sql:  "WITH my_cte AS (SELECT 1) SELECT * FROM ",
			desc: "CTE name appears alongside regular tables",
			want: []string{"my_cte", "public.users"},
		},
	}

	for _, tc := range cases {
		names := symbolNames(e.Suggest(tc.sql, len(tc.sql), cfg()))
		for _, w := range tc.want {
			if !slices.Contains(names, w) {
				t.Errorf("[%s] expected %q in suggestions: %v", tc.desc, w, names)
			}
		}
		for _, nw := range tc.noWant {
			if slices.Contains(names, nw) {
				t.Errorf("[%s] unexpected %q in suggestions: %v", tc.desc, nw, names)
			}
		}
	}
}

// TestEngine_FROM_AfterCompleteTableRef verifies that after a fully-typed table
// reference the engine offers clause keywords (WHERE, JOIN, ORDER BY) not more tables.
func TestEngine_FROM_AfterCompleteTableRef(t *testing.T) {
	cases := []struct {
		sql  string
		desc string
	}{
		{"SELECT * FROM users ", "unqualified table"},
		{"SELECT * FROM public.users ", "schema-qualified table"},
		{"SELECT * FROM public.users u ", "table with alias"},
		{"SELECT * FROM orders.orders ", "schema-qualified from non-public schema"},
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
			if !slices.Contains(names, kw) {
				t.Errorf("[%s] expected clause keyword %q: %v", tc.desc, kw, names)
			}
		}
	}
}

// TestEngine_JOIN_Tables verifies table suggestions after JOIN keywords,
// including partial filtering.
func TestEngine_JOIN_Tables(t *testing.T) {
	e := NewDefaultEngine()
	cases := []struct {
		sql    string
		desc   string
		want   []string
		noWant []string
	}{
		{
			sql:  "SELECT * FROM users JOIN ",
			desc: "all tables listed after JOIN",
			want: []string{"public.orders", "catalog.products"},
		},
		{
			sql:  "SELECT * FROM users LEFT JOIN ",
			desc: "all tables listed after LEFT JOIN",
			want: []string{"public.orders", "catalog.products"},
		},
		{
			sql:    "SELECT * FROM users JOIN ord",
			desc:   "partial 'ord' filters to orders tables only",
			want:   []string{"public.orders", "orders.orders"},
			noWant: []string{"public.users", "catalog.products"},
		},
	}

	for _, tc := range cases {
		names := symbolNames(e.Suggest(tc.sql, len(tc.sql), cfg()))
		for _, w := range tc.want {
			if !slices.Contains(names, w) {
				t.Errorf("[%s] expected %q: %v", tc.desc, w, names)
			}
		}
		for _, nw := range tc.noWant {
			if slices.Contains(names, nw) {
				t.Errorf("[%s] unexpected %q: %v", tc.desc, nw, names)
			}
		}
	}
}

// TestEngine_JOIN_AfterCompleteRef verifies that after a complete JOIN table ref
// clause keywords (ON, WHERE) are offered, not more tables.
func TestEngine_JOIN_AfterCompleteRef(t *testing.T) {
	e := NewDefaultEngine()
	cases := []struct {
		sql  string
		desc string
	}{
		{"SELECT * FROM users JOIN orders ", "JOIN unqualified table"},
		{"SELECT * FROM users u JOIN orders o ", "JOIN with aliases"},
		{"SELECT * FROM catalog.products cp JOIN catalog.categories cc ", "JOIN schema-qualified with aliases"},
	}

	for _, tc := range cases {
		syms := e.Suggest(tc.sql, len(tc.sql), cfg())
		names := symbolNames(syms)

		for _, s := range syms {
			if s.Kind == KindTable {
				t.Errorf("[%s] table %q must not appear after complete JOIN ref", tc.desc, s.Name)
				break
			}
		}
		for _, kw := range []string{"ON", "WHERE"} {
			if !slices.Contains(names, kw) {
				t.Errorf("[%s] expected %q after complete JOIN ref: %v", tc.desc, kw, names)
			}
		}
	}
}

// TestEngine_JOIN_ON_Columns verifies that column suggestions appear in the ON clause.
func TestEngine_JOIN_ON_Columns(t *testing.T) {
	e := NewDefaultEngine()
	c := cfgWithCols()

	cases := []struct {
		sql  string
		desc string
		want []string
	}{
		{
			sql:  "SELECT * FROM users u JOIN orders o ON ",
			desc: "columns from both tables available in ON",
			want: []string{"id", "email", "user_id", "method"},
		},
		{
			sql:  "SELECT * FROM catalog.products cp JOIN catalog.categories cc ON ",
			desc: "columns from schema-qualified joins available in ON",
			want: []string{"id", "category_id", "name", "description"},
		},
	}

	for _, tc := range cases {
		names := symbolNames(e.Suggest(tc.sql, len(tc.sql), c))
		for _, w := range tc.want {
			if !slices.Contains(names, w) {
				t.Errorf("[%s] expected column %q in ON clause: %v", tc.desc, w, names)
			}
		}
	}
}

// TestEngine_SELECT_ExpressionPositions covers the three positions in the SELECT
// column list: right after SELECT (with DISTINCT), after a comma (no DISTINCT),
// and after AS (no keywords at all).
func TestEngine_SELECT_ExpressionPositions(t *testing.T) {
	e := NewDefaultEngine()

	// Right after SELECT: DISTINCT + expression starters + functions; no binary operators.
	t.Run("after SELECT keyword", func(t *testing.T) {
		names := symbolNames(e.Suggest("SELECT ", len("SELECT "), cfg()))
		for _, kw := range append(append(isql.ExpressionStarters, isql.FunctionKeywords...), "DISTINCT") {
			if !slices.Contains(names, kw) {
				t.Errorf("expected %q right after SELECT: %v", kw, names)
			}
		}
		for _, kw := range isql.BinaryOperators {
			if slices.Contains(names, kw) {
				t.Errorf("binary operator %q must not appear right after SELECT: %v", kw, names)
			}
		}
	})

	// After a comma: DISTINCT must not reappear.
	t.Run("after comma", func(t *testing.T) {
		names := symbolNames(e.Suggest("SELECT id, ", len("SELECT id, "), cfg()))
		if slices.Contains(names, "DISTINCT") {
			t.Errorf("DISTINCT must not appear after comma in SELECT list: %v", names)
		}
		for _, kw := range isql.ExpressionStarters {
			if !slices.Contains(names, kw) {
				t.Errorf("expected expression starter %q after comma: %v", kw, names)
			}
		}
	})

	// After AS: no keywords at all (user types a free alias name).
	t.Run("after AS", func(t *testing.T) {
		syms := e.Suggest("SELECT id AS ", len("SELECT id AS "), cfg())
		for _, s := range syms {
			if s.Kind == KindKeyword {
				t.Errorf("must not suggest keywords after AS: got %q", s.Name)
			}
		}
	})
}

// TestEngine_SELECT_AfterCompletedExpression verifies that after a completed SELECT
// expression AS and FROM are offered, while expression starters and function keywords
// are suppressed.
func TestEngine_SELECT_AfterCompletedExpression(t *testing.T) {
	cases := []struct {
		sql  string
		desc string
	}{
		{"SELECT * ", "wildcard star"},
		{"SELECT id ", "single identifier"},
		{"SELECT id, name ", "trailing identifier after comma"},
		{"SELECT COUNT(*) ", "aggregate function call"},
		{"SELECT 'literal' ", "string literal"},
		{"SELECT 1 ", "numeric literal"},
	}

	e := NewDefaultEngine()

	for _, tc := range cases {
		names := symbolNames(e.Suggest(tc.sql, len(tc.sql), cfg()))

		if !slices.Contains(names, "FROM") {
			t.Errorf("[%s] expected FROM: %v", tc.desc, names)
		}
		if !slices.Contains(names, "AS") {
			t.Errorf("[%s] expected AS: %v", tc.desc, names)
		}
		for _, kw := range isql.FunctionKeywords {
			if slices.Contains(names, kw) {
				t.Errorf("[%s] function keyword %q must not appear after completed expression: %v", tc.desc, kw, names)
			}
		}
		for _, kw := range isql.ExpressionStarters {
			if slices.Contains(names, kw) {
				t.Errorf("[%s] expression starter %q must not appear after completed expression: %v", tc.desc, kw, names)
			}
		}
	}
}

// TestEngine_SELECT_InJoinQuery tests cursor mid-SELECT in a real JOIN query.
// User scenario: SELECT cp.category_id, cc.name, ‹cursor› FROM catalog.products cp
// JOIN catalog.categories cc ON cc.id = cp.category_id
func TestEngine_SELECT_InJoinQuery(t *testing.T) {
	query := "SELECT cp.category_id, cc.name, FROM catalog.products cp JOIN catalog.categories cc ON cc.id = cp.category_id"
	cursorPos := len("SELECT cp.category_id, cc.name, ")

	e := NewDefaultEngine()
	names := symbolNames(e.Suggest(query, cursorPos, cfg()))

	for _, kw := range isql.ExpressionStarters {
		if !slices.Contains(names, kw) {
			t.Errorf("expected expression starter %q in mid-JOIN SELECT list: %v", kw, names)
		}
	}
	for _, kw := range isql.FunctionKeywords {
		if !slices.Contains(names, kw) {
			t.Errorf("expected function keyword %q in mid-JOIN SELECT list: %v", kw, names)
		}
	}
	for _, bad := range []string{"DISTINCT", "FROM", "AS"} {
		if slices.Contains(names, bad) {
			t.Errorf("%q must not appear after comma in SELECT list: %v", bad, names)
		}
	}
}

// TestEngine_WHERE_Keywords covers both the expression-start position (after WHERE
// or AND/OR) and the post-value position (after a column name or literal), using
// realistic queries including schema-qualified tables and multi-join FROM clauses.
func TestEngine_WHERE_Keywords(t *testing.T) {
	e := NewDefaultEngine()

	// Expression-start: unary starters and functions; no binary operators.
	startCases := []struct {
		sql  string
		desc string
	}{
		{"SELECT * FROM users WHERE ", "simple WHERE start"},
		{"SELECT * FROM users WHERE id = 1 AND ", "after AND"},
		{"SELECT * FROM orders.orders WHERE status = 'paid' AND ", "schema-qualified table, after AND"},
		{"SELECT * FROM catalog.products cp JOIN catalog.categories cc ON cc.id = cp.category_id WHERE ", "multi-join WHERE start"},
	}
	for _, tc := range startCases {
		names := symbolNames(e.Suggest(tc.sql, len(tc.sql), cfg()))
		for _, kw := range []string{"NOT", "EXISTS", "CASE"} {
			if !slices.Contains(names, kw) {
				t.Errorf("[%s] expected %q at WHERE start: %v", tc.desc, kw, names)
			}
		}
		for _, kw := range isql.BinaryOperators {
			if slices.Contains(names, kw) {
				t.Errorf("[%s] binary operator %q must not appear at expression start: %v", tc.desc, kw, names)
			}
		}
	}

	// Post-value: binary operators; no expression starters or functions.
	postValueCases := []struct {
		sql  string
		desc string
	}{
		{"SELECT * FROM users WHERE id ", "simple column"},
		{"SELECT * FROM orders.orders WHERE method ", "schema-qualified table column"},
		{"SELECT * FROM catalog.products cp JOIN catalog.categories cc ON cc.id = cp.category_id WHERE cp.price ", "multi-join column"},
	}
	for _, tc := range postValueCases {
		names := symbolNames(e.Suggest(tc.sql, len(tc.sql), cfg()))
		for _, kw := range isql.BinaryOperators {
			if !slices.Contains(names, kw) {
				t.Errorf("[%s] expected binary operator %q: %v", tc.desc, kw, names)
			}
		}
		for _, kw := range append(isql.ExpressionStarters, isql.FunctionKeywords...) {
			if slices.Contains(names, kw) {
				t.Errorf("[%s] expression starter/function %q must not appear after column: %v", tc.desc, kw, names)
			}
		}
	}
}

// TestEngine_WHERE_PostPredicateClauseKeywords verifies that ORDER BY and other
// clause keywords appear after a complete WHERE predicate (value position).
func TestEngine_WHERE_PostPredicateClauseKeywords(t *testing.T) {
	e := NewDefaultEngine()
	cases := []struct {
		sql  string
		desc string
	}{
		{"SELECT * FROM users WHERE status IS NOT NULL ", "IS NOT NULL"},
		{"SELECT * FROM users WHERE status = 'active' ", "string literal equality"},
		{"SELECT * FROM users WHERE id = 1 ", "numeric equality"},
		{"SELECT * FROM users WHERE id ", "bare column identifier"},
	}
	for _, tc := range cases {
		names := symbolNames(e.Suggest(tc.sql, len(tc.sql), cfg()))
		for _, kw := range isql.PostPredicateKeywords {
			if !slices.Contains(names, kw) {
				t.Errorf("[%s] expected clause keyword %q after complete predicate: %v", tc.desc, kw, names)
			}
		}
	}
}

// TestEngine_WHERE_AfterComparisonOp verifies that after a comparison operator
// (=, <, >, !=, …) UnaryOperators are suppressed and scalar starters are offered.
func TestEngine_WHERE_AfterComparisonOp(t *testing.T) {
	e := NewDefaultEngine()

	cases := []struct {
		sql  string
		desc string
	}{
		{"SELECT * FROM users WHERE id = ", "equals"},
		{"SELECT * FROM users WHERE id != ", "not-equals"},
		{"SELECT * FROM orders WHERE amount > ", "greater-than"},
		{"SELECT * FROM orders WHERE amount <= ", "less-or-equal"},
	}
	for _, tc := range cases {
		names := symbolNames(e.Suggest(tc.sql, len(tc.sql), cfg()))
		for _, kw := range isql.UnaryOperators {
			if slices.Contains(names, kw) {
				t.Errorf("[%s] unary operator %q must not appear after comparison op: %v", tc.desc, kw, names)
			}
		}
		for _, kw := range isql.ExpressionStarters {
			if !slices.Contains(names, kw) {
				t.Errorf("[%s] expected expression starter %q after comparison op: %v", tc.desc, kw, names)
			}
		}
	}
}

// TestEngine_WHERE_Columns verifies column suggestions after WHERE, with and without
// partial filtering, including schema-qualified and multi-join queries.
func TestEngine_WHERE_Columns(t *testing.T) {
	e := NewDefaultEngine()
	c := cfgWithCols()

	cases := []struct {
		sql    string
		desc   string
		want   []string
		noWant []string
	}{
		{
			sql:  "SELECT * FROM users WHERE ",
			desc: "all user columns available",
			want: []string{"id", "email", "status"},
		},
		{
			sql:    "SELECT * FROM users WHERE em",
			desc:   "partial 'em' filters to email only",
			want:   []string{"email"},
			noWant: []string{"id", "status"},
		},
		{
			sql:  "SELECT * FROM orders.orders WHERE ",
			desc: "schema-qualified table columns available",
			want: []string{"id", "method", "amount"},
		},
		{
			sql:  `SELECT * FROM "orders"."orders" WHERE `,
			desc: "quoted schema-qualified table columns available",
			want: []string{"id", "method", "amount"},
		},
		{
			sql:  "SELECT * FROM users u JOIN orders o ON u.id = o.user_id WHERE ",
			desc: "columns from both joined tables available",
			want: []string{"id", "email", "method", "amount"},
		},
		{
			sql:  "SELECT * FROM catalog.products cp JOIN catalog.categories cc ON cc.id = cp.category_id WHERE ",
			desc: "columns from schema-qualified join tables available",
			want: []string{"category_id", "price", "name", "description"},
		},
	}

	for _, tc := range cases {
		names := symbolNames(e.Suggest(tc.sql, len(tc.sql), c))
		for _, w := range tc.want {
			if !slices.Contains(names, w) {
				t.Errorf("[%s] expected column %q: %v", tc.desc, w, names)
			}
		}
		for _, nw := range tc.noWant {
			if slices.Contains(names, nw) {
				t.Errorf("[%s] unexpected column %q: %v", tc.desc, nw, names)
			}
		}
	}
}

// TestEngine_ORDER_BY_Keywords covers both positions in ORDER BY: expression start
// (functions offered) and post-column (ASC/DESC offered).
func TestEngine_ORDER_BY_Keywords(t *testing.T) {
	e := NewDefaultEngine()

	// Expression start: function keywords.
	names := symbolNames(e.Suggest("SELECT * FROM users ORDER BY ", len("SELECT * FROM users ORDER BY "), cfg()))
	for _, kw := range isql.FunctionKeywords {
		if !slices.Contains(names, kw) {
			t.Errorf("expected function %q at ORDER BY start: %v", kw, names)
		}
	}

	// Post-column: order direction keywords, no functions.
	names = symbolNames(e.Suggest("SELECT * FROM users ORDER BY id ", len("SELECT * FROM users ORDER BY id "), cfg()))
	for _, kw := range isql.OrderKeywords {
		if !slices.Contains(names, kw) {
			t.Errorf("expected %q after ORDER BY column: %v", kw, names)
		}
	}
	for _, kw := range isql.FunctionKeywords {
		if slices.Contains(names, kw) {
			t.Errorf("function %q must not appear after ORDER BY column: %v", kw, names)
		}
	}
}

// TestEngine_HAVING_Keywords covers both HAVING positions: expression start (functions)
// and post-aggregate (binary operators).
func TestEngine_HAVING_Keywords(t *testing.T) {
	e := NewDefaultEngine()

	// Expression start: functions and expression starters; no binary operators.
	names := symbolNames(e.Suggest(
		"SELECT id, COUNT(*) FROM users GROUP BY id HAVING ",
		len("SELECT id, COUNT(*) FROM users GROUP BY id HAVING "),
		cfg(),
	))
	for _, kw := range isql.FunctionKeywords {
		if !slices.Contains(names, kw) {
			t.Errorf("expected function %q at HAVING start: %v", kw, names)
		}
	}
	for _, kw := range isql.BinaryOperators {
		if slices.Contains(names, kw) {
			t.Errorf("binary operator %q must not appear at HAVING start: %v", kw, names)
		}
	}

	// Post-aggregate: binary operators.
	names = symbolNames(e.Suggest(
		"SELECT id, COUNT(*) FROM users GROUP BY id HAVING COUNT(*) ",
		len("SELECT id, COUNT(*) FROM users GROUP BY id HAVING COUNT(*) "),
		cfg(),
	))
	for _, kw := range isql.BinaryOperators {
		if !slices.Contains(names, kw) {
			t.Errorf("expected binary operator %q after HAVING aggregate: %v", kw, names)
		}
	}
}

// TestEngine_DDL_AfterVerb verifies DDL object keyword suggestions after CREATE/DROP,
// with and without partial filtering.
func TestEngine_DDL_AfterVerb(t *testing.T) {
	e := NewDefaultEngine()
	cases := []struct {
		sql    string
		desc   string
		want   []string
		noWant []string
	}{
		{
			sql:  "CREATE ",
			desc: "all DDL object types after CREATE",
			want: isql.DDLObjectKeywords,
		},
		{
			sql:    "CREATE ta",
			desc:   "partial 'ta' filters to TABLE only",
			want:   []string{"TABLE"},
			noWant: []string{"VIEW", "INDEX"},
		},
		{
			sql:  "DROP ",
			desc: "all DDL object types after DROP",
			want: []string{"TABLE", "VIEW", "INDEX"},
		},
	}

	for _, tc := range cases {
		names := symbolNames(e.Suggest(tc.sql, len(tc.sql), cfg()))
		for _, w := range tc.want {
			if !slices.Contains(names, w) {
				t.Errorf("[%s] expected %q: %v", tc.desc, w, names)
			}
		}
		for _, nw := range tc.noWant {
			if slices.Contains(names, nw) {
				t.Errorf("[%s] unexpected %q: %v", tc.desc, nw, names)
			}
		}
	}
}

func TestEngine_DDL_CreateObject(t *testing.T) {
	e := NewDefaultEngine()
	syms := e.Suggest("CREATE TABLE ", len("CREATE TABLE "), cfg())
	names := symbolNames(syms)
	for _, kw := range isql.DDLObjectKeywords {
		if slices.Contains(names, kw) {
			t.Errorf("DDL object keyword %q must not appear after CREATE TABLE: %v", kw, names)
		}
	}
	if slices.Contains(names, "public.users") {
		t.Errorf("existing tables must not appear in CtxCreateObject: %v", names)
	}
}

func TestEngine_DDL_ExistingObject(t *testing.T) {
	e := NewDefaultEngine()
	cases := []struct {
		sql    string
		desc   string
		want   []string
		noWant []string
	}{
		{
			sql:    "DROP TABLE us",
			desc:   "partial 'us' filters to users",
			want:   []string{"public.users"},
			noWant: []string{"public.orders"},
		},
		{
			sql:  "ALTER TABLE ",
			desc: "all tables listed after ALTER TABLE",
			want: []string{"public.users", "public.orders"},
		},
		{
			sql:  "TRUNCATE TABLE ",
			desc: "all tables listed after TRUNCATE TABLE",
			want: []string{"public.users", "public.orders"},
		},
	}

	for _, tc := range cases {
		names := symbolNames(e.Suggest(tc.sql, len(tc.sql), cfg()))
		for _, w := range tc.want {
			if !slices.Contains(names, w) {
				t.Errorf("[%s] expected %q: %v", tc.desc, w, names)
			}
		}
		for _, nw := range tc.noWant {
			if slices.Contains(names, nw) {
				t.Errorf("[%s] unexpected %q: %v", tc.desc, nw, names)
			}
		}
	}
}

func TestEngine_SELECT_ColumnStarstWithDigit(t *testing.T) {
	e := NewDefaultEngine()
	c := Context{
		Schemas: []database.Schema{{Schema: "auth", Tables: []string{"access_roles"}}},
		ColumnFetcher: func(_, _ string) ([]Column, error) {
			return []Column{
				{Name: "id"}, {Name: "name"}, {Name: "2fa"}, {Name: "isDefault"},
			}, nil
		},
		ColumnCache: make(map[string][]Column),
	}

	cases := []struct {
		sql       string
		cursorPos int
		want      string
		suppress  bool
	}{
		{
			sql:       "SELECT 2fa FROM auth.access_roles",
			cursorPos: len("SELECT 2fa"),
			want:      "2fa",
		},
		{
			sql:       "SELECT 2f FROM auth.access_roles",
			cursorPos: len("SELECT 2f"),
			want:      "2fa",
		},
		{
			sql:       "SELECT 2 FROM auth.access_roles",
			cursorPos: len("SELECT 2"),
			suppress:  true,
		},
	}

	for _, tc := range cases {
		names := symbolNames(e.Suggest(tc.sql, tc.cursorPos, c))
		if tc.suppress {
			if len(names) != 0 {
				t.Errorf("cursor at %d: expected suppressed, got %v", tc.cursorPos, names)
			}
			continue
		}
		if !slices.Contains(names, tc.want) {
			t.Errorf("cursor at %d: expected column %q in suggestions: %v", tc.cursorPos, tc.want, names)
		}
	}
}

// TestEngine_QuotedIdentifier_Columns covers issue #55: an unterminated opening
// quote before a column must still suggest columns (the following FROM is no
// longer swallowed), flag the symbol as Quoted, and extend the replace range
// back over the opening quote so acceptance emits a balanced identifier.
func TestEngine_QuotedIdentifier_Columns(t *testing.T) {
	e := NewDefaultEngine()
	c := cfgWithCols()

	cases := []struct {
		name     string
		sql      string
		want     string
		openQuot byte
	}{
		{"ansi", `SELECT "cr FROM users`, "created_at", '"'},
		{"backtick", "SELECT `cr FROM users", "created_at", '`'},
		{"bracket", `SELECT [cr FROM users`, "created_at", '['},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cursor := len("SELECT ") + 3 // just past the opening quote + "cr"
			syms := e.Suggest(tc.sql, cursor, c)

			var got *Symbol
			for i := range syms {
				if syms[i].Name == tc.want {
					got = &syms[i]
					break
				}
			}
			if got == nil {
				t.Fatalf("expected column %q in suggestions: %v", tc.want, symbolNames(syms))
			}
			if !got.Quoted {
				t.Errorf("Quoted=false, want true")
			}
			// Replace.Start must point at the opening quote so it is replaced too.
			if tc.sql[got.Replace.Start] != tc.openQuot {
				t.Errorf("Replace.Start points at %q, want opening quote %q",
					tc.sql[got.Replace.Start], tc.openQuot)
			}
			if got.Replace.End != cursor {
				t.Errorf("Replace.End=%d, want %d", got.Replace.End, cursor)
			}
		})
	}
}

func TestBuildScope(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		cursor   int // byte offset; 0 means end of text
		wantRefs []TableRef
		wantCTEs []string
	}{
		{
			name:     "simple from",
			sql:      "SELECT * FROM users",
			wantRefs: []TableRef{{Table: "users"}},
		},
		{
			name:     "schema qualified",
			sql:      "SELECT * FROM public.users",
			wantRefs: []TableRef{{Schema: "public", Table: "users"}},
		},
		{
			name:     "schema qualified with alias",
			sql:      "SELECT * FROM public.users u",
			wantRefs: []TableRef{{Schema: "public", Table: "users", Alias: "u"}},
		},
		{
			name:     "quoted schema qualified strips quotes",
			sql:      `SELECT * FROM "audit"."events"`,
			wantRefs: []TableRef{{Schema: "audit", Table: "events"}},
		},
		{
			name:     "quoted table with quoted alias strips quotes",
			sql:      `SELECT * FROM "users" AS "u"`,
			wantRefs: []TableRef{{Table: "users", Alias: "u"}},
		},
		{
			name:     "alias with AS",
			sql:      "SELECT * FROM users AS u",
			wantRefs: []TableRef{{Table: "users", Alias: "u"}},
		},
		{
			name:     "implicit alias",
			sql:      "SELECT * FROM users u",
			wantRefs: []TableRef{{Table: "users", Alias: "u"}},
		},
		{
			name:     "join clause",
			sql:      "SELECT * FROM users u JOIN orders o ON u.id = o.user_id",
			wantRefs: []TableRef{{Table: "users", Alias: "u"}, {Table: "orders", Alias: "o"}},
		},
		{
			name:     "left join keyword is not an alias",
			sql:      "SELECT * FROM a LEFT JOIN b ON a.id = b.a_id",
			wantRefs: []TableRef{{Table: "a"}, {Table: "b"}},
		},
		{
			name:     "comma list only captures first table",
			sql:      "SELECT * FROM a, b",
			wantRefs: []TableRef{{Table: "a"}},
		},
		{
			name:     "cte names",
			sql:      "WITH cte AS (SELECT 1) SELECT * FROM cte",
			wantRefs: []TableRef{{Table: "cte"}},
			wantCTEs: []string{"cte"},
		},
		{
			name:     "multi cte",
			sql:      "WITH a AS (SELECT 1), b AS (SELECT 2) SELECT * FROM a JOIN b ON true",
			wantRefs: []TableRef{{Table: "a"}, {Table: "b"}},
			wantCTEs: []string{"a", "b"},
		},
		{
			name:     "with recursive collects cte name and body tables",
			sql:      "WITH RECURSIVE tree AS (SELECT id FROM nodes WHERE parent IS NULL UNION ALL SELECT n.id FROM nodes n JOIN tree t ON n.parent = t.id) SELECT * FROM tree",
			wantRefs: []TableRef{{Table: "nodes"}, {Table: "nodes", Alias: "n"}, {Table: "tree", Alias: "t"}, {Table: "tree"}},
			wantCTEs: []string{"tree"},
		},
		{
			name:     "subquery in from is not promoted",
			sql:      "SELECT * FROM (SELECT id FROM inner_table) sub",
			wantRefs: nil,
		},
		{
			name:     "cursor at end takes last statement",
			sql:      "SELECT * FROM a; SELECT * FROM b",
			wantRefs: []TableRef{{Table: "b"}},
		},
		{
			name:     "cursor in first statement takes first statement",
			sql:      "SELECT * FROM a; SELECT * FROM b",
			cursor:   len("SELECT * FROM a"),
			wantRefs: []TableRef{{Table: "a"}},
		},
		{
			name:     "cursor in middle statement",
			sql:      "SELECT 1; SELECT * FROM mid; SELECT 2",
			cursor:   len("SELECT 1; SELECT * FROM mid"),
			wantRefs: []TableRef{{Table: "mid"}},
		},
		{
			name:     "cursor before trailing semicolon",
			sql:      "SELECT  FROM auth.roles;",
			cursor:   len("SELECT "),
			wantRefs: []TableRef{{Schema: "auth", Table: "roles"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cursor := tc.cursor
			if cursor == 0 {
				cursor = len(tc.sql)
			}
			scope := BuildScope(isql.Tokenize(tc.sql), cursor)
			if !reflect.DeepEqual(scope.TableRefs, tc.wantRefs) {
				t.Errorf("TableRefs = %+v, want %+v", scope.TableRefs, tc.wantRefs)
			}
			if !reflect.DeepEqual(scope.CTENames, tc.wantCTEs) {
				t.Errorf("CTENames = %v, want %v", scope.CTENames, tc.wantCTEs)
			}
		})
	}
}
