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
	pending  string // buffer for multi-key sequences: "d", "g", "c", "y", "f", "F", "t", "T", "r"
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

// byteToRowCol converts a byte offset to (row, col) where col is rune-count from line start.
func byteToRowCol(text string, offset int) (row, col int) {
	if offset > len(text) {
		offset = len(text)
	}
	before := text[:offset]
	row = strings.Count(before, "\n")
	lastNL := strings.LastIndexByte(before, '\n') + 1
	col = utf8.RuneCountInString(before[lastNL:])
	return
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

	// Resolve pending operator + motion.
	if v.pending != "" {
		p := v.pending
		v.pending = ""
		switch p {
		case "r":
			after := ta.GetTextAfterCursor()
			if len(after) > 0 && after[0] != '\n' {
				_, oldSize := utf8.DecodeRuneInString(after)
				pos := ta.GetCursorByteOffset()
				ta.Replace(pos, pos+oldSize, string(ch))
				ta.InputHandler()(synth(tcell.KeyLeft), setFocus)
			}
			return true
		case "g":
			if ch == 'g' {
				ta.MoveCursorTo(0, 0)
			} else {
				return v.handleNormal(ev, setFocus)
			}
			return true
		case "d":
			switch ch {
			case 'd':
				v.deleteLine()
			case 'w':
				v.deleteForward(func() { ta.MoveWordRight(true, true) })
			case 'e':
				v.deleteForward(func() { ta.MoveWordRight(false, true) })
			case 'b':
				v.deleteBackward(func() { ta.MoveWordLeft(true) })
			case '$':
				v.deleteToEOL()
			default:
				return v.handleNormal(ev, setFocus)
			}
			return true
		case "c":
			switch ch {
			case 'c':
				v.changeCurrentLine()
			case 'w':
				v.deleteForward(func() { ta.MoveWordRight(true, true) })
				v.enterInsert()
			case 'e':
				v.deleteForward(func() { ta.MoveWordRight(false, true) })
				v.enterInsert()
			case 'b':
				v.deleteBackward(func() { ta.MoveWordLeft(true) })
				v.enterInsert()
			case '$':
				v.deleteToEOL()
				v.enterInsert()
			}
			return true
		case "y":
			write, _ := util.GetClipboard()
			if write != nil {
				text := ta.GetText()
				pos := ta.GetCursorByteOffset()
				switch ch {
				case 'y':
					v.yankLine(write)
				case 'w':
					ta.MoveWordRight(true, true)
					newPos := ta.GetCursorByteOffset()
					write(text[pos:newPos])
					row, col := byteToRowCol(text, pos)
					ta.MoveCursorTo(row, col)
				case 'b':
					ta.MoveWordLeft(true)
					newPos := ta.GetCursorByteOffset()
					write(text[newPos:pos])
					row, col := byteToRowCol(text, pos)
					ta.MoveCursorTo(row, col)
				case '$':
					eol := strings.IndexByte(text[pos:], '\n')
					if eol < 0 {
						write(text[pos:])
					} else {
						write(text[pos : pos+eol])
					}
				}
			}
			return true
		case "f":
			v.findCharForward(ch, false)
			return true
		case "F":
			v.findCharBackward(ch, false)
			return true
		case "t":
			v.findCharForward(ch, true)
			return true
		case "T":
			v.findCharBackward(ch, true)
			return true
		}
		return true
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
	case '^':
		v.moveToFirstNonBlank()
	case '$':
		ta.InputHandler()(synth(tcell.KeyEnd), setFocus)
	case '{':
		v.jumpToPrevBlankLine()
	case '}':
		v.jumpToNextBlankLine()

	case 'g':
		v.pending = "g"
		return true
	case 'G':
		text := ta.GetText()
		lastRow := strings.Count(text, "\n")
		ta.MoveCursorTo(lastRow, -1)

	case 'i':
		v.enterInsert()
	case 'I':
		v.moveToFirstNonBlank()
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
	case 's':
		v.deleteCharUnderCursor()
		v.enterInsert()
	case 'S':
		v.changeCurrentLine()

	case 'd':
		v.pending = "d"
		return true
	case 'D':
		v.deleteToEOL()
	case 'c':
		v.pending = "c"
		return true
	case 'C':
		v.deleteToEOL()
		v.enterInsert()
	case 'x':
		v.deleteCharUnderCursor()

	case 'y':
		v.pending = "y"
		return true
	case 'Y':
		write, _ := util.GetClipboard()
		if write != nil {
			v.yankLine(write)
		}

	case 'p':
		_, read := util.GetClipboard()
		if read != nil {
			if text := read(); text != "" {
				pos := ta.GetCursorByteOffset()
				after := ta.GetTextAfterCursor()
				insertAt := pos
				if len(after) > 0 {
					_, size := utf8.DecodeRuneInString(after)
					insertAt = pos + size
				}
				ta.Replace(insertAt, insertAt, text)
			}
		}
	case 'P':
		_, read := util.GetClipboard()
		if read != nil {
			if text := read(); text != "" {
				pos := ta.GetCursorByteOffset()
				ta.Replace(pos, pos, text)
			}
		}

	case 'J':
		v.joinLines()

	case 'u':
		ta.InputHandler()(synth(tcell.KeyCtrlZ), setFocus)

	case 'f':
		v.pending = "f"
		return true
	case 'F':
		v.pending = "F"
		return true
	case 't':
		v.pending = "t"
		return true
	case 'T':
		v.pending = "T"
		return true

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
	ch := ev.Rune()

	applySelection := func(newCursor int) {
		if newCursor >= v.selStart {
			ta.Select(v.selStart, newCursor)
		} else {
			ta.Select(newCursor, v.selStart)
		}
	}

	// Clear current selection, run move, then reapply from selStart.
	clearAndMove := func(move func()) {
		cur := ta.GetCursorByteOffset()
		ta.Select(cur, cur)
		move()
		applySelection(ta.GetCursorByteOffset())
	}

	// Resolve pending "g" in visual mode.
	if v.pending == "g" {
		v.pending = ""
		if ch == 'g' {
			clearAndMove(func() { ta.MoveCursorTo(0, 0) })
		}
		return true
	}

	switch ch {
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
		clearAndMove(func() { ta.InputHandler()(synth(tcell.KeyDown), setFocus) })
	case 'k':
		clearAndMove(func() { ta.InputHandler()(synth(tcell.KeyUp), setFocus) })
	case 'w':
		clearAndMove(func() { ta.MoveWordRight(true, true) })
	case 'e':
		clearAndMove(func() { ta.MoveWordRight(false, true) })
	case 'b':
		clearAndMove(func() { ta.MoveWordLeft(true) })
	case '0':
		clearAndMove(func() { ta.InputHandler()(synth(tcell.KeyHome), setFocus) })
	case '^':
		clearAndMove(func() { v.moveToFirstNonBlank() })
	case '$':
		clearAndMove(func() { ta.InputHandler()(synth(tcell.KeyEnd), setFocus) })
	case 'G':
		text := ta.GetText()
		lastRow := strings.Count(text, "\n")
		clearAndMove(func() { ta.MoveCursorTo(lastRow, 0) })
	case 'g':
		v.pending = "g"
		return true
	case 'd', 'x':
		_, start, end := ta.GetSelection()
		ta.Replace(start, end, "")
		v.enterNormal()
	case 'c':
		_, start, end := ta.GetSelection()
		ta.Replace(start, end, "")
		v.enterInsert()
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

// deleteForward deletes from current cursor position to wherever move() lands.
func (v *vimHandler) deleteForward(move func()) {
	ta := v.editor.TextArea
	pos := ta.GetCursorByteOffset()
	move()
	newPos := ta.GetCursorByteOffset()
	if newPos > pos {
		ta.Replace(pos, newPos, "")
	}
}

// deleteBackward deletes from wherever move() lands back to the original cursor position.
func (v *vimHandler) deleteBackward(move func()) {
	ta := v.editor.TextArea
	pos := ta.GetCursorByteOffset()
	move()
	newPos := ta.GetCursorByteOffset()
	if pos > newPos {
		ta.Replace(newPos, pos, "")
	}
}

func (v *vimHandler) changeCurrentLine() {
	ta := v.editor.TextArea
	text := ta.GetText()
	pos := ta.GetCursorByteOffset()
	lineStart := strings.LastIndexByte(text[:pos], '\n') + 1
	lineEnd := strings.IndexByte(text[pos:], '\n')
	var end int
	if lineEnd < 0 {
		end = len(text)
	} else {
		end = pos + lineEnd
	}
	ta.Replace(lineStart, end, "")
	v.enterInsert()
}

func (v *vimHandler) yankLine(write func(string)) {
	ta := v.editor.TextArea
	text := ta.GetText()
	pos := ta.GetCursorByteOffset()
	lineStart := strings.LastIndexByte(text[:pos], '\n') + 1
	eol := strings.IndexByte(text[pos:], '\n')
	if eol < 0 {
		write(text[lineStart:])
	} else {
		write(text[lineStart : pos+eol+1])
	}
}

func (v *vimHandler) moveToFirstNonBlank() {
	ta := v.editor.TextArea
	text := ta.GetText()
	pos := ta.GetCursorByteOffset()
	lineStart := strings.LastIndexByte(text[:pos], '\n') + 1
	row := strings.Count(text[:lineStart], "\n")
	col := 0
	for lineStart+col < len(text) && (text[lineStart+col] == ' ' || text[lineStart+col] == '\t') {
		col++ // spaces and tabs are single-byte
	}
	if lineStart+col >= len(text) || text[lineStart+col] == '\n' {
		col = 0 // blank line — stay at col 0
	}
	ta.MoveCursorTo(row, col)
}

func (v *vimHandler) joinLines() {
	ta := v.editor.TextArea
	text := ta.GetText()
	pos := ta.GetCursorByteOffset()

	nlIdx := strings.IndexByte(text[pos:], '\n')
	if nlIdx < 0 {
		return // last line
	}
	eol := pos + nlIdx
	nextStart := eol + 1
	skip := 0
	for nextStart+skip < len(text) && (text[nextStart+skip] == ' ' || text[nextStart+skip] == '\t') {
		skip++
	}
	sep := " "
	if nextStart+skip >= len(text) || text[nextStart+skip] == '\n' {
		sep = "" // next line is blank
	}
	ta.Replace(eol, nextStart+skip, sep)
	if sep == " " {
		// Place cursor on the inserted space.
		newText := ta.GetText()
		row, col := byteToRowCol(newText, eol)
		ta.MoveCursorTo(row, col)
	}
}

func (v *vimHandler) jumpToNextBlankLine() {
	ta := v.editor.TextArea
	text := ta.GetText()
	pos := ta.GetCursorByteOffset()

	// Skip past the current line.
	nlIdx := strings.IndexByte(text[pos:], '\n')
	if nlIdx < 0 {
		return
	}
	searchFrom := pos + nlIdx + 1
	idx := strings.Index(text[searchFrom:], "\n\n")
	if idx < 0 {
		lastRow := strings.Count(text, "\n")
		ta.MoveCursorTo(lastRow, 0)
		return
	}
	target := searchFrom + idx + 1 // byte of the blank line's \n
	row, _ := byteToRowCol(text, target)
	ta.MoveCursorTo(row, 0)
}

func (v *vimHandler) jumpToPrevBlankLine() {
	ta := v.editor.TextArea
	text := ta.GetText()
	pos := ta.GetCursorByteOffset()

	lineStart := strings.LastIndexByte(text[:pos], '\n')
	if lineStart < 0 {
		ta.MoveCursorTo(0, 0)
		return
	}
	idx := strings.LastIndex(text[:lineStart], "\n\n")
	if idx < 0 {
		ta.MoveCursorTo(0, 0)
		return
	}
	target := idx + 1 // byte of the blank line's \n
	row, _ := byteToRowCol(text, target)
	ta.MoveCursorTo(row, 0)
}

func (v *vimHandler) findCharForward(target rune, till bool) {
	ta := v.editor.TextArea
	text := ta.GetText()
	pos := ta.GetCursorByteOffset()

	// Start from char after cursor.
	i := pos
	if i < len(text) {
		_, sz := utf8.DecodeRuneInString(text[i:])
		i += sz
	}

	prev := pos
	for i < len(text) {
		r, sz := utf8.DecodeRuneInString(text[i:])
		if r == '\n' {
			return
		}
		if r == target {
			dest := i
			if till {
				dest = prev
			}
			row, col := byteToRowCol(text, dest)
			ta.MoveCursorTo(row, col)
			return
		}
		prev = i
		i += sz
	}
}

func (v *vimHandler) findCharBackward(target rune, till bool) {
	ta := v.editor.TextArea
	text := ta.GetText()
	pos := ta.GetCursorByteOffset()
	lineStart := strings.LastIndexByte(text[:pos], '\n') + 1

	i := pos
	for i > lineStart {
		_, prevSz := utf8.DecodeLastRuneInString(text[:i])
		i -= prevSz
		ch, chSz := utf8.DecodeRuneInString(text[i:])
		if ch == target {
			dest := i
			if till {
				dest = i + chSz
				if dest >= pos {
					return
				}
			}
			row, col := byteToRowCol(text, dest)
			ta.MoveCursorTo(row, col)
			return
		}
	}
}
