package config

import (
	"github.com/gdamore/tcell/v2"
)

// chordState holds the vim-mode chord prefix state machine. Embedded
// anonymously in KeyBindings so its methods are promoted.
type chordState struct {
	vimMode       bool
	pending       rune
	chordPrefixes map[rune]struct{}
	// chordEvent identifies the event currently being dispatched as a chord
	// completion attempt. tview routes input through the focus chain
	// (Pages → page → focused primitive), so several wrappers in that chain
	// see the same event. Tracking the event lets the first wrapper mark the
	// dispatch as in-progress and lets later wrappers in the same chain reuse
	// the pending state without re-entering the absorption logic.
	chordEvent       *tcell.EventKey
	OnPendingChanged func(rune)
	// ChordsDisabled is set for text inputs and vim insert mode where
	// every rune must reach the inner handler verbatim.
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
	cs.chordEvent = nil
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
// tview dispatches input through the entire focus chain (root → page → ... →
// focused primitive), invoking each level's inputCapture. When the chord
// completion rune arrives, the wrapper at the topmost level cannot know whether
// the chord belongs to its own handler or to a deeper one. So if its inner
// doesn't match, the wrapper propagates the event (returning it instead of
// nil) so deeper wrappers in the chain can also try matching. Pending state is
// kept until some wrapper matches; if none does, the next event triggers a
// stale-state cleanup.
//
// When ChordsDisabled returns true, chord handling is bypassed entirely so
// runes pass through to inner verbatim — used inside text inputs and vim
// insert mode.
func (cs *chordState) WrapInputCapture(inner func(*tcell.EventKey) *tcell.EventKey) func(*tcell.EventKey) *tcell.EventKey {
	return func(ev *tcell.EventKey) *tcell.EventKey {
		// Stale-state cleanup: a previous chord-completion attempt didn't
		// match anywhere, leaving pending set. Clear it before processing
		// this fresh event.
		if cs.chordEvent != nil && cs.chordEvent != ev {
			cs.chordEvent = nil
			if cs.pending != 0 {
				cs.pending = 0
				cs.notifyPending(0)
			}
		}

		if cs.ChordsDisabled != nil && cs.ChordsDisabled() {
			cs.Reset()
			return inner(ev)
		}
		if ev.Key() != tcell.KeyRune {
			cs.Reset()
			return inner(ev)
		}
		if cs.pending != 0 {
			// Mark this event as the in-progress completion attempt the
			// first time we see it; deeper wrappers in the chain will see
			// the same chordEvent and skip re-marking.
			if cs.chordEvent == nil {
				cs.chordEvent = ev
			}
			result := inner(ev)
			if result == nil {
				// Inner matched and consumed the chord.
				cs.pending = 0
				cs.chordEvent = nil
				cs.notifyPending(0)
				return nil
			}
			// Inner didn't match. Propagate so a deeper wrapper can try.
			// Pending stays set; Match in the deeper inner uses it for
			// chord matching.
			return result
		}
		if _, ok := cs.chordPrefixes[ev.Rune()]; ok {
			cs.pending = ev.Rune()
			cs.notifyPending(ev.Rune())
			return nil
		}
		return inner(ev)
	}
}
