package config

import (
	"github.com/gdamore/tcell/v2"
)

// sequenceState holds the vim-mode sequence prefix state machine. Embedded
// anonymously in KeyBindings so its methods are promoted.
type sequenceState struct {
	vimMode          bool
	pending          string
	sequencePrefixes map[string]struct{}
	// inFlightEvent marks the event currently traversing wrapper chain that was not
	// consumed on given level, so deeper wrappers can reuse the pending state
	inFlightEvent     *tcell.EventKey
	OnPendingChanged  func(string)
	SequencesDisabled func() bool
}

func (cs *sequenceState) HasPending() bool   { return cs.pending != "" }
func (cs *sequenceState) GetPending() string { return cs.pending }

func (cs *sequenceState) IsSequencePrefix(s string) bool {
	_, ok := cs.sequencePrefixes[s]
	return ok
}

func (cs *sequenceState) SetPending(s string) {
	cs.pending = s
	cs.notifyPending(s)
}

func (cs *sequenceState) Reset() {
	cs.inFlightEvent = nil
	if cs.pending != "" {
		cs.pending = ""
		cs.notifyPending("")
	}
}

func (cs *sequenceState) notifyPending(s string) {
	if cs.OnPendingChanged != nil {
		cs.OnPendingChanged(s)
	}
}

// WrapInputCapture turns a component's InputCapture into the vim sequence state
// machine, so rune that extends sequences like `g` for `gg` or `yr` for `yrj`
// could be absorbed first. If key unmatched it's propagated further down the wrapper chain.
// inner returning nil means it consumed the event (sequence matched).
func (cs *sequenceState) WrapInputCapture(inner func(*tcell.EventKey) *tcell.EventKey) func(*tcell.EventKey) *tcell.EventKey {
	return func(ev *tcell.EventKey) *tcell.EventKey {
		// Stale in-flight event: previous sequence found no match.
		if cs.inFlightEvent != nil && cs.inFlightEvent != ev {
			cs.inFlightEvent = nil
			if cs.pending != "" {
				cs.pending = ""
				cs.notifyPending("")
			}
		}

		if cs.SequencesDisabled != nil && cs.SequencesDisabled() {
			cs.Reset()
			return inner(ev)
		}
		if ev.Key() != tcell.KeyRune {
			if cs.pending != "" && ev.Key() == tcell.KeyEsc {
				cs.Reset()
				return nil
			}
			cs.Reset()
			return inner(ev)
		}

		candidate := cs.pending + string(ev.Rune())

		if _, ok := cs.sequencePrefixes[candidate]; ok {
			cs.pending = candidate
			cs.notifyPending(candidate)
			return nil
		}

		if cs.pending != "" {
			if cs.inFlightEvent == nil {
				cs.inFlightEvent = ev
			}
			result := inner(ev)
			if result == nil {
				cs.pending = ""
				cs.inFlightEvent = nil
				cs.notifyPending("")
				return nil
			}
			return result
		}

		return inner(ev)
	}
}
