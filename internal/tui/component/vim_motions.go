package component

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// runeClass returns the character class used by vim word motions.
// 0 = blank, 1 = keyword (letter/digit/_), 2 = other (punctuation etc.)
// When big is true (WORD motions), any non-blank maps to 1.
func runeClass(r rune, big bool) int {
	if unicode.IsSpace(r) {
		return 0
	}
	if big {
		return 1
	}
	if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
		return 1
	}
	return 2
}

// wordForward implements the vim `w`/`W` motion: advance to the start of the
// next word. An empty line counts as a stop.
func wordForward(text string, pos int, big bool) int {
	n := len(text)
	if pos >= n {
		return n
	}

	r, _ := utf8.DecodeRuneInString(text[pos:])
	startClass := runeClass(r, big)

	i := pos
	var size int
	// Skip the current class run (unless we're on a blank).
	if startClass != 0 {
		for i < n {
			r, size = utf8.DecodeRuneInString(text[i:])
			if runeClass(r, big) != startClass {
				break
			}
			i += size
		}
	}

	// Skip blanks — but stop at each empty line boundary.
	for i < n {
		r, size = utf8.DecodeRuneInString(text[i:])
		if r == '\n' {
			next := i + size
			if next >= n || text[next] == '\n' {
				return i + size
			}
			i += size
			continue
		}
		if !unicode.IsSpace(r) {
			break
		}
		i += size
	}
	return i
}

// wordForwardEnd implements the vim `e`/`E` motion: advance to the last char
// of the next word. Always moves at least one character.
func wordForwardEnd(text string, pos int, big bool) int {
	n := len(text)
	if pos >= n {
		return n
	}

	_, size := utf8.DecodeRuneInString(text[pos:])
	i := pos + size // Advance at least one char.

	for i < n {
		r, size := utf8.DecodeRuneInString(text[i:])
		if !unicode.IsSpace(r) {
			break
		}
		i += size
	}
	if i >= n {
		return n - utf8.RuneLen(lastRune(text))
	}

	// Consume the word; stop just before the class changes.
	r, _ := utf8.DecodeRuneInString(text[i:])
	cls := runeClass(r, big)
	last := i
	for i < n {
		r, size = utf8.DecodeRuneInString(text[i:])
		if runeClass(r, big) != cls {
			break
		}
		last = i
		i += size
	}
	return last
}

// wordBackward implements the vim `b`/`B` motion: move to the start of the
// current or previous word.
func wordBackward(text string, pos int, big bool) int {
	if pos <= 0 {
		return 0
	}

	_, size := utf8.DecodeLastRuneInString(text[:pos])
	i := pos - size // Step back one char (helpful if on edge of the word)

	for i > 0 {
		r, _ := utf8.DecodeRuneInString(text[i:])
		if !unicode.IsSpace(r) {
			break
		}
		_, size = utf8.DecodeLastRuneInString(text[:i])
		i -= size
	}

	r, _ := utf8.DecodeRuneInString(text[i:])
	cls := runeClass(r, big)
	for i > 0 {
		_, size = utf8.DecodeLastRuneInString(text[:i])
		r, _ = utf8.DecodeRuneInString(text[i-size:])
		if runeClass(r, big) != cls {
			break
		}
		i -= size
	}
	return i
}

// wordBackwardEnd implements the vim `ge`/`gE` motion: move back to the end
// (last char) of the previous word.
func wordBackwardEnd(text string, pos int, big bool) int {
	if pos <= 0 {
		return 0
	}

	_, size := utf8.DecodeLastRuneInString(text[:pos])
	i := pos - size // Step back one char.

	for i > 0 {
		r, _ := utf8.DecodeRuneInString(text[i:])
		if !unicode.IsSpace(r) {
			break
		}
		_, size = utf8.DecodeLastRuneInString(text[:i])
		i -= size
	}
	return i
}

func lastRune(s string) rune {
	r, _ := utf8.DecodeLastRuneInString(s)
	return r
}

type motionKind int

const (
	mExclusive motionKind = iota // [pos, dest)
	mInclusive                   // [pos, dest] — dest char included
	mLinewise                    // both ends expanded to whole lines incl. \n
)

type motion struct {
	// run returns the destination byte offset. count == 0 means "no count given";
	// each run applies its own default (most clamp to 1; G/gg treat 0 as last/first).
	run  func(text string, pos, count int) int
	kind motionKind
}

// repeatMotion applies a single-step motion fn count times (loop-outside counts).
func repeatMotion(fn func(string, int, bool) int, text string, pos, count int, big bool) int {
	if count < 1 {
		count = 1
	}
	for range count {
		next := fn(text, pos, big)
		if next == pos {
			break
		}
		pos = next
	}
	return pos
}

func lineStartAt(text string, pos int) int {
	return strings.LastIndexByte(text[:pos], '\n') + 1
}

// lineContentEnd returns the offset of the newline ending the line at pos (or
// len(text)) — the line's content end, excluding the newline itself.
func lineContentEnd(text string, pos int) int {
	if nl := strings.IndexByte(text[pos:], '\n'); nl >= 0 {
		return pos + nl
	}
	return len(text)
}

// lineEndAfterNL returns the offset just past the newline ending the line at pos
// (or len(text) for the last line).
func lineEndAfterNL(text string, pos int) int {
	if pos > len(text) {
		pos = len(text)
	}
	nl := strings.IndexByte(text[pos:], '\n')
	if nl < 0 {
		return len(text)
	}
	return pos + nl + 1
}

func firstNonBlankOffset(text string, pos int) int {
	ls := lineStartAt(text, pos)
	i := ls
	for i < len(text) && (text[i] == ' ' || text[i] == '\t') {
		i++
	}
	if i >= len(text) || text[i] == '\n' {
		return ls // blank line — stay at col 0
	}
	return i
}

// endOfLineOffset returns the offset of the last char on the line (count lines
// down for count>1), matching vim `$` which lands on — not past — the last char.
func endOfLineOffset(text string, pos, count int) int {
	for range max(count, 1) - 1 {
		pos = lineEndAfterNL(text, pos)
	}
	nl := strings.IndexByte(text[pos:], '\n')
	var eol int
	if nl < 0 {
		eol = len(text)
	} else {
		eol = pos + nl
	}
	ls := lineStartAt(text, pos)
	if eol > ls {
		_, size := utf8.DecodeLastRuneInString(text[:eol])
		return eol - size
	}
	return eol
}

// lineDownStart moves to the start of the line max(count,1)-1 lines below — the
// motion behind doubled operators (dd/cc/yy and 2dd).
func lineDownStart(text string, pos, count int) int {
	for range max(count, 1) - 1 {
		pos = lineEndAfterNL(text, pos)
	}
	return pos
}

// lineDownN / lineUpN move max(count,1) lines for operator j/k (dj deletes the
// current line plus the next).
func lineDownN(text string, pos, count int) int {
	for range max(count, 1) {
		next := lineEndAfterNL(text, pos)
		if next == pos {
			break
		}
		pos = next
	}
	return pos
}

func lineUpN(text string, pos, count int) int {
	for range max(count, 1) {
		ls := lineStartAt(text, pos)
		if ls == 0 {
			pos = 0
			break
		}
		pos = lineStartAt(text, ls-1)
	}
	return pos
}

// gotoLine returns the start offset of the 1-based line (clamped to the last
// line). count 0 from G means last line; from gg means first.
func gotoLine(text string, line int) int {
	if line <= 1 {
		return 0
	}
	off := 0
	for range line - 1 {
		nl := strings.IndexByte(text[off:], '\n')
		if nl < 0 {
			return off
		}
		off += nl + 1
	}
	return off
}

func lastLineStart(text string) int {
	return strings.LastIndexByte(text, '\n') + 1
}

// findOnce returns the offset of the next/previous occurrence of target on the
// current line, or -1. Forward search stops at a newline.
func findOnce(text string, pos int, target rune, forward bool) int {
	if forward {
		i := pos
		if i < len(text) {
			_, size := utf8.DecodeRuneInString(text[i:])
			i += size
		}
		for i < len(text) {
			r, size := utf8.DecodeRuneInString(text[i:])
			if r == '\n' {
				return -1
			}
			if r == target {
				return i
			}
			i += size
		}
		return -1
	}
	ls := lineStartAt(text, pos)
	i := pos
	for i > ls {
		_, size := utf8.DecodeLastRuneInString(text[:i])
		i -= size
		if r, _ := utf8.DecodeRuneInString(text[i:]); r == target {
			return i
		}
	}
	return -1
}

// findCharOffset implements f/F/t/T: the count-th occurrence of target, with
// `till` backing off one rune toward the origin. Returns -1 if not found.
func findCharOffset(text string, pos int, target rune, forward, till bool, count int) int {
	cur := pos
	for range max(count, 1) {
		next := findOnce(text, cur, target, forward)
		if next < 0 {
			return -1
		}
		cur = next
	}
	if !till {
		return cur
	}
	if forward {
		_, size := utf8.DecodeLastRuneInString(text[:cur])
		return cur - size
	}
	_, size := utf8.DecodeRuneInString(text[cur:])
	dest := cur + size
	if dest >= pos {
		return -1
	}
	return dest
}

func findMotion(target rune, forward, till bool) motion {
	kind := mInclusive
	if !forward {
		kind = mExclusive
	}
	return motion{kind: kind, run: func(t string, p, c int) int {
		if dest := findCharOffset(t, p, target, forward, till, c); dest >= 0 {
			return dest
		}
		return p
	}}
}

// rangeFor turns a motion destination into the [start, end) byte range an
// operator deletes/yanks, per the motion kind.
func rangeFor(text string, pos, dest int, kind motionKind) (start, end int) {
	start, end = pos, dest
	if start > end {
		start, end = end, start
	}
	switch kind {
	case mInclusive:
		if end < len(text) {
			_, size := utf8.DecodeRuneInString(text[end:])
			end += size
		}
	case mLinewise:
		start = lineStartAt(text, start)
		end = lineEndAfterNL(text, end)
		if end == len(text) && start > 0 && (len(text) == 0 || text[len(text)-1] != '\n') {
			start-- // last line w/o trailing newline: eat the preceding \n (dd behaviour)
		}
	}
	return
}

// operatorRange turns a motion destination into the [start,end) range operator
// op deletes/yanks. It is rangeFor plus the cc/S exception: a linewise change
// clears the line text but keeps the (now empty) line, so the trailing newline
// is preserved.
func operatorRange(text string, pos, dest int, op rune, kind motionKind) (start, end int) {
	if op == 'c' && kind == mLinewise {
		return lineStartAt(text, min(pos, dest)), lineContentEnd(text, max(pos, dest))
	}
	return rangeFor(text, pos, dest, kind)
}

// clampWordMotion stops a forward word motion at the line's newline so dw/yw
// don't pull the next line up (vim quirk; bare `w` movement still crosses).
func clampWordMotion(m motion) motion {
	inner := m.run
	return motion{kind: m.kind, run: func(t string, p, c int) int {
		dest := inner(t, p, c)
		if nl := strings.IndexByte(t[p:], '\n'); nl >= 0 && p+nl < dest {
			return p + nl
		}
		return dest
	}}
}

func charLeft(text string, pos, count int) int {
	ls := lineStartAt(text, pos)
	for range max(count, 1) {
		if pos <= ls {
			break
		}
		_, size := utf8.DecodeLastRuneInString(text[:pos])
		pos -= size
	}
	return pos
}

func charRight(text string, pos, count int) int {
	for range max(count, 1) {
		if pos >= len(text) || text[pos] == '\n' {
			break
		}
		_, size := utf8.DecodeRuneInString(text[pos:])
		pos += size
	}
	return pos
}

var motions = map[rune]motion{
	'w': {kind: mExclusive, run: func(t string, p, c int) int { return repeatMotion(wordForward, t, p, c, false) }},
	'W': {kind: mExclusive, run: func(t string, p, c int) int { return repeatMotion(wordForward, t, p, c, true) }},
	'e': {kind: mInclusive, run: func(t string, p, c int) int { return repeatMotion(wordForwardEnd, t, p, c, false) }},
	'E': {kind: mInclusive, run: func(t string, p, c int) int { return repeatMotion(wordForwardEnd, t, p, c, true) }},
	'b': {kind: mExclusive, run: func(t string, p, c int) int { return repeatMotion(wordBackward, t, p, c, false) }},
	'B': {kind: mExclusive, run: func(t string, p, c int) int { return repeatMotion(wordBackward, t, p, c, true) }},
	'0': {kind: mExclusive, run: func(t string, p, c int) int { return lineStartAt(t, p) }},
	'^': {kind: mExclusive, run: func(t string, p, c int) int { return firstNonBlankOffset(t, p) }},
	'$': {kind: mInclusive, run: endOfLineOffset},
	'G': {kind: mLinewise, run: func(t string, p, c int) int {
		if c == 0 {
			return lastLineStart(t)
		}
		return gotoLine(t, c)
	}},
}

var simpleMotions = map[rune]motion{
	'h': {kind: mExclusive, run: charLeft},
	'l': {kind: mExclusive, run: charRight},
	'j': {kind: mLinewise, run: lineDownN},
	'k': {kind: mLinewise, run: lineUpN},
}

var gMotions = map[rune]motion{
	'g': {kind: mLinewise, run: func(t string, p, c int) int { return gotoLine(t, c) }},
	'e': {kind: mExclusive, run: func(t string, p, c int) int { return repeatMotion(wordBackwardEnd, t, p, c, false) }},
	'E': {kind: mExclusive, run: func(t string, p, c int) int { return repeatMotion(wordBackwardEnd, t, p, c, true) }},
}
