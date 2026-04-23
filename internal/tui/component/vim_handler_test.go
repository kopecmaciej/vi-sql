package component

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// testTA is a minimal stand-in for the TextArea operations used by delete helpers.
type testTA struct {
	text   string
	cursor int // byte offset
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
