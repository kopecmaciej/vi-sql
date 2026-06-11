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
	// inFlightEvent marks the event currently traversing the wrapper chain so
	// deeper wrappers reuse the pending state instead of re-absorbing it.
	inFlightEvent    *tcell.EventKey
	OnPendingChanged func(string)
	// SequencesDisabled is set for text inputs and vim insert mode
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

// Reset clears any pending sequence prefix. Call on focus change or mode switch.
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

// WrapInputCapture wraps tview InputCapture handler so sequences (e.g. `gg`,
// `yrj`) are absorbed first. Non-rune events and unmatched runes propagate
// further down the wrapper chain (app → main → data → ...).
//
// Prefix extension (e.g. absorbing `y` then `r` toward `yrj`) is handled here
// before inner is called. Inner is only involved when the full candidate
// sequence needs to be matched by a k.Match call.
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
			// Escape cancels a pending prefix without forwarding the event.
			if cs.pending != "" && ev.Key() == tcell.KeyEsc {
				cs.Reset()
				return nil
			}
			cs.Reset()
			return inner(ev)
		}

		candidate := cs.pending + string(ev.Rune())

		// If candidate is a proper prefix of some sequence, absorb and extend.
		if _, ok := cs.sequencePrefixes[candidate]; ok {
			cs.pending = candidate
			cs.notifyPending(candidate)
			return nil
		}

		// If there is a pending prefix, let inner try to complete the sequence.
		if cs.pending != "" {
			if cs.inFlightEvent == nil {
				cs.inFlightEvent = ev
			}
			result := inner(ev)
			if result == nil {
				// Sequence consumed by a matching k.Match call.
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
