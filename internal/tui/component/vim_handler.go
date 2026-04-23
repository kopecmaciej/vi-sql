package component

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

type vimMode int

const (
	vimInsert vimMode = iota
	vimNormal
	vimVisual
)

type vimHandler struct {
	mode     vimMode
	pending  string // buffer for multi-key sequences: "d", "g"
	selStart int    // byte offset where visual selection began
	editor   *SQLQueryEditor
}

func newVimHandler(e *SQLQueryEditor) *vimHandler {
	return &vimHandler{mode: vimInsert, editor: e}
}

// Handle processes an input event and returns true if it was consumed.
// Non-rune keys (Ctrl+Enter, arrows, etc.) always fall through so existing
// editor keybindings (execute, paste, history, autocomplete) remain intact.
func (v *vimHandler) Handle(event *tcell.EventKey, setFocus func(tview.Primitive)) bool {
	if event.Key() == tcell.KeyEscape {
		switch v.mode {
		case vimInsert:
			// Autocomplete open — let Escape close it; stay in Insert.
			if v.editor.TextArea.IsAutocompleteVisible() {
				return false
			}
			v.enterNormal()
			return true
		case vimVisual:
			v.enterNormal()
			return true
		case vimNormal:
			// Propagate so onCancel (e.g. cancel running query) can fire.
			return false
		}
	}

	switch v.mode {
	case vimNormal:
		return v.handleNormal(event, setFocus)
	case vimVisual:
		return v.handleVisual(event, setFocus)
	}
	return false // Insert — don't consume
}

func (v *vimHandler) enterNormal() {
	v.mode = vimNormal
	v.pending = ""
	v.editor.refreshTitle()
}

func (v *vimHandler) enterInsert() {
	v.mode = vimInsert
	v.pending = ""
	v.editor.refreshTitle()
}

func (v *vimHandler) enterVisual() {
	v.mode = vimVisual
	v.pending = ""
	ta := v.editor.TextArea
	v.selStart = ta.GetCursorByteOffset()
	// Select the char under the cursor immediately (vim inclusive visual).
	end := v.selStart
	after := ta.GetTextAfterCursor()
	if len(after) > 0 && after[0] != '\n' {
		_, size := utf8.DecodeRuneInString(after)
		end = v.selStart + size
	}
	ta.Select(v.selStart, end)
	v.editor.refreshTitle()
}

func synth(key tcell.Key) *tcell.EventKey {
	return tcell.NewEventKey(key, 0, tcell.ModNone)
}

func (v *vimHandler) handleNormal(ev *tcell.EventKey, setFocus func(tview.Primitive)) bool {
	ta := v.editor.TextArea

	if ev.Key() == tcell.KeyCtrlR {
		ta.InputHandler()(synth(tcell.KeyCtrlY), setFocus)
		return true
	}

	if ev.Key() != tcell.KeyRune {
		return false
	}
	ch := ev.Rune()

	// Resolve pending "r" (replace char under cursor).
	if v.pending == "r" {
		v.pending = ""
		after := ta.GetTextAfterCursor()
		if len(after) > 0 && after[0] != '\n' {
			_, oldSize := utf8.DecodeRuneInString(after)
			pos := ta.GetCursorByteOffset()
			ta.Replace(pos, pos+oldSize, string(ch))
			ta.InputHandler()(synth(tcell.KeyLeft), setFocus)
		}
		return true
	}

	// Resolve pending "d" operator.
	if v.pending == "d" {
		v.pending = ""
		if ch == 'd' {
			v.deleteLine()
			return true
		}
		// Unknown motion after "d" — clear pending and re-dispatch as a fresh key.
		return v.handleNormal(ev, setFocus)
	}

	// Resolve pending "g" operator.
	if v.pending == "g" {
		v.pending = ""
		if ch == 'g' {
			ta.MoveCursorTo(0, 0)
			return true
		}
		return v.handleNormal(ev, setFocus)
	}

	switch ch {
	case 'h':
		ta.InputHandler()(synth(tcell.KeyLeft), setFocus)
	case 'l':
		ta.InputHandler()(synth(tcell.KeyRight), setFocus)
	case 'j':
		ta.InputHandler()(synth(tcell.KeyDown), setFocus)
	case 'k':
		ta.InputHandler()(synth(tcell.KeyUp), setFocus)

	case 'w':
		ta.MoveWordRight(true, true)
	case 'e':
		ta.MoveWordRight(false, true)
	case 'b':
		ta.MoveWordLeft(true)

	case '0':
		ta.InputHandler()(synth(tcell.KeyHome), setFocus)
	case '$':
		ta.InputHandler()(synth(tcell.KeyEnd), setFocus)

	case 'g':
		v.pending = "g"
		return true
	case 'G':
		text := ta.GetText()
		lastRow := strings.Count(text, "\n")
		ta.MoveCursorTo(lastRow, -1)

	case 'i':
		v.enterInsert()
	case 'a':
		ta.InputHandler()(synth(tcell.KeyRight), setFocus)
		v.enterInsert()
	case 'A':
		ta.InputHandler()(synth(tcell.KeyEnd), setFocus)
		v.enterInsert()
	case 'o':
		ta.InputHandler()(synth(tcell.KeyEnd), setFocus)
		ta.InputHandler()(synth(tcell.KeyEnter), setFocus)
		v.enterInsert()
	case 'O':
		ta.InputHandler()(synth(tcell.KeyHome), setFocus)
		ta.InputHandler()(synth(tcell.KeyEnter), setFocus)
		ta.InputHandler()(synth(tcell.KeyUp), setFocus)
		v.enterInsert()

	case 'r':
		v.pending = "r"
		return true

	case 'd':
		v.pending = "d"
		return true
	case 'D':
		v.deleteToEOL()
	case 'x':
		v.deleteCharUnderCursor()

	case 'p':
		_, read := util.GetClipboard()
		if read != nil {
			if text := read(); text != "" {
				pos := ta.GetCursorByteOffset()
				ta.Replace(pos, pos, text)
			}
		}

	case 'u':
		ta.InputHandler()(synth(tcell.KeyCtrlZ), setFocus)

	case 'v':
		v.enterVisual()

	default:
		return true // consume all unrecognised runes — Normal mode doesn't type
	}
	return true
}

func (v *vimHandler) handleVisual(ev *tcell.EventKey, setFocus func(tview.Primitive)) bool {
	ta := v.editor.TextArea

	if ev.Key() != tcell.KeyRune {
		return false
	}

	// Extend selection to the new cursor byte offset.
	// tview always puts its cursor at the higher byte; we track v.selStart as anchor.
	applySelection := func(newCursor int) {
		if newCursor >= v.selStart {
			ta.Select(v.selStart, newCursor)
		} else {
			ta.Select(newCursor, v.selStart)
		}
	}

	// For j/k (row-based): use InputHandler (no selection-jump side-effect),
	// then recompute selection from v.selStart to wherever the cursor ended up.
	moveRow := func(key tcell.Key) {
		// Clear selection first so KeyDown/KeyUp don't have to fight it.
		cur := ta.GetCursorByteOffset()
		ta.Select(cur, cur)
		ta.InputHandler()(tcell.NewEventKey(key, 0, tcell.ModNone), setFocus)
		applySelection(ta.GetCursorByteOffset())
	}

	switch ev.Rune() {
	case 'h':
		// Bypass InputHandler: KeyLeft with an active selection jumps to the
		// selection start rather than moving left by one char.
		cur := ta.GetCursorByteOffset()
		text := ta.GetText()
		if cur > 0 {
			_, size := utf8.DecodeLastRuneInString(text[:cur])
			applySelection(cur - size)
		}
	case 'l':
		// Bypass InputHandler for the same reason.
		after := ta.GetTextAfterCursor()
		if len(after) > 0 {
			_, size := utf8.DecodeRuneInString(after)
			applySelection(ta.GetCursorByteOffset() + size)
		}
	case 'j':
		moveRow(tcell.KeyDown)
	case 'k':
		moveRow(tcell.KeyUp)
	case 'w':
		cur := ta.GetCursorByteOffset()
		ta.Select(cur, cur)
		ta.MoveWordRight(true, true)
		applySelection(ta.GetCursorByteOffset())
	case 'b':
		cur := ta.GetCursorByteOffset()
		ta.Select(cur, cur)
		ta.MoveWordLeft(true)
		applySelection(ta.GetCursorByteOffset())
	case 'd', 'x':
		_, start, end := ta.GetSelection()
		ta.Replace(start, end, "")
		v.enterNormal()
	case 'y':
		sel, _, _ := ta.GetSelection()
		write, _ := util.GetClipboard()
		if write != nil {
			write(sel)
		}
		v.enterNormal()
	default:
		return true // consume all unrecognised runes — Visual mode doesn't type
	}
	return true
}

func (v *vimHandler) deleteCharUnderCursor() {
	ta := v.editor.TextArea
	after := ta.GetTextAfterCursor()
	// No-op on newline (matches vim behaviour — x doesn't join lines).
	if len(after) == 0 || after[0] == '\n' {
		return
	}
	_, size := utf8.DecodeRuneInString(after)
	pos := ta.GetCursorByteOffset()
	ta.Replace(pos, pos+size, "")
}

func (v *vimHandler) deleteToEOL() {
	ta := v.editor.TextArea
	pos := ta.GetCursorByteOffset()
	after := ta.GetTextAfterCursor()
	if nl := strings.IndexByte(after, '\n'); nl >= 0 {
		ta.Replace(pos, pos+nl, "")
	} else {
		ta.Replace(pos, pos+len(after), "")
	}
}

func (v *vimHandler) deleteLine() {
	ta := v.editor.TextArea
	text := ta.GetText()
	pos := ta.GetCursorByteOffset()

	lineStart := strings.LastIndexByte(text[:pos], '\n') + 1
	lineEnd := strings.IndexByte(text[pos:], '\n')
	if lineEnd < 0 {
		// Last line — remove the preceding newline too, if any.
		if lineStart > 0 {
			lineStart--
		}
		ta.Replace(lineStart, len(text), "")
	} else {
		ta.Replace(lineStart, pos+lineEnd+1, "")
	}
}
