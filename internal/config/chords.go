package config

import (
	"github.com/gdamore/tcell/v2"
)

// chordState holds the vim-mode chord prefix state machine. Embedded
// anonymously in KeyBindings so its methods are promoted.
type chordState struct {
	vimMode          bool
	pending          rune
	chordPrefixes    map[rune]struct{}
	OnPendingChanged func(rune)
	// ChordsDisabled is set for text inputs and vim insert mode where
	// every rune must reach the inner handler verbatim
	ChordsDisabled func() bool
}

func (cs *chordState) HasPending() bool { return cs.pending != 0 }

func (cs *chordState) IsChordPrefix(r rune) bool {
	_, ok := cs.chordPrefixes[r]
	return ok
}

func (cs *chordState) SetPending(r rune) {
	cs.pending = r
	cs.notifyPending(r)
}

// Reset clears any pending chord prefix. Call on focus change or mode switch.
func (cs *chordState) Reset() {
	if cs.pending != 0 {
		cs.pending = 0
		cs.notifyPending(0)
	}
}

func (cs *chordState) notifyPending(r rune) {
	if cs.OnPendingChanged != nil {
		cs.OnPendingChanged(r)
	}
}

// WrapInputCapture wraps an InputCapture handler so that chord prefix runes are
// absorbed and the second rune is delivered to inner — where each k.Match arm
// can recognize its own chord. Non-rune events reset pending. No-op in normal
// mode (chordPrefixes is empty there).
//
// When SkipAbsorb is set and returns true, chord-prefix handling is bypassed
// entirely so runes pass through to inner verbatim — used inside text inputs
// and vim insert mode.

// WrapInputCapture wraps InputCapture handler to absorb runes by chordState first
// so that chords like `gg` could be properly processed.
func (cs *chordState) WrapInputCapture(inner func(*tcell.EventKey) *tcell.EventKey) func(*tcell.EventKey) *tcell.EventKey {
	return func(ev *tcell.EventKey) *tcell.EventKey {
		if cs.ChordsDisabled != nil && cs.ChordsDisabled() {
			cs.Reset()
			return inner(ev)
		}
		if ev.Key() != tcell.KeyRune {
			cs.Reset()
			return inner(ev)
		}
		if cs.pending != 0 {
			_ = inner(ev)
			cs.pending = 0
			cs.notifyPending(0)
			return nil
		}
		if _, ok := cs.chordPrefixes[ev.Rune()]; ok {
			cs.pending = ev.Rune()
			cs.notifyPending(ev.Rune())
			return nil
		}
		return inner(ev)
	}
}
