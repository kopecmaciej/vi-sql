package core

import "unicode/utf8"

// ChordResolver resolves 2-rune vim-style chords (e.g. "gg", "gd").
// Call Feed from an input handler; it returns true when the rune was
// absorbed — either stored as a pending prefix or completing/discarding a chord.
// Unknown chords (registered prefix + unregistered second rune) silently consume
// both runes, matching vim's behaviour.
type ChordResolver struct {
	prefixes map[rune]map[rune]func()
	pending  rune
}

func NewChordResolver() *ChordResolver {
	return &ChordResolver{prefixes: make(map[rune]map[rune]func())}
}

// Register binds a 2-rune chord (e.g. "gg") to fn. Silently ignored if chord
// is not exactly 2 runes.
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

// Feed processes one rune from an input event. Returns true when the rune was
// absorbed: either stored as a pending prefix, or completing/discarding a chord.
func (c *ChordResolver) Feed(r rune) bool {
	if c.pending != 0 {
		motions := c.prefixes[c.pending]
		c.pending = 0
		if fn, ok := motions[r]; ok {
			fn()
		}
		return true
	}
	if _, isPrefix := c.prefixes[r]; isPrefix {
		c.pending = r
		return true
	}
	return false
}

// Reset clears any pending prefix (call on mode switches or Escape).
func (c *ChordResolver) Reset() {
	c.pending = 0
}
