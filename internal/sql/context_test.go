package sql

import (
	"testing"
)

// ctx is a shorthand to run DetectContext on raw SQL with the cursor at the end.
func ctx(sql string) CompletionContext {
	tokens := Tokenize(sql)
	return DetectContext(tokens, len(sql))
}

// ctxAt runs DetectContext with the cursor at a specific byte offset.
func ctxAt(sql string, pos int) CompletionContext {
	tokens := Tokenize(sql)
	return DetectContext(tokens, pos)
}

func TestDetectContext_Empty(t *testing.T) {
	c := ctx("")
	if c.Type != CtxStatementStart {
		t.Errorf("empty: got %d, want CtxStatementStart", c.Type)
	}
}

func TestDetectContext_AfterSelect(t *testing.T) {
	c := ctx("SELECT ")
	if c.Type != CtxAfterSelect {
		t.Errorf("got %d, want CtxAfterSelect", c.Type)
	}
}

func TestDetectContext_AfterFrom(t *testing.T) {
	c := ctx("SELECT * FROM ")
	if c.Type != CtxAfterFrom {
		t.Errorf("got %d, want CtxAfterFrom", c.Type)
	}
}

func TestDetectContext_AfterJoin(t *testing.T) {
	for _, sql := range []string{
		"SELECT * FROM users JOIN ",
		"SELECT * FROM users LEFT JOIN ",
		"SELECT * FROM users INNER JOIN ",
	} {
		c := ctx(sql)
		if c.Type != CtxAfterJoin {
			t.Errorf("sql=%q: got %d, want CtxAfterJoin", sql, c.Type)
		}
	}
}

func TestDetectContext_AfterWhere(t *testing.T) {
	c := ctx("SELECT * FROM users WHERE ")
	if c.Type != CtxAfterWhere {
		t.Errorf("got %d, want CtxAfterWhere", c.Type)
	}
}

func TestDetectContext_AfterWhereExpression(t *testing.T) {
	c := ctx("SELECT * FROM users WHERE id = 1 AND ")
	if c.Type != CtxAfterWhere {
		t.Errorf("got %d, want CtxAfterWhere", c.Type)
	}
}

func TestDetectContext_AfterOn(t *testing.T) {
	c := ctx("SELECT * FROM a JOIN b ON ")
	if c.Type != CtxAfterOn {
		t.Errorf("got %d, want CtxAfterOn", c.Type)
	}
}

func TestDetectContext_AfterOrderBy(t *testing.T) {
	c := ctx("SELECT * FROM users ORDER BY ")
	if c.Type != CtxAfterOrderBy {
		t.Errorf("got %d, want CtxAfterOrderBy", c.Type)
	}
}

func TestDetectContext_AfterGroupBy(t *testing.T) {
	c := ctx("SELECT id, COUNT(*) FROM users GROUP BY ")
	if c.Type != CtxAfterGroupBy {
		t.Errorf("got %d, want CtxAfterGroupBy", c.Type)
	}
}

func TestDetectContext_AfterSet(t *testing.T) {
	c := ctx("UPDATE users SET ")
	if c.Type != CtxAfterSet {
		t.Errorf("got %d, want CtxAfterSet", c.Type)
	}
}

func TestDetectContext_AfterInto(t *testing.T) {
	c := ctx("INSERT INTO ")
	if c.Type != CtxAfterInto {
		t.Errorf("got %d, want CtxAfterInto", c.Type)
	}
}

func TestDetectContext_AfterValues(t *testing.T) {
	c := ctx("INSERT INTO t VALUES (")
	if c.Type != CtxAfterValues {
		t.Errorf("got %d, want CtxAfterValues", c.Type)
	}
}

func TestDetectContext_AfterDot_NoPartial(t *testing.T) {
	c := ctx("SELECT * FROM users.")
	if c.Type != CtxAfterDot {
		t.Errorf("got %d, want CtxAfterDot", c.Type)
	}
	if c.TableName != "users" {
		t.Errorf("TableName=%q, want %q", c.TableName, "users")
	}
	if c.PartialWord != "" {
		t.Errorf("PartialWord=%q, want empty", c.PartialWord)
	}
}

func TestDetectContext_AfterDot_WithPartial(t *testing.T) {
	sql := "SELECT users.na"
	c := ctx(sql)
	if c.Type != CtxAfterDot {
		t.Errorf("got %d, want CtxAfterDot", c.Type)
	}
	if c.TableName != "users" {
		t.Errorf("TableName=%q, want %q", c.TableName, "users")
	}
	if c.PartialWord != "na" {
		t.Errorf("PartialWord=%q, want %q", c.PartialWord, "na")
	}
}

func TestDetectContext_SchemaQualified(t *testing.T) {
	c := ctx("SELECT * FROM public.")
	if c.Type != CtxAfterDot {
		t.Errorf("got %d, want CtxAfterDot", c.Type)
	}
	if c.TableName != "public" {
		t.Errorf("TableName=%q, want %q", c.TableName, "public")
	}
}

func TestDetectContext_PartialKeyword(t *testing.T) {
	c := ctx("sel")
	if c.Type != CtxStatementStart {
		t.Errorf("got %d, want CtxStatementStart", c.Type)
	}
	if c.PartialWord != "sel" {
		t.Errorf("PartialWord=%q, want %q", c.PartialWord, "sel")
	}
}

func TestDetectContext_PartialInWhere(t *testing.T) {
	sql := "SELECT * FROM users WHERE id"
	c := ctx(sql)
	if c.Type != CtxAfterWhere {
		t.Errorf("got %d, want CtxAfterWhere", c.Type)
	}
	if c.PartialWord != "id" {
		t.Errorf("PartialWord=%q, want %q", c.PartialWord, "id")
	}
}

func TestDetectContext_CursorMidQuery(t *testing.T) {
	sql := "SELECT * FROM users WHERE id = 1"
	pos := len("SELECT * FROM ")
	c := ctxAt(sql, pos)
	if c.Type != CtxAfterFrom {
		t.Errorf("got %d, want CtxAfterFrom", c.Type)
	}
}

func TestDetectContext_DMLStatement(t *testing.T) {
	cases := []struct {
		sql  string
		want ContextType
	}{
		{"INSERT ", CtxStatementStart},
		{"UPDATE ", CtxStatementStart},
		{"DELETE ", CtxStatementStart},
		{"CREATE ", CtxAfterDDLVerb},
		{"DROP ", CtxAfterDDLVerb},
		{"ALTER ", CtxAfterDDLVerb},
		{"TRUNCATE ", CtxAfterDDLVerb},
	}
	for _, tc := range cases {
		c := ctx(tc.sql)
		if c.Type != tc.want {
			t.Errorf("sql=%q: got %d, want %d", tc.sql, c.Type, tc.want)
		}
	}
}

func TestDetectContext_AfterDDLVerb_WithPartial(t *testing.T) {
	c := ctx("CREATE ta")
	if c.Type != CtxAfterDDLVerb {
		t.Errorf("got %d, want CtxAfterDDLVerb", c.Type)
	}
	if c.PartialWord != "ta" {
		t.Errorf("PartialWord=%q, want %q", c.PartialWord, "ta")
	}
}

// TestDetectContext_AfterCreateObject verifies CREATE TABLE/VIEW/... → CtxCreateObject
// (new name expected; suppress existing-table suggestions).
func TestDetectContext_AfterCreateObject(t *testing.T) {
	cases := []string{
		"CREATE TABLE ",
		"CREATE VIEW ",
	}
	for _, sql := range cases {
		c := ctx(sql)
		if c.Type != CtxCreateObject {
			t.Errorf("sql=%q: got %d, want CtxCreateObject", sql, c.Type)
		}
	}
}

// TestDetectContext_AfterExistingObject verifies DROP/ALTER/TRUNCATE TABLE/... → CtxExistingObject
// (existing name expected). Note: "TRUNCATE " alone (without an object keyword) stays CtxAfterDDLVerb.
func TestDetectContext_AfterExistingObject(t *testing.T) {
	cases := []string{
		"DROP TABLE ",
		"ALTER TABLE ",
		"TRUNCATE TABLE ",
		"DROP INDEX ",
	}
	for _, sql := range cases {
		c := ctx(sql)
		if c.Type != CtxExistingObject {
			t.Errorf("sql=%q: got %d, want CtxExistingObject", sql, c.Type)
		}
	}
}

func TestDetectContext_AfterDDLObject_WithPartial(t *testing.T) {
	c := ctx("DROP TABLE us")
	if c.Type != CtxAfterDDLObject {
		t.Errorf("got %d, want CtxAfterDDLObject (= CtxExistingObject)", c.Type)
	}
	if c.PartialWord != "us" {
		t.Errorf("PartialWord=%q, want %q", c.PartialWord, "us")
	}
}

func TestDetectContext_InsideParen_DDL(t *testing.T) {
	cases := []string{
		"CREATE TABLE shipping.companies (",
		"CREATE TABLE foo (id INT, name ",
		"ALTER TABLE foo ADD COLUMN (",
	}
	for _, sql := range cases {
		c := ctx(sql)
		if c.Type != CtxGeneral {
			t.Errorf("sql=%q: got %d, want CtxGeneral", sql, c.Type)
		}
	}
}

func TestDetectContext_InsideParen_InList(t *testing.T) {
	c := ctx("SELECT * FROM users WHERE id IN (")
	if c.Type != CtxStatementStart {
		t.Errorf("got %d, want CtxStatementStart", c.Type)
	}
}

func TestDetectContext_InsideParen_SubqueryKeywordNotLeaking(t *testing.T) {
	c := ctx("SELECT * FROM users WHERE id IN (SELECT id FROM orders) AND ")
	if c.Type != CtxAfterWhere {
		t.Errorf("got %d, want CtxAfterWhere", c.Type)
	}
}

// TestDetectContext_HavingClause verifies HAVING gets its own context type
// (distinct from CtxAfterWhere) so providers can tailor suggestions.
func TestDetectContext_HavingClause(t *testing.T) {
	c := ctx("SELECT id, COUNT(*) FROM users GROUP BY id HAVING ")
	if c.Type != CtxAfterHaving {
		t.Errorf("got %d, want CtxAfterHaving", c.Type)
	}
}

func TestShouldSuppressCompletion(t *testing.T) {
	cases := []struct {
		sql  string
		want bool
	}{
		// number cases
		{"SELECT * FROM t WHERE id = 1", true},   // cursor right after number
		{"SELECT * FROM t WHERE id = 1 ", false}, // space after number
		{"SELECT * FROM t WHERE id = 123", true}, // multi-digit number
		{"SELECT * FROM t WHERE id = 1a", true},  // letter after number (partial word, preceded by number)

		// string literal cases
		{"WHERE name = 'hell", true},     // inside unclosed string
		{"WHERE name = '", true},         // cursor right after opening quote
		{"WHERE name = 'hello' ", false}, // after closed string + space

		// ! prefix case
		{"WHERE id !", true},  // after bare !
		{"WHERE id !x", true}, // typing after !

		// operator cases: suppress directly after operator, not after operator+space
		{"WHERE id =", true},    // cursor right after =
		{"WHERE id >=", true},   // cursor right after >=
		{"WHERE id !=", true},   // cursor right after !=
		{"WHERE id <>", true},   // cursor right after <>
		{"WHERE id = ", false},  // = then space
		{"WHERE id >= ", false}, // >= then space
		{"WHERE id != ", false}, // != then space

		// normal cases that should NOT suppress
		{"SELECT * FROM ", false},
		{"SELECT * FROM t WHERE ", false},
		{"SELECT col", false}, // partial identifier
		{"", false},
	}
	for _, tc := range cases {
		got := SuppressRaw(tc.sql, len(tc.sql))
		if got != tc.want {
			t.Errorf("suppress(%q) = %v, want %v", tc.sql, got, tc.want)
		}
	}
}
