package component

import (
	"strconv"
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

	vimOperators = "dcy"
	vimPrefixes  = "gfFtTr"
)

// pending accumulates next key sequences, it's form is: [operatorCount][operator][motionCount]motion
type pending struct {
	prefix        rune
	motionCount   int
	operator      rune
	operatorCount int
}

// vimRegister is the vim yank/delete register. The system clipboard can't carry the
// linewise flag (and util.Paste trims trailing newlines), so we keep the raw
// text here and only trust it when the clipboard still matches what we yanked.
type vimRegister struct {
	text     string
	linewise bool
}

type vimHandler struct {
	mode            vimMode
	pending         pending
	selectionAnchor int
	selectionCursor int
	register        vimRegister
	editor          *SQLQueryEditor
}

func newVimHandler(e *SQLQueryEditor) *vimHandler {
	return &vimHandler{mode: vimInsert, editor: e}
}

func (v *vimHandler) notifyPending(s string) {
	v.editor.App.GetManager().Broadcast(manager.NewSequencePendingChangedMsg(s))
}

func (v *vimHandler) resetPending() {
	v.pending = pending{}
	v.notifyPending("")
}

// combineCount multiplies pre/post counts, like in ex: 2d4j
func combineCount(a, b int) int {
	if a == 0 && b == 0 {
		return 0
	}
	if a == 0 {
		a = 1
	}
	if b == 0 {
		b = 1
	}
	return a * b
}

func (v *vimHandler) transitionTo(m vimMode) {
	v.mode = m
	v.resetPending()
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
			if v.editor.TextArea.IsAutocompleteVisible() {
				return false
			}
			return true
		case vimVisual, vimVisualLine:
			v.enterNormal()
			return true
		case vimNormal:
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
	return false
}

func (v *vimHandler) enterNormal() {
	v.transitionTo(vimNormal)
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
	v.selectionAnchor = ta.GetCursorByteOffset()
	v.selectionCursor = v.selectionAnchor
	v.applySelection()
}

func (v *vimHandler) applySelection() {
	ta := v.editor.TextArea
	start, end := visualSelectionRange(ta.GetText(), v.selectionAnchor, v.selectionCursor)
	ta.Select(start, end)
	if v.selectionCursor < v.selectionAnchor {
		ta.SwapCursorAndSelectionStart()
	}
}

func (v *vimHandler) enterVisualLine() {
	v.transitionTo(vimVisualLine)
	ta := v.editor.TextArea
	text := ta.GetText()
	pos := ta.GetCursorByteOffset()
	lineStart := strings.LastIndexByte(text[:pos], '\n') + 1
	v.selectionAnchor = lineStart
	v.selectionCursor = lineStart
	v.applyLineSelection()
}

func (v *vimHandler) applyLineSelection() {
	ta := v.editor.TextArea
	start, end := visualLineRange(ta.GetText(), v.selectionAnchor, v.selectionCursor)
	ta.Select(start, end)
	if v.selectionCursor < v.selectionAnchor {
		ta.SwapCursorAndSelectionStart()
	}
}

// visualSelectionRange returns the [start, end) byte range for ta.Select given
// anchor and cursor as inclusive char positions. The char at cursor is always
// included (vim-inclusive visual semantics).
func visualSelectionRange(text string, anchor, cursor int) (start, end int) {
	if cursor >= anchor {
		end = cursor
		if cursor < len(text) {
			_, size := utf8.DecodeRuneInString(text[cursor:])
			end = cursor + size
		}
		return anchor, end
	}
	end = anchor
	if anchor < len(text) {
		_, size := utf8.DecodeRuneInString(text[anchor:])
		end = anchor + size
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

	if ev.Key() != tcell.KeyRune {
		switch ev.Key() {
		case tcell.KeyCtrlR:
			ta.InputHandler()(synth(tcell.KeyCtrlY), setFocus)
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			ta.InputHandler()(synth(tcell.KeyLeft), setFocus)
		case tcell.KeyDelete:
			v.deleteCharUnderCursor()
		case tcell.KeyEnter:
			ta.InputHandler()(synth(tcell.KeyDown), setFocus)
			v.moveToFirstNonBlank()
		default:
			return false
		}
		return true
	}
	if ev.Modifiers() != tcell.ModNone {
		return false
	}
	ch := ev.Rune()

	// The global sequence machine (on in normal mode) absorbs the first rune of a
	// sequence like `g`/`d`. When an upstream handler (e.g. main's `ge`) didn't
	// claim the full sequence, pull that prefix back into our pending struct so this
	// handler resolves it — counts and motions then follow the normal path.
	if v.pending.operator == 0 && v.pending.prefix == 0 {
		if kb := v.editor.App.GetKeys(); kb.HasPending() {
			if p := kb.GetPending(); len(p) == 1 {
				switch r := rune(p[0]); {
				case strings.ContainsRune(vimOperators, r):
					kb.Reset()
					v.pending.operator = r
					v.pending.operatorCount = v.pending.motionCount
					v.pending.motionCount = 0
				case strings.ContainsRune(vimPrefixes, r):
					kb.Reset()
					v.pending.prefix = r
				}
			}
		}
	}

	// Complete a pending multi-key prefix (g-motion, f/F/t/T target, r replacement).
	if v.pending.prefix != 0 {
		return v.resolvePrefix(ch, setFocus)
	}

	// Count digits: 1-9 always start/extend; 0 extends only a non-empty count,
	// otherwise it's the `0` motion.
	if (ch >= '1' && ch <= '9') || (ch == '0' && v.pending.motionCount > 0) {
		v.pending.motionCount = v.pending.motionCount*10 + int(ch-'0')
		v.notifyPending(v.pendingLabel())
		return true
	}

	if strings.ContainsRune(vimOperators, ch) {
		switch v.pending.operator {
		case ch: // dd/cc/yy — linewise current line(s)
			v.applyOperator(ch, motion{kind: mLinewise, run: lineDownStart}, combineCount(v.pending.operatorCount, v.pending.motionCount))
			v.resetPending()
		case 0:
			v.pending.operator = ch
			v.pending.operatorCount = v.pending.motionCount
			v.pending.motionCount = 0
			v.notifyPending(v.pendingLabel())
		default: // mismatched operator (e.g. dy) — cancel
			v.resetPending()
		}
		return true
	}

	if strings.ContainsRune(vimPrefixes, ch) {
		v.pending.prefix = ch
		v.notifyPending(v.pendingLabel())
		return true
	}

	// h/l/j/k: plain movement keeps TextArea row/column semantics; under an
	// operator they go through the motion table for proper ranges.
	if m, ok := simpleMotions[ch]; ok {
		if v.pending.operator != 0 {
			v.applyOperator(v.pending.operator, m, combineCount(v.pending.operatorCount, v.pending.motionCount))
		} else {
			v.moveSimple(ch, max(v.pending.motionCount, 1), setFocus)
		}
		v.resetPending()
		return true
	}

	// Table motions (word, line, document).
	if m, ok := motions[ch]; ok {
		v.runMotion(ch, m)
		v.resetPending()
		return true
	}

	// Operator was pending but the key is not a motion — cancel it.
	if v.pending.operator != 0 {
		v.resetPending()
		return true
	}

	count := max(v.pending.motionCount, 1)
	v.resetPending()
	return v.handleCommand(ch, count, setFocus)
}

// pendingLabel renders the in-progress command for the footer.
func (v *vimHandler) pendingLabel() string {
	var b strings.Builder
	if v.pending.operatorCount > 0 {
		b.WriteString(strconv.Itoa(v.pending.operatorCount))
	}
	if v.pending.operator != 0 {
		b.WriteRune(v.pending.operator)
	}
	if v.pending.motionCount > 0 {
		b.WriteString(strconv.Itoa(v.pending.motionCount))
	}
	if v.pending.prefix != 0 {
		b.WriteRune(v.pending.prefix)
	}
	return b.String()
}

// runMotion positions the cursor (plain movement) or applies the pending
// operator over the table motion m. ch drives the w/W operator quirks.
func (v *vimHandler) runMotion(ch rune, m motion) {
	ta := v.editor.TextArea
	text := ta.GetText()
	pos := ta.GetCursorByteOffset()
	if v.pending.operator != 0 {
		m = operatorMotion(v.pending.operator, ch, m, text, pos)
		v.applyOperator(v.pending.operator, m, combineCount(v.pending.operatorCount, v.pending.motionCount))
		return
	}
	dest := m.run(text, pos, v.pending.motionCount)
	ta.Select(dest, dest)
	_, _, row, col := ta.GetCursor()
	ta.MoveCursorTo(row, col)
}

// operatorMotion applies vim's word-motion quirks under an operator: dw/yw clamp
// at the newline; cw/cW on a non-blank fold to ce/cE.
func operatorMotion(op, ch rune, m motion, text string, pos int) motion {
	if ch != 'w' && ch != 'W' {
		return m
	}
	if op == 'c' {
		if r, _ := utf8.DecodeRuneInString(text[pos:]); pos < len(text) && runeClass(r, ch == 'W') != 0 {
			if ch == 'W' {
				return motions['E']
			}
			return motions['e']
		}
		return m
	}
	return clampWordMotion(m)
}

// applyOperator turns motion m's destination into a [start,end) range and runs
// the operator over it. Adding an operator is one more case here.
func (v *vimHandler) applyOperator(op rune, m motion, count int) {
	ta := v.editor.TextArea
	text := ta.GetText()
	pos := ta.GetCursorByteOffset()
	dest := m.run(text, pos, count)
	start, end := operatorRange(text, pos, dest, op, m.kind)
	if start >= end {
		if op == 'c' {
			v.enterInsert() // empty line: still enter insert
		}
		return
	}
	util.Copy(text[start:end])
	v.register = vimRegister{text: text[start:end], linewise: m.kind == mLinewise}
	switch op {
	case 'y':
		ta.Select(pos, pos)
		v.editor.BeginYankHighlight(start, end)
	case 'd':
		ta.Replace(start, end, "")
	case 'c':
		ta.Replace(start, end, "")
		v.enterInsert()
	}
}

// paste inserts the register (or external clipboard) at the cursor. A linewise
// yank (yy/dd) is pasted as whole lines below (after=true, p) or above (P);
// charwise pastes after/before the cursor rune like vim.
func (v *vimHandler) paste(after bool) {
	ta := v.editor.TextArea
	clip := util.Paste()
	text, linewise := clip, false
	if v.register.text != "" && strings.TrimSpace(v.register.text) == clip {
		text, linewise = v.register.text, v.register.linewise
	}
	if text == "" {
		return
	}

	full := ta.GetText()
	pos := ta.GetCursorByteOffset()

	if !linewise {
		insertAt := pos
		if after {
			if rest := ta.GetTextAfterCursor(); len(rest) > 0 {
				_, size := utf8.DecodeRuneInString(rest)
				insertAt = pos + size
			}
		}
		ta.Replace(insertAt, insertAt, text)
		return
	}

	body := strings.TrimSuffix(text, "\n")
	var at int
	var insert string
	if after {
		at = lineEndAfterNL(full, pos)
		if at == len(full) && (full == "" || full[len(full)-1] != '\n') {
			insert = "\n" + body // last line lacks a trailing newline: add one first
		} else {
			insert = body + "\n"
		}
	} else {
		at = lineStartAt(full, pos)
		insert = body + "\n"
	}
	ta.Replace(at, at, insert)

	pastedStart := at
	if strings.HasPrefix(insert, "\n") {
		pastedStart = at + 1
	}
	dest := firstNonBlankOffset(ta.GetText(), pastedStart)
	row, col := byteToRowCol(ta.GetText(), dest)
	ta.MoveCursorTo(row, col)
}

func (v *vimHandler) moveSimple(ch rune, count int, setFocus func(tview.Primitive)) {
	ta := v.editor.TextArea
	key := map[rune]tcell.Key{'h': tcell.KeyLeft, 'l': tcell.KeyRight, 'j': tcell.KeyDown, 'k': tcell.KeyUp}[ch]
	for range count {
		ta.InputHandler()(synth(key), setFocus)
	}
}

// resolvePrefix completes a pending g/f/F/t/T/r sequence with the rune ch.
func (v *vimHandler) resolvePrefix(ch rune, setFocus func(tview.Primitive)) bool {
	ta := v.editor.TextArea
	prefix := v.pending.prefix

	switch prefix {
	case 'r':
		after := ta.GetTextAfterCursor()
		if len(after) > 0 && after[0] != '\n' {
			_, oldSize := utf8.DecodeRuneInString(after)
			pos := ta.GetCursorByteOffset()
			ta.Replace(pos, pos+oldSize, string(ch))
			ta.InputHandler()(synth(tcell.KeyLeft), setFocus)
		}
	case 'g':
		if m, ok := gMotions[ch]; ok {
			v.runMotion('g', m)
		}
	default: // f/F/t/T — ch is the target rune
		forward := prefix == 'f' || prefix == 't'
		till := prefix == 't' || prefix == 'T'
		m := findMotion(ch, forward, till)
		if v.pending.operator != 0 {
			v.applyOperator(v.pending.operator, m, combineCount(v.pending.operatorCount, v.pending.motionCount))
		} else {
			text := ta.GetText()
			pos := ta.GetCursorByteOffset()
			if dest := m.run(text, pos, v.pending.motionCount); dest != pos {
				row, col := byteToRowCol(text, dest)
				ta.MoveCursorTo(row, col)
			}
		}
	}
	v.resetPending()
	return true
}

// handleCommand runs the non-motion normal-mode commands (mode switches, paste,
// line ops). count is already defaulted to >= 1.
func (v *vimHandler) handleCommand(ch rune, count int, setFocus func(tview.Primitive)) bool {
	ta := v.editor.TextArea

	switch ch {
	case '{':
		v.jumpToPrevBlankLine()
	case '}':
		v.jumpToNextBlankLine()

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

	case 's':
		for range count {
			v.deleteCharUnderCursor()
		}
		v.enterInsert()
	case 'S':
		v.applyOperator('c', motion{kind: mLinewise, run: lineDownStart}, count)
	case 'D':
		v.applyOperator('d', motions['$'], count)
	case 'C':
		v.applyOperator('c', motions['$'], count)
	case 'Y':
		v.applyOperator('y', motions['$'], count)
	case 'x':
		for range count {
			v.deleteCharUnderCursor()
		}

	case 'p':
		v.paste(true)
	case 'P':
		v.paste(false)

	case 'J':
		v.joinLines()
	case 'u':
		ta.InputHandler()(synth(tcell.KeyCtrlZ), setFocus)

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

	// applyMotion extends the selection's active end via a table motion.
	applyMotion := func(m motion) {
		v.selectionCursor = m.run(ta.GetText(), v.selectionCursor, v.pending.motionCount)
		v.applySelection()
	}

	// vertical moves keep column memory via synth keys, starting from the active end.
	clearAndMove := func(move func()) {
		ta.Select(v.selectionCursor, v.selectionCursor)
		move()
		v.selectionCursor = ta.GetCursorByteOffset()
		v.applySelection()
	}

	// Complete a pending g/f/F/t/T prefix.
	if v.pending.prefix != 0 {
		prefix := v.pending.prefix
		switch prefix {
		case 'g':
			if m, ok := gMotions[ch]; ok {
				applyMotion(m)
			}
		default:
			forward := prefix == 'f' || prefix == 't'
			till := prefix == 't' || prefix == 'T'
			applyMotion(findMotion(ch, forward, till))
		}
		v.resetPending()
		return true
	}

	// Count digits (0 extends a non-empty count, else it's the `0` motion).
	if (ch >= '1' && ch <= '9') || (ch == '0' && v.pending.motionCount > 0) {
		v.pending.motionCount = v.pending.motionCount*10 + int(ch-'0')
		v.notifyPending(v.pendingLabel())
		return true
	}

	if m, ok := motions[ch]; ok {
		applyMotion(m)
		v.resetPending()
		return true
	}

	switch ch {
	case 'h':
		// Bypass InputHandler: KeyLeft with an active selection jumps to the
		// selection start rather than moving left by one char.
		v.selectionCursor = charLeft(ta.GetText(), v.selectionCursor, max(v.pending.motionCount, 1))
		v.applySelection()
	case 'l':
		v.selectionCursor = charRight(ta.GetText(), v.selectionCursor, max(v.pending.motionCount, 1))
		v.applySelection()
	case 'j':
		for range max(v.pending.motionCount, 1) {
			clearAndMove(func() { ta.InputHandler()(synth(tcell.KeyDown), setFocus) })
		}
	case 'k':
		for range max(v.pending.motionCount, 1) {
			clearAndMove(func() { ta.InputHandler()(synth(tcell.KeyUp), setFocus) })
		}
	case 'g', 'f', 'F', 't', 'T':
		v.pending.prefix = ch
		v.notifyPending(v.pendingLabel())
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
		v.register = vimRegister{text: sel}
		v.enterNormal()
		v.editor.BeginYankHighlight(hlStart, hlEnd)
	case 'p', 'P':
		if text := util.Paste(); text != "" {
			_, start, end := ta.GetSelection()
			ta.Replace(start, end, text)
		}
		v.enterNormal()
	default:
		return true // consume all unrecognised runes — Visual mode doesn't type
	}
	v.resetPending()
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
		v.selectionCursor = strings.LastIndexByte(text[:newPos], '\n') + 1
		v.applyLineSelection()
	}

	if v.pending.prefix == 'g' {
		v.resetPending()
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
		if eol := strings.IndexByte(text[v.selectionCursor:], '\n'); eol >= 0 {
			v.selectionCursor += eol + 1
			ta.Select(v.selectionCursor, v.selectionCursor)
			ta.MoveCursorTo(ta.GetCurrentRow(), 0)
			v.applyLineSelection()
		}
	case 'k':
		text := ta.GetText()
		if v.selectionCursor > 0 {
			prev := strings.LastIndexByte(text[:v.selectionCursor-1], '\n')
			v.selectionCursor = prev + 1
			ta.Select(v.selectionCursor, v.selectionCursor)
			ta.MoveCursorTo(ta.GetCurrentRow(), 0)
			v.applyLineSelection()
		}
	case 'G':
		text := ta.GetText()
		lastRow := strings.Count(text, "\n")
		moveAndUpdate(func() { ta.MoveCursorTo(lastRow, 0) })
	case 'g':
		v.pending.prefix = 'g'
		v.notifyPending(v.pendingLabel())
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
		v.register = vimRegister{text: sel, linewise: true}
		v.enterNormal()
		v.editor.BeginYankHighlight(hlStart, hlEnd)
	case 'p', 'P':
		if text := util.Paste(); text != "" {
			_, start, end := ta.GetSelection()
			ta.Replace(start, end, text)
		}
		v.enterNormal()
	case 'v':
		// Switch to char-visual at the current cursor position.
		v.mode = vimVisual
		v.selectionAnchor = v.selectionCursor
		v.applySelection()
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
