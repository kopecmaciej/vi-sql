package component

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

type vimMode int

const (
	vimInsert vimMode = iota
	vimNormal
	vimVisual
	vimVisualLine
)

type vimHandler struct {
	mode       vimMode
	pending    string // buffer for operator sequences: "d", "c", "y"...
	selStart   int    // byte offset of the anchor char (where visual/visual-line began)
	selCurrent int    // byte offset of the active cursor char in visual modes
	editor     *SQLQueryEditor
}

func newVimHandler(e *SQLQueryEditor) *vimHandler {
	return &vimHandler{mode: vimInsert, editor: e}
}

func (v *vimHandler) setPending(p string) {
	v.pending = p
	var r rune
	if len(p) > 0 {
		r = rune(p[0])
	}
	v.editor.App.GetManager().Broadcast(manager.NewSequencePendingChangedMsg(r))
}

func (v *vimHandler) transitionTo(m vimMode) {
	v.mode = m
	v.setPending("")
	v.editor.App.GetKeys().Reset()
	if m == vimInsert {
		v.editor.App.SetCursorStyle(tcell.CursorStyleSteadyBar)
	} else {
		v.editor.TextArea.HideAutocomplete()
		v.editor.App.SetCursorStyle(tcell.CursorStyleSteadyBlock)
	}
	v.editor.refreshTitle()
}

// Handle processes an input event and returns true if it was consumed.
// Non-rune keys (Ctrl+Enter, arrows, etc.) always fall through so existing
// editor keybindings (execute, paste, history, autocomplete) remain intact.
func (v *vimHandler) Handle(event *tcell.EventKey, setFocus func(tview.Primitive)) bool {
	if event.Key() == tcell.KeyEscape {
		switch v.mode {
		case vimInsert:
			v.enterNormal()
			// Autocomplete open — also let TextArea close it.
			if v.editor.TextArea.IsAutocompleteVisible() {
				return false
			}
			return true
		case vimVisual, vimVisualLine:
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
	case vimVisualLine:
		return v.handleVisualLine(event, setFocus)
	}
	return false // Insert — don't consume
}

func (v *vimHandler) enterNormal() {
	v.transitionTo(vimNormal)
	// Collapse any active selection so the highlight disappears immediately.
	ta := v.editor.TextArea
	pos := ta.GetCursorByteOffset()
	ta.Select(pos, pos)
}

func (v *vimHandler) enterInsert() {
	v.transitionTo(vimInsert)
}

func (v *vimHandler) enterVisual() {
	v.transitionTo(vimVisual)
	ta := v.editor.TextArea
	v.selStart = ta.GetCursorByteOffset()
	v.selCurrent = v.selStart
	v.applyVisualSelection()
}

// applyVisualSelection calls ta.Select with the correct [start, end) range for
// the current visual anchor (selStart) and active cursor (selCurrent).
func (v *vimHandler) applyVisualSelection() {
	ta := v.editor.TextArea
	start, end := visualSelectionRange(ta.GetText(), v.selStart, v.selCurrent)
	ta.Select(start, end)
}

func (v *vimHandler) enterVisualLine() {
	v.transitionTo(vimVisualLine)
	ta := v.editor.TextArea
	text := ta.GetText()
	pos := ta.GetCursorByteOffset()
	lineStart := strings.LastIndexByte(text[:pos], '\n') + 1
	v.selStart = lineStart
	v.selCurrent = lineStart
	v.applyVisualLineSelection()
}

func (v *vimHandler) applyVisualLineSelection() {
	ta := v.editor.TextArea
	start, end := visualLineRange(ta.GetText(), v.selStart, v.selCurrent)
	ta.Select(start, end)
}

// visualSelectionRange returns the [start, end) byte range for ta.Select given
// anchor and cursor as inclusive char positions. The char at cursor is always
// included (vim-inclusive visual semantics).
func visualSelectionRange(text string, anchor, cursor int) (start, end int) {
	if cursor >= anchor {
		end = cursor
		if cursor < len(text) {
			_, sz := utf8.DecodeRuneInString(text[cursor:])
			end = cursor + sz
		}
		return anchor, end
	}
	end = anchor
	if anchor < len(text) {
		_, sz := utf8.DecodeRuneInString(text[anchor:])
		end = anchor + sz
	}
	return cursor, end
}

// visualLineRange returns the [start, end) byte range for a linewise selection
// given two line-start byte offsets (either order). The trailing newline of the
// last selected line is included so that `dd` / `V d` removes full lines.
func visualLineRange(text string, lineA, lineB int) (start, end int) {
	if lineA > lineB {
		lineA, lineB = lineB, lineA
	}
	start = lineA
	eol := strings.IndexByte(text[lineB:], '\n')
	if eol < 0 {
		end = len(text)
	} else {
		end = lineB + eol + 1
	}
	return
}

func synth(key tcell.Key) *tcell.EventKey {
	return tcell.NewEventKey(key, 0, tcell.ModNone)
}

// yankLineBounds returns [lineStart, lineEnd) covering the full line at pos,
// including the trailing '\n' so that `yy` highlights the whole line.
func yankLineBounds(text string, pos int) (start, end int) {
	start = strings.LastIndexByte(text[:pos], '\n') + 1
	eol := strings.IndexByte(text[pos:], '\n')
	if eol < 0 {
		return start, len(text)
	}
	return start, pos + eol + 1
}

// yankToEOLBounds returns [pos, eol) — from pos to just before '\n' (or end of
// text), matching vim's y$ which does not capture the newline.
func yankToEOLBounds(text string, pos int) (start, end int) {
	eol := strings.IndexByte(text[pos:], '\n')
	if eol < 0 {
		return pos, len(text)
	}
	return pos, pos + eol
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
	if ev.Modifiers() != tcell.ModNone {
		return false
	}
	ch := ev.Rune()

	// Resolve pending operator + motion.
	if v.pending != "" {
		p := v.pending
		v.setPending("")
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
		case "d":
			switch ch {
			case 'd':
				v.deleteLine()
			case 'w':
				v.deleteForward(func() { ta.MoveWordRight(true, true) })
			case 'e':
				v.deleteForwardInclusive(func() { ta.MoveWordRight(false, true) })
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
				v.deleteForwardInclusive(func() { ta.MoveWordRight(false, true) })
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
			text := ta.GetText()
			pos := ta.GetCursorByteOffset()
			var hlStart, hlEnd int
			switch ch {
			case 'y':
				hlStart, hlEnd = yankLineBounds(text, pos)
				util.Copy(text[hlStart:hlEnd])
			case 'w':
				ta.MoveWordRight(true, true)
				newPos := ta.GetCursorByteOffset()
				util.Copy(text[pos:newPos])
				// Use Select(pos, pos) rather than byteToRowCol+MoveCursorTo:
				// MoveCursorTo uses display rows (wraps at terminal width) while
				// byteToRowCol counts logical lines, causing misalignment on long lines.
				ta.Select(pos, pos)
				hlStart, hlEnd = pos, newPos
			case 'e':
				ta.MoveWordRight(false, true)
				newPos := ta.GetCursorByteOffset()
				if newPos < len(text) {
					_, sz := utf8.DecodeRuneInString(text[newPos:])
					newPos += sz
				}
				util.Copy(text[pos:newPos])
				ta.Select(pos, pos)
				hlStart, hlEnd = pos, newPos
			case 'b':
				ta.MoveWordLeft(true)
				newPos := ta.GetCursorByteOffset()
				util.Copy(text[newPos:pos])
				ta.Select(pos, pos)
				hlStart, hlEnd = newPos, pos
			case '$':
				hlStart, hlEnd = yankToEOLBounds(text, pos)
				util.Copy(text[hlStart:hlEnd])
			}
			v.editor.BeginYankHighlight(hlStart, hlEnd)
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
		case "g":
			if ch == 'g' {
				v.editor.TextArea.MoveCursorTo(0, 0)
			}
			return true
		}
		return true
	}

	// When global WrapInputCapture absorbed the first rune of a sequence, transfer
	// it to v.pending so this handler resolves the full sequence.
	kb := v.editor.App.GetKeys()
	if kb.HasPending() {
		op := kb.GetPending()
		kb.Reset()
		if strings.ContainsRune("dcyrfFtTg", op) {
			v.setPending(string(op))
			return v.handleNormal(ev, setFocus)
		}
		return true
	}
	if kb.IsSequencePrefix(ch) {
		kb.SetPending(ch)
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
		v.setPending("r")
		return true
	case 's':
		v.deleteCharUnderCursor()
		v.enterInsert()
	case 'S':
		v.changeCurrentLine()

	case 'd':
		v.setPending("d")
		return true
	case 'D':
		v.deleteToEOL()
	case 'c':
		v.setPending("c")
		return true
	case 'C':
		v.deleteToEOL()
		v.enterInsert()
	case 'x':
		v.deleteCharUnderCursor()

	case 'y':
		v.setPending("y")
		return true
	case 'Y':
		text := ta.GetText()
		start, end := yankToEOLBounds(text, ta.GetCursorByteOffset())
		util.Copy(text[start:end])
		v.editor.BeginYankHighlight(start, end)

	case 'p':
		if text := util.Paste(); text != "" {
			pos := ta.GetCursorByteOffset()
			after := ta.GetTextAfterCursor()
			insertAt := pos
			if len(after) > 0 {
				_, size := utf8.DecodeRuneInString(after)
				insertAt = pos + size
			}
			ta.Replace(insertAt, insertAt, text)
		}
	case 'P':
		if text := util.Paste(); text != "" {
			pos := ta.GetCursorByteOffset()
			ta.Replace(pos, pos, text)
		}

	case 'J':
		v.joinLines()

	case 'u':
		ta.InputHandler()(synth(tcell.KeyCtrlZ), setFocus)

	case 'f':
		v.setPending("f")
		return true
	case 'F':
		v.setPending("F")
		return true
	case 't':
		v.setPending("t")
		return true
	case 'T':
		v.setPending("T")
		return true

	case 'v':
		v.enterVisual()
	case 'V':
		v.enterVisualLine()

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

	applySelection := func(newCurrent int) {
		v.selCurrent = newCurrent
		v.applyVisualSelection()
	}

	// Clear current selection to selCurrent, run move, then reapply from selStart.
	// Using selCurrent (not ta.GetCursorByteOffset()) ensures the move starts from
	// the active end even when the selection is backward.
	clearAndMove := func(move func()) {
		ta.Select(v.selCurrent, v.selCurrent)
		move()
		v.selCurrent = ta.GetCursorByteOffset()
		v.applyVisualSelection()
	}

	// Resolve pending "g" in visual mode.
	if v.pending == "g" {
		v.setPending("")
		if ch == 'g' {
			clearAndMove(func() { ta.MoveCursorTo(0, 0) })
		}
		return true
	}

	switch ch {
	case 'h':
		// Bypass InputHandler: KeyLeft with an active selection jumps to the
		// selection start rather than moving left by one char.
		text := ta.GetText()
		if v.selCurrent > 0 {
			_, size := utf8.DecodeLastRuneInString(text[:v.selCurrent])
			applySelection(v.selCurrent - size)
		}
	case 'l':
		// Bypass InputHandler for the same reason.
		text := ta.GetText()
		if v.selCurrent < len(text) {
			_, size := utf8.DecodeRuneInString(text[v.selCurrent:])
			applySelection(v.selCurrent + size)
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
		v.setPending("g")
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
		sel, hlStart, hlEnd := ta.GetSelection()
		util.Copy(sel)
		v.enterNormal()
		v.editor.BeginYankHighlight(hlStart, hlEnd)
	default:
		return true // consume all unrecognised runes — Visual mode doesn't type
	}
	return true
}

func (v *vimHandler) handleVisualLine(ev *tcell.EventKey, _ func(tview.Primitive)) bool {
	ta := v.editor.TextArea

	if ev.Key() != tcell.KeyRune {
		return false
	}
	ch := ev.Rune()

	moveAndUpdate := func(move func()) {
		move()
		newPos := ta.GetCursorByteOffset()
		text := ta.GetText()
		v.selCurrent = strings.LastIndexByte(text[:newPos], '\n') + 1
		v.applyVisualLineSelection()
	}

	if v.pending == "g" {
		v.setPending("")
		if ch == 'g' {
			moveAndUpdate(func() { ta.MoveCursorTo(0, 0) })
		}
		return true
	}

	switch ch {
	case 'j':
		// Use byte search instead of KeyDown so wrapped screen lines don't confuse
		// movement — KeyDown advances by screen row, not logical line.
		text := ta.GetText()
		if eol := strings.IndexByte(text[v.selCurrent:], '\n'); eol >= 0 {
			v.selCurrent += eol + 1
			v.applyVisualLineSelection()
		}
	case 'k':
		text := ta.GetText()
		if v.selCurrent > 0 {
			prev := strings.LastIndexByte(text[:v.selCurrent-1], '\n')
			v.selCurrent = prev + 1
			v.applyVisualLineSelection()
		}
	case 'G':
		text := ta.GetText()
		lastRow := strings.Count(text, "\n")
		moveAndUpdate(func() { ta.MoveCursorTo(lastRow, 0) })
	case 'g':
		v.setPending("g")
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
		sel, hlStart, hlEnd := ta.GetSelection()
		util.Copy(sel)
		v.enterNormal()
		v.editor.BeginYankHighlight(hlStart, hlEnd)
	case 'v':
		// Switch to char-visual at the current cursor position.
		v.mode = vimVisual
		v.selStart = v.selCurrent
		v.applyVisualSelection()
		v.editor.refreshTitle()
	default:
		return true
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

// deleteForwardInclusive is like deleteForward but also deletes the char the
// cursor lands on — used for e-motion (cursor-inclusive) variants like de/ce.
func (v *vimHandler) deleteForwardInclusive(move func()) {
	ta := v.editor.TextArea
	pos := ta.GetCursorByteOffset()
	move()
	newPos := ta.GetCursorByteOffset()
	if newPos > pos {
		after := ta.GetTextAfterCursor()
		if len(after) > 0 {
			_, sz := utf8.DecodeRuneInString(after)
			newPos += sz
		}
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
