package component

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// parseCursor splits "text with |cursor" into (text, byteOffset).
func parseCursor(s string) (text string, pos int) {
	pos = strings.IndexByte(s, '|')
	if pos < 0 {
		return s, 0
	}
	return s[:pos] + s[pos+1:], pos
}

// withCursor inserts | at byteOffset pos in text.
func withCursor(text string, pos int) string {
	return text[:pos] + "|" + text[pos:]
}

func TestWordForward(t *testing.T) {
	tests := []struct {
		in   string
		big  bool
		want string
	}{
		{in: "|SELECT * FROM something;", want: "SELECT |* FROM something;"},
		{in: "SELECT |* FROM something;", want: "SELECT * |FROM something;"},
		{in: "SELECT * FROM |something;", want: "SELECT * FROM something|;"},
		{in: "|SELECT * FROM something;", big: true, want: "SELECT |* FROM something;"},
		{in: "SELECT |* FROM something;", big: true, want: "SELECT * |FROM something;"},
		{in: "SELECT * FROM |something;", big: true, want: "SELECT * FROM something;|"},

		{in: "|foo   bar", want: "foo   |bar"},
		{in: "|foo   bar", big: true, want: "foo   |bar"},
		// empty line is a stop
		{in: "|foo\n\nbar", want: "foo\n|\nbar"},
		{in: "foo\n|\nbar", want: "foo\n\n|bar"},
		// punctuation runs
		{in: "foo|===bar", want: "foo===|bar"},
		{in: "|((foo))", want: "((|foo))"},
		{in: "((|foo))", want: "((foo|))"},
		// SQL-flavoured strings
		{in: "WHERE |id=42", want: "WHERE id|=42"},
		{in: "WHERE id|=42", want: "WHERE id=|42"},
		{in: "|a, b, c", want: "a|, b, c"},
		{in: "a|, b, c", want: "a, |b, c"},
		{in: "foo|", want: "foo|"},
		{in: "|héllo wörld", want: "héllo |wörld"},
	}

	for _, tt := range tests {
		t.Run(tt.in+"(big="+boolStr(tt.big)+")", func(t *testing.T) {
			text, pos := parseCursor(tt.in)
			wantText, wantPos := parseCursor(tt.want)
			if text != wantText {
				t.Fatalf("parseCursor mismatch: text %q != want %q", text, wantText)
			}
			got := wordForward(text, pos, tt.big)
			if got != wantPos {
				t.Errorf("wordForward(%q, %d, %v) = %d, want %d\n  got cursor:  %q\n  want cursor: %q",
					text, pos, tt.big, got, wantPos, withCursor(text, got), tt.want)
			}
		})
	}
}

func TestWordForwardEnd(t *testing.T) {
	tests := []struct {
		in   string
		big  bool
		want string
	}{
		{in: "|SELECT * FROM something;", want: "SELEC|T * FROM something;"},
		{in: "SELECT| * FROM something;", want: "SELECT |* FROM something;"},
		{in: "SELECT |* FROM something;", want: "SELECT * FRO|M something;"},
		{in: "SELECT * |FROM something;", want: "SELECT * FRO|M something;"},
		{in: "SELECT * FROM |something;", want: "SELECT * FROM somethin|g;"},
		{in: "SELECT * FROM something|;", want: "SELECT * FROM something|;"},

		{in: "==|=foo", want: "===fo|o"},
		{in: "|((foo))", want: "(|(foo))"},
		{in: "|foo   bar", want: "fo|o   bar"},
		{in: "foo   |bar", want: "foo   ba|r"},
		{in: "|WHERE id=42", want: "WHER|E id=42"},
		{in: "WHERE| id=42", want: "WHERE i|d=42"},
	}

	for _, tt := range tests {
		t.Run(tt.in+"(big="+boolStr(tt.big)+")", func(t *testing.T) {
			text, pos := parseCursor(tt.in)
			wantText, wantPos := parseCursor(tt.want)
			if text != wantText {
				t.Fatalf("parseCursor mismatch")
			}
			got := wordForwardEnd(text, pos, tt.big)
			if got != wantPos {
				t.Errorf("wordForwardEnd(%q, %d, %v) = %d, want %d\n  got cursor:  %q\n  want cursor: %q",
					text, pos, tt.big, got, wantPos, withCursor(text, got), tt.want)
			}
		})
	}
}

func TestWordBackward(t *testing.T) {
	tests := []struct {
		in   string
		big  bool
		want string
	}{
		{in: "SELECT * FROM something|;", want: "SELECT * FROM |something;"},
		{in: "SELECT * FROM |something;", want: "SELECT * |FROM something;"},
		{in: "SELECT * |FROM something;", want: "SELECT |* FROM something;"},
		{in: "SELECT |* FROM something;", want: "|SELECT * FROM something;"},
		{in: "foo==|bar", want: "foo|==bar"},
		{in: "foo|==bar", want: "|foo==bar"},
		{in: "SELECT * FROM something|;", big: true, want: "SELECT * FROM |something;"},
		{in: "|foo bar", want: "|foo bar"},
		{in: "WHERE id=|42", want: "WHERE id|=42"},
		{in: "WHERE id|=42", want: "WHERE |id=42"},
		{in: "héllo |wörld", want: "|héllo wörld"},
	}

	for _, tt := range tests {
		t.Run(tt.in+"(big="+boolStr(tt.big)+")", func(t *testing.T) {
			text, pos := parseCursor(tt.in)
			wantText, wantPos := parseCursor(tt.want)
			if text != wantText {
				t.Fatalf("parseCursor mismatch")
			}
			got := wordBackward(text, pos, tt.big)
			if got != wantPos {
				t.Errorf("wordBackward(%q, %d, %v) = %d, want %d\n  got cursor:  %q\n  want cursor: %q",
					text, pos, tt.big, got, wantPos, withCursor(text, got), tt.want)
			}
		})
	}
}

func TestWordBackwardEnd(t *testing.T) {
	tests := []struct {
		in   string
		big  bool
		want string
	}{
		{in: "SELECT * FROM |something;", want: "SELECT * FRO|M something;"},
		{in: "SELECT * |FROM something;", want: "SELECT |* FROM something;"},
		{in: "SELECT |* FROM something;", want: "SELEC|T * FROM something;"},

		{in: "SELECT * FROM |something;", big: true, want: "SELECT * FRO|M something;"},
		{in: "foo |bar", big: true, want: "fo|o bar"},
		{in: "foo===|bar", want: "foo==|=bar"},
		{in: "foo |===bar", want: "fo|o ===bar"},
		{in: "|foo", want: "|foo"},
	}

	for _, tt := range tests {
		t.Run(tt.in+"(big="+boolStr(tt.big)+")", func(t *testing.T) {
			text, pos := parseCursor(tt.in)
			wantText, wantPos := parseCursor(tt.want)
			if text != wantText {
				t.Fatalf("parseCursor mismatch: text=%q wantText=%q", text, wantText)
			}
			got := wordBackwardEnd(text, pos, tt.big)
			if got != wantPos {
				t.Errorf("wordBackwardEnd(%q, %d, %v) = %d, want %d\n  got cursor:  %q\n  want cursor: %q",
					text, pos, tt.big, got, wantPos, withCursor(text, got), tt.want)
			}
		})
	}
}

func TestWordForwardOperator(t *testing.T) {
	tests := []struct {
		in     string
		big    bool
		change bool
		want   string
	}{
		// dw: delete to next word start, clamp at newline
		{in: "|SELECT * FROM t;", want: "|* FROM t;"},
		{in: "SELECT |* FROM t;", want: "SELECT |FROM t;"},
		{in: "SELECT * |FROM t;", want: "SELECT * |t;"},

		// dw does NOT pull next line
		{in: "|foo\nbar", want: "|\nbar"},

		// dW: WORD variant
		{in: "|SELECT * FROM t;", big: true, want: "|* FROM t;"},
		{in: "SELECT |* FROM t;", big: true, want: "SELECT |FROM t;"},

		// cw on non-blank: behaves like ce (don't eat trailing space)
		{in: "|SELECT * FROM t;", change: true, want: "| * FROM t;"},
		// cw from * (1-char punct word at end): ce → e lands at next word end (M)
		{in: "SELECT |* FROM t;", change: true, want: "SELECT | t;"},
		// cw on blank: eats through blanks to next word (like dw)
		{in: "SELECT |  foo", change: true, want: "SELECT |foo"},
	}

	for _, tt := range tests {
		t.Run(tt.in+"(big="+boolStr(tt.big)+",change="+boolStr(tt.change)+")", func(t *testing.T) {
			text, pos := parseCursor(tt.in)
			wantText, _ := parseCursor(tt.want)
			_ = wantText

			start, end := operatorWordRange(text, pos, tt.big, tt.change)
			got := text[:start] + text[end:]
			gotWithCursor := withCursor(got, start)

			if gotWithCursor != tt.want {
				t.Errorf("wordForwardOperator(%q, %d, big=%v, change=%v)\n  got:  %q\n  want: %q",
					text, pos, tt.big, tt.change, gotWithCursor, tt.want)
			}
		})
	}
}

// operatorWordRange mirrors the dispatch layer's w/W handling under an operator:
// dw/yw clamp at the newline; cw/cW on a non-blank fold to ce/cE.
func operatorWordRange(text string, pos int, big, change bool) (int, int) {
	key := 'w'
	if big {
		key = 'W'
	}
	m := motions[key]
	if change {
		if r, _ := utf8.DecodeRuneInString(text[pos:]); pos < len(text) && runeClass(r, big) != 0 {
			if big {
				m = motions['E']
			} else {
				m = motions['e']
			}
		}
	} else {
		m = clampWordMotion(m)
	}
	dest := m.run(text, pos, 0)
	return rangeFor(text, pos, dest, m.kind)
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// buildOperatorMotion resolves a test motion key into a motion + its char, the
// same way the dispatch layer does (doubled op = linewise current line; gg;
// f/F/t/T find with a target rune; single-key table motion).
func buildOperatorMotion(key string, op rune) (motion, rune) {
	switch {
	case key == string(op)+string(op): // dd / cc / yy
		return motion{kind: mLinewise, run: lineDownStart}, 0
	case key == "gg":
		return gMotions['g'], 'g'
	case len(key) == 2 && strings.ContainsRune("fFtT", rune(key[0])):
		p, target := rune(key[0]), rune(key[1])
		return findMotion(target, p == 'f' || p == 't', p == 't' || p == 'T'), p
	default:
		ch := rune(key[0])
		return motions[ch], ch
	}
}

func TestOperatorRanges(t *testing.T) {
	tests := []struct {
		name     string
		inBuffer string
		operator rune
		key      string
		count    int
		want     string
	}{
		{"de inclusive at EOF", "|ab", 'd', "e", 0, ""},
		{"dw stops before newline", "|foo\nbar", 'd', "w", 0, "\nbar"},
		{"dw last word of line", "foo |bar\nbaz", 'd', "w", 0, "foo \nbaz"},
		{"dd last line eats preceding nl", "a\nb\n|c", 'd', "dd", 1, "a\nb"},
		{"dd only line", "|hello", 'd', "dd", 1, ""},
		{"2dd two lines", "|a\nb\nc", 'd', "dd", 2, "c"},
		{"99w clamps no panic", "|hi", 'd', "w", 99, ""},
		{"d3e three word ends", "|one two three", 'd', "e", 3, ""},
		{"dG to last line", "|a\nb\nc", 'd', "G", 0, ""},
		{"dgg to top", "a\nb\n|c", 'd', "gg", 0, ""},
		{"dfx inclusive find", "a|bcxd", 'd', "fx", 0, "ad"},
		{"dtx till find", "a|bcxd", 'd', "tx", 0, "axd"},
		{"cw on non-blank folds to ce", "|foo bar", 'c', "w", 0, " bar"},
		{"cc keeps the line", "a\n|bbb\nc", 'c', "cc", 1, "a\n\nc"},
		{"de multibyte word", "|héllo", 'd', "e", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, pos := parseCursor(tt.inBuffer)
			m, ch := buildOperatorMotion(tt.key, tt.operator)
			m = operatorMotion(tt.operator, ch, m, text, pos)
			dest := m.run(text, pos, tt.count)
			start, end := operatorRange(text, pos, dest, tt.operator, m.kind)
			got := text[:start] + text[end:]
			if got != tt.want {
				t.Errorf("%c%s (count=%d) on %q\n  got:  %q\n  want: %q",
					tt.operator, tt.key, tt.count, text, got, tt.want)
			}
		})
	}
}

func TestCombineCount(t *testing.T) {
	tests := []struct {
		exKey      string
		a, b, want int
	}{
		{"none", 0, 0, 0},
		{"d3w", 0, 3, 3},
		{"2dw", 2, 0, 2},
		{"2d2e", 2, 3, 6},
	}
	for _, tt := range tests {
		if got := combineCount(tt.a, tt.b); got != tt.want {
			t.Errorf("example key: %s for combineCount(%d,%d) = %d, want %d", tt.exKey, tt.a, tt.b, got, tt.want)
		}
	}
}
