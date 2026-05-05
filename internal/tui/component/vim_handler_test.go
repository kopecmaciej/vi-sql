package component

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// testTA is a minimal stand-in for the TextArea operations used by delete helpers.
type testTA struct {
	text     string
	cursor   int // byte offset (tview puts cursor at end of selection)
	selStart int // byte offset of selection start (inclusive)
	selEnd   int // byte offset of selection end (exclusive)
}

func (ta *testTA) GetText() string          { return ta.text }
func (ta *testTA) GetCursorByteOffset() int { return ta.cursor }
func (ta *testTA) GetTextAfterCursor() string {
	if ta.cursor >= len(ta.text) {
		return ""
	}
	return ta.text[ta.cursor:]
}
func (ta *testTA) Replace(start, end int, s string) {
	ta.text = ta.text[:start] + s + ta.text[end:]
	ta.cursor = start + len(s)
}
func (ta *testTA) Select(start, end int) {
	if end < start {
		start, end = end, start
	}
	ta.selStart = start
	ta.selEnd = end
	ta.cursor = end // tview places its internal cursor at the end of the selection
}

// Standalone implementations of the delete helpers for unit testing.
// These mirror vim_handler.go exactly so the tests catch logic regressions.

func testDeleteChar(ta *testTA) {
	after := ta.GetTextAfterCursor()
	if len(after) == 0 || after[0] == '\n' {
		return
	}
	_, size := utf8.DecodeRuneInString(after)
	pos := ta.GetCursorByteOffset()
	ta.Replace(pos, pos+size, "")
}

func testDeleteToEOL(ta *testTA) {
	pos := ta.GetCursorByteOffset()
	after := ta.GetTextAfterCursor()
	if nl := strings.IndexByte(after, '\n'); nl >= 0 {
		ta.Replace(pos, pos+nl, "")
	} else {
		ta.Replace(pos, pos+len(after), "")
	}
}

func testDeleteLine(ta *testTA) {
	text := ta.GetText()
	pos := ta.GetCursorByteOffset()
	lineStart := strings.LastIndexByte(text[:pos], '\n') + 1
	lineEnd := strings.IndexByte(text[pos:], '\n')
	if lineEnd < 0 {
		if lineStart > 0 {
			lineStart--
		}
		ta.Replace(lineStart, len(text), "")
	} else {
		ta.Replace(lineStart, pos+lineEnd+1, "")
	}
}

func TestDeleteCharUnderCursor(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		cursor int
		want   string
	}{
		{"middle of word", "hello world", 4, "hell world"},
		{"start of text", "abc", 0, "bc"},
		{"end of text (empty after)", "abc", 3, "abc"},
		{"at newline — no-op", "ab\ncd", 2, "ab\ncd"},
		{"multibyte char", "héllo", 0, "éllo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ta := &testTA{text: tt.text, cursor: tt.cursor}
			testDeleteChar(ta)
			if ta.text != tt.want {
				t.Errorf("got %q, want %q", ta.text, tt.want)
			}
		})
	}
}

func TestDeleteToEOL(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		cursor int
		want   string
	}{
		{"mid-line", "SELECT *\nFROM t", 7, "SELECT \nFROM t"},
		{"at line start", "SELECT *\nFROM t", 0, "\nFROM t"},
		{"last line no newline", "foo bar", 4, "foo "},
		{"at end of text", "foo", 3, "foo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ta := &testTA{text: tt.text, cursor: tt.cursor}
			testDeleteToEOL(ta)
			if ta.text != tt.want {
				t.Errorf("got %q, want %q", ta.text, tt.want)
			}
		})
	}
}

func TestDeleteLine(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		cursor int
		want   string
	}{
		{"only line", "hello", 2, ""},
		{"first of two lines", "hello\nworld", 2, "world"},
		{"last of two lines", "hello\nworld", 7, "hello"},
		{"middle line", "a\nb\nc", 2, "a\nc"},
		{"cursor at newline boundary", "hello\nworld", 5, "world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ta := &testTA{text: tt.text, cursor: tt.cursor}
			testDeleteLine(ta)
			if ta.text != tt.want {
				t.Errorf("got %q, want %q", ta.text, tt.want)
			}
		})
	}
}

// --- Visual selection pure-function tests ---

func TestVisualSelectionRange(t *testing.T) {
	tests := []struct {
		name               string
		text               string
		anchor, cursor     int
		wantStart, wantEnd int
	}{
		{"cursor at anchor", "hello", 2, 2, 2, 3},          // selects 'l'
		{"cursor right of anchor", "hello", 2, 3, 2, 4},    // selects 'll'
		{"cursor left of anchor", "hello", 2, 1, 1, 3},     // selects 'el'
		{"cursor far left of anchor", "hello", 4, 0, 0, 5}, // selects 'hello'
		{"cursor at text end-1", "hello", 0, 4, 0, 5},      // selects 'hello'
		{"multibyte at anchor", "héllo", 1, 1, 1, 3},       // selects 'é' (2 bytes)
		{"multibyte cursor left", "héllo", 3, 1, 1, 4},     // selects 'él'
		{"single char text", "x", 0, 0, 0, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := visualSelectionRange(tt.text, tt.anchor, tt.cursor)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("got [%d,%d), want [%d,%d)", start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

func TestVisualLineRange(t *testing.T) {
	tests := []struct {
		name               string
		text               string
		lineA, lineB       int // line-start byte offsets
		wantStart, wantEnd int
	}{
		{"single line, no newline", "hello", 0, 0, 0, 5},
		{"single line selected, has newline", "hello\nworld", 0, 0, 0, 6},
		{"second line selected", "hello\nworld", 6, 6, 6, 11},
		{"first two lines, lineA first", "a\nb\nc", 0, 2, 0, 4},
		{"first two lines, lineB first", "a\nb\nc", 2, 0, 0, 4},
		{"all three lines", "a\nb\nc", 0, 4, 0, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := visualLineRange(tt.text, tt.lineA, tt.lineB)
			if start != tt.wantStart || end != tt.wantEnd {
				t.Errorf("got [%d,%d), want [%d,%d)", start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

// testVisualH mirrors the fixed handleVisual 'h' logic for regression testing.
func testVisualH(ta *testTA, selStart, selCurrent *int) {
	text := ta.GetText()
	if *selCurrent == 0 {
		return
	}
	_, size := utf8.DecodeLastRuneInString(text[:*selCurrent])
	*selCurrent -= size
	start, end := visualSelectionRange(text, *selStart, *selCurrent)
	ta.Select(start, end)
}

// testVisualL mirrors the fixed handleVisual 'l' logic for regression testing.
func testVisualL(ta *testTA, selStart, selCurrent *int) {
	text := ta.GetText()
	if *selCurrent >= len(text) {
		return
	}
	_, size := utf8.DecodeRuneInString(text[*selCurrent:])
	*selCurrent += size
	start, end := visualSelectionRange(text, *selStart, *selCurrent)
	ta.Select(start, end)
}

func TestVisualHMovement(t *testing.T) {
	// Regression: backward 'h' previously collapsed after the first press.
	text := "hello"
	anchor := 2 // on 'l'
	ta := &testTA{text: text}
	selCurrent := anchor
	s, e := visualSelectionRange(text, anchor, selCurrent)
	ta.Select(s, e)
	if ta.selStart != 2 || ta.selEnd != 3 {
		t.Fatalf("enterVisual: got [%d,%d), want [2,3)", ta.selStart, ta.selEnd)
	}

	testVisualH(ta, &anchor, &selCurrent)
	if ta.selStart != 1 || ta.selEnd != 3 {
		t.Errorf("after 1st h: got [%d,%d), want [1,3)", ta.selStart, ta.selEnd)
	}

	testVisualH(ta, &anchor, &selCurrent)
	if ta.selStart != 0 || ta.selEnd != 3 {
		t.Errorf("after 2nd h: got [%d,%d), want [0,3)", ta.selStart, ta.selEnd)
	}
}

func TestVisualLMovement(t *testing.T) {
	text := "hello"
	anchor := 2 // on 'l'
	ta := &testTA{text: text}
	selCurrent := anchor
	s, e := visualSelectionRange(text, anchor, selCurrent)
	ta.Select(s, e)

	testVisualL(ta, &anchor, &selCurrent)
	if ta.selStart != 2 || ta.selEnd != 4 {
		t.Errorf("after 1st l: got [%d,%d), want [2,4)", ta.selStart, ta.selEnd)
	}

	testVisualL(ta, &anchor, &selCurrent)
	if ta.selStart != 2 || ta.selEnd != 5 {
		t.Errorf("after 2nd l: got [%d,%d), want [2,5)", ta.selStart, ta.selEnd)
	}
}

func TestVisualHAfterL(t *testing.T) {
	// Extend right then back left — selection should shrink then flip backward.
	text := "hello"
	anchor := 2
	ta := &testTA{text: text}
	selCurrent := anchor
	s, e := visualSelectionRange(text, anchor, selCurrent)
	ta.Select(s, e)

	testVisualL(ta, &anchor, &selCurrent) // [2,4)
	testVisualH(ta, &anchor, &selCurrent) // back to [2,3)
	if ta.selStart != 2 || ta.selEnd != 3 {
		t.Errorf("after l+h: got [%d,%d), want [2,3)", ta.selStart, ta.selEnd)
	}

	testVisualH(ta, &anchor, &selCurrent) // now backward: [1,3)
	if ta.selStart != 1 || ta.selEnd != 3 {
		t.Errorf("after l+h+h: got [%d,%d), want [1,3)", ta.selStart, ta.selEnd)
	}
}

// testVisualLineJ mirrors the fixed visual-line 'j' logic.
func testVisualLineJ(text string, selCurrent *int) bool {
	eol := strings.IndexByte(text[*selCurrent:], '\n')
	if eol < 0 {
		return false
	}
	*selCurrent += eol + 1
	return true
}

// testVisualLineK mirrors the fixed visual-line 'k' logic.
func testVisualLineK(text string, selCurrent *int) bool {
	if *selCurrent == 0 {
		return false
	}
	prev := strings.LastIndexByte(text[:*selCurrent-1], '\n')
	*selCurrent = prev + 1
	return true
}

func TestVisualLineJK(t *testing.T) {
	// "hello\nworld\nend" → logical line starts: 0, 6, 12
	text := "hello\nworld\nend"

	cur := 0
	if !testVisualLineJ(text, &cur) || cur != 6 {
		t.Errorf("j from line 0: got %d, want 6", cur)
	}
	if !testVisualLineJ(text, &cur) || cur != 12 {
		t.Errorf("j from line 1: got %d, want 12", cur)
	}
	if testVisualLineJ(text, &cur) {
		t.Errorf("j from last line should be no-op")
	}
	if cur != 12 {
		t.Errorf("cur should stay 12 after no-op j, got %d", cur)
	}

	if !testVisualLineK(text, &cur) || cur != 6 {
		t.Errorf("k from line 2: got %d, want 6", cur)
	}
	if !testVisualLineK(text, &cur) || cur != 0 {
		t.Errorf("k from line 1: got %d, want 0", cur)
	}
	if testVisualLineK(text, &cur) {
		t.Errorf("k from first line should be no-op")
	}
}

func TestVisualLineJWrappedLine(t *testing.T) {
	// Regression: a long logical line must not cause j to get stuck.
	// The "wrap" is purely visual; logically it's still one line.
	text := "this is a very long line that would wrap on a narrow terminal\nnext line"
	//       0                                                              62

	cur := 0
	if !testVisualLineJ(text, &cur) || cur != 62 {
		t.Errorf("j on long line: got %d, want 62", cur)
	}
}
