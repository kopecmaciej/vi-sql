package config

import (
	"reflect"
	"slices"

	"github.com/gdamore/tcell/v2"
)

// chordState holds the vim-mode chord prefix state machine. Embedded
// anonymously in KeyBindings so its methods are promoted.
type chordState struct {
	vimMode          bool
	pending          rune
	chordPrefixes    map[rune]struct{}
	OnPendingChanged func(rune) `yaml:"-"`
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
func (cs *chordState) WrapInputCapture(inner func(*tcell.EventKey) *tcell.EventKey) func(*tcell.EventKey) *tcell.EventKey {
	return func(ev *tcell.EventKey) *tcell.EventKey {
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

// Match reports whether ev should fire configKey's handler. Chord-aware
// replacement for Contains: when a chord prefix is pending, matches the second
// rune against configKey.Chords; otherwise delegates to the Keys/Runes match.
// Must be used inside a handler wrapped with WrapInputCapture.
func (kb *KeyBindings) Match(configKey Key, ev *tcell.EventKey) bool {
	if ev.Key() == tcell.KeyRune && kb.pending != 0 {
		chord := string([]rune{kb.pending, ev.Rune()})
		return slices.Contains(configKey.Chords, chord)
	}
	return kb.Contains(configKey, ev.Name())
}

// buildChordPrefixes scans every Key in the bindings and records the first
// rune of every chord so WrapInputCapture knows which runes to absorb.
// Vim mode only — leaves the map nil/empty otherwise.
func (kb *KeyBindings) buildChordPrefixes() {
	kb.chordPrefixes = nil
	if !kb.vimMode {
		return
	}
	prefixes := make(map[rune]struct{})
	v := reflect.ValueOf(*kb)
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() != reflect.Struct {
			continue
		}
		for _, k := range extractKeysFromStruct(f) {
			for _, ch := range k.Chords {
				if r, _ := utf8FirstRune(ch); r != 0 {
					prefixes[r] = struct{}{}
				}
			}
		}
	}
	kb.chordPrefixes = prefixes
}

func utf8FirstRune(s string) (rune, int) {
	for _, r := range s {
		return r, 1
	}
	return 0, 0
}
