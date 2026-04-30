package core

import (
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
)

// ChordResolver resolves 2-rune vim-style chords (e.g. "gg", "gd").
// Unknown chords (registered prefix + unregistered second rune) silently consume
// both runes.
type ChordResolver struct {
	prefixes  map[rune]map[rune]func()
	pending   rune
	OnPending func(rune) // called on every pending-state change; 0 means cleared
}

func NewChordResolver() *ChordResolver {
	return &ChordResolver{prefixes: make(map[rune]map[rune]func())}
}

// Register binds a 2-rune chord (e.g. "gg") to fn.
func (c *ChordResolver) Register(chord string, fn func()) {
	if utf8.RuneCountInString(chord) != 2 {
		return
	}
	runes := []rune(chord)
	if c.prefixes[runes[0]] == nil {
		c.prefixes[runes[0]] = make(map[rune]func())
	}
	c.prefixes[runes[0]][runes[1]] = fn
}

// Feed processes one rune from an input event. Returns true when rune
// was absorbed: either stored as a pending prefix, or completing/discarding a chord.
func (c *ChordResolver) Feed(r rune) bool {
	if c.pending != 0 {
		motions := c.prefixes[c.pending]
		c.pending = 0
		if c.OnPending != nil {
			c.OnPending(0)
		}
		if fn, ok := motions[r]; ok {
			fn()
		}
		return true
	}
	if _, isPrefix := c.prefixes[r]; isPrefix {
		c.pending = r
		if c.OnPending != nil {
			c.OnPending(r)
		}
		return true
	}
	return false
}

// Reset clears any pending prefix (call on mode switches or Escape).
func (c *ChordResolver) Reset() {
	if c.pending != 0 {
		c.pending = 0
		if c.OnPending != nil {
			c.OnPending(0)
		}
	}
}

// RegisterChords registers every chord string from chords into cr with fn.
func RegisterChords(chords []string, cr *ChordResolver, fn func()) {
	for _, ch := range chords {
		cr.Register(ch, fn)
	}
}

// WithChords either consume event in ChordResolver or leaves it to the inner function intact
// while also resetting pending rune if it was non-rune event
func WithChords(cr *ChordResolver, inner func(*tcell.EventKey) *tcell.EventKey) func(*tcell.EventKey) *tcell.EventKey {
	return func(ev *tcell.EventKey) *tcell.EventKey {
		if cr != nil {
			if ev.Key() == tcell.KeyRune {
				if cr.Feed(ev.Rune()) {
					return nil
				}
			} else {
				cr.Reset()
			}
		}
		return inner(ev)
	}
}
