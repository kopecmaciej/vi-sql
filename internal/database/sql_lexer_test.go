package database

import (
	"testing"
)

func TestTokenize_Basic(t *testing.T) {
	tokens := Tokenize("SELECT * FROM users")
	wantTypes := []TokenType{
		TokenKeyword,     // SELECT
		TokenWhitespace,  // " "
		TokenStar,        // *
		TokenWhitespace,  // " "
		TokenKeyword,     // FROM
		TokenWhitespace,  // " "
		TokenIdentifier,  // users
	}
	if len(tokens) != len(wantTypes) {
		t.Fatalf("got %d tokens, want %d; tokens: %v", len(tokens), len(wantTypes), tokens)
	}
	for i, tt := range wantTypes {
		if tokens[i].Type != tt {
			t.Errorf("token[%d]: got type %d (%q), want %d", i, tokens[i].Type, tokens[i].Value, tt)
		}
	}
}

func TestTokenize_StringLiteral(t *testing.T) {
	tests := []struct {
		sql      string
		wantVal  string
		wantType TokenType
	}{
		{"'hello'", "'hello'", TokenString},
		{"'it''s'", "'it''s'", TokenString},   // escaped quote
		{"'a''b''c'", "'a''b''c'", TokenString}, // multiple escaped quotes
	}
	for _, tc := range tests {
		tokens := Tokenize(tc.sql)
		if len(tokens) != 1 {
			t.Errorf("sql=%q: got %d tokens, want 1", tc.sql, len(tokens))
			continue
		}
		if tokens[0].Type != tc.wantType {
			t.Errorf("sql=%q: type=%d, want %d", tc.sql, tokens[0].Type, tc.wantType)
		}
		if tokens[0].Value != tc.wantVal {
			t.Errorf("sql=%q: value=%q, want %q", tc.sql, tokens[0].Value, tc.wantVal)
		}
	}
}

func TestTokenize_QuotedIdentifier(t *testing.T) {
	tokens := Tokenize(`"My Table"`)
	if len(tokens) != 1 {
		t.Fatalf("got %d tokens, want 1", len(tokens))
	}
	if tokens[0].Type != TokenQuotedIdentifier {
		t.Errorf("type=%d, want TokenQuotedIdentifier", tokens[0].Type)
	}
	if tokens[0].Value != `"My Table"` {
		t.Errorf("value=%q", tokens[0].Value)
	}
}

func TestTokenize_LineComment(t *testing.T) {
	tokens := Tokenize("-- comment\nSELECT")
	if len(tokens) != 3 {
		t.Fatalf("got %d tokens, want 3: %v", len(tokens), tokens)
	}
	if tokens[0].Type != TokenComment {
		t.Errorf("token[0] type=%d, want TokenComment", tokens[0].Type)
	}
	if tokens[1].Type != TokenWhitespace {
		t.Errorf("token[1] type=%d, want TokenWhitespace", tokens[1].Type)
	}
	if tokens[2].Type != TokenKeyword || tokens[2].Value != "SELECT" {
		t.Errorf("token[2]=%v", tokens[2])
	}
}

func TestTokenize_BlockComment(t *testing.T) {
	tokens := Tokenize("/* block */SELECT")
	if len(tokens) != 2 {
		t.Fatalf("got %d tokens, want 2: %v", len(tokens), tokens)
	}
	if tokens[0].Type != TokenComment {
		t.Errorf("token[0] type=%d, want TokenComment", tokens[0].Type)
	}
	if tokens[1].Type != TokenKeyword {
		t.Errorf("token[1] type=%d, want TokenKeyword", tokens[1].Type)
	}
}

func TestTokenize_Typecast(t *testing.T) {
	tokens := Tokenize("val::text")
	wantTypes := []TokenType{TokenIdentifier, TokenTypecast, TokenIdentifier}
	if len(tokens) != len(wantTypes) {
		t.Fatalf("got %d tokens: %v", len(tokens), tokens)
	}
	for i, tt := range wantTypes {
		if tokens[i].Type != tt {
			t.Errorf("token[%d] type=%d, want %d", i, tokens[i].Type, tt)
		}
	}
}

func TestTokenize_Dot(t *testing.T) {
	tokens := Tokenize("public.users")
	wantTypes := []TokenType{TokenIdentifier, TokenDot, TokenIdentifier}
	if len(tokens) != len(wantTypes) {
		t.Fatalf("got %d tokens: %v", len(tokens), tokens)
	}
	for i, tt := range wantTypes {
		if tokens[i].Type != tt {
			t.Errorf("token[%d] type=%d (%q), want %d", i, tokens[i].Type, tokens[i].Value, tt)
		}
	}
}

func TestTokenize_Operators(t *testing.T) {
	cases := []struct {
		sql string
		val string
	}{
		{"!=", "!="},
		{"<>", "<>"},
		{"<=", "<="},
		{">=", ">="},
		{"=", "="},
		{"<", "<"},
		{">", ">"},
	}
	for _, tc := range cases {
		tokens := Tokenize(tc.sql)
		if len(tokens) != 1 || tokens[0].Type != TokenOperator || tokens[0].Value != tc.val {
			t.Errorf("sql=%q: got %v", tc.sql, tokens)
		}
	}
}

func TestTokenize_Number(t *testing.T) {
	cases := []string{"42", "3.14", "1e10"}
	for _, sql := range cases {
		tokens := Tokenize(sql)
		if len(tokens) != 1 || tokens[0].Type != TokenNumber {
			t.Errorf("sql=%q: got %v", sql, tokens)
		}
	}
}

func TestTokenize_Parameter(t *testing.T) {
	tokens := Tokenize("$1")
	if len(tokens) != 1 || tokens[0].Type != TokenIdentifier || tokens[0].Value != "$1" {
		t.Errorf("got %v", tokens)
	}
}

func TestTokenize_KeywordCaseInsensitive(t *testing.T) {
	for _, kw := range []string{"select", "SELECT", "Select"} {
		tokens := Tokenize(kw)
		if len(tokens) != 1 || tokens[0].Type != TokenKeyword {
			t.Errorf("kw=%q: got type %d", kw, tokens[0].Type)
		}
	}
}

func TestTokenize_BytePositions(t *testing.T) {
	sql := "SELECT id"
	tokens := Tokenize(sql)
	// SELECT: 0..6, space: 6..7, id: 7..9
	cases := []struct{ start, end int }{{0, 6}, {6, 7}, {7, 9}}
	if len(tokens) != 3 {
		t.Fatalf("got %d tokens", len(tokens))
	}
	for i, c := range cases {
		if tokens[i].Start != c.start || tokens[i].End != c.end {
			t.Errorf("token[%d]: got [%d,%d), want [%d,%d)", i, tokens[i].Start, tokens[i].End, c.start, c.end)
		}
	}
}

func TestTokenize_ComplexQuery(t *testing.T) {
	sql := `SELECT u.id, u.name FROM "public".users u WHERE u.age > 18 -- filter`
	tokens := Tokenize(sql)
	// Sanity: no panics, all bytes covered, start/end monotonic
	for i := 1; i < len(tokens); i++ {
		if tokens[i].Start != tokens[i-1].End {
			t.Errorf("gap between token[%d].End=%d and token[%d].Start=%d",
				i-1, tokens[i-1].End, i, tokens[i].Start)
		}
	}
	if len(tokens) > 0 && tokens[len(tokens)-1].End != len(sql) {
		t.Errorf("last token ends at %d, want %d", tokens[len(tokens)-1].End, len(sql))
	}
}
