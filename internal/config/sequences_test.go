package config

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
)

func TestHasPending(t *testing.T) {
	cs := sequenceState{}
	assert.False(t, cs.HasPending())

	cs.pending = "g"
	assert.True(t, cs.HasPending())
}

func TestIsSequencePrefix(t *testing.T) {
	cs := sequenceState{sequencePrefixes: map[string]struct{}{"g": {}}}
	assert.True(t, cs.IsSequencePrefix("g"))
	assert.False(t, cs.IsSequencePrefix("x"))
}

func TestSetPending_FiresNotification(t *testing.T) {
	var notified []string
	cs := sequenceState{OnPendingChanged: func(s string) { notified = append(notified, s) }}

	cs.SetPending("g")
	assert.Equal(t, "g", cs.pending)
	assert.Equal(t, []string{"g"}, notified)
}

func TestReset_ClearsPendingAndFires(t *testing.T) {
	var notified []string
	cs := sequenceState{
		pending:          "g",
		OnPendingChanged: func(s string) { notified = append(notified, s) },
	}

	cs.Reset()
	assert.Equal(t, "", cs.pending)
	assert.Equal(t, []string{""}, notified)
}

func TestReset_ClearsInFlightEvent(t *testing.T) {
	ev := mkRune('g')
	cs := sequenceState{pending: "g", inFlightEvent: ev}

	cs.Reset()
	assert.Nil(t, cs.inFlightEvent, "Reset must clear inFlightEvent")
}

func TestWrapInputCapture_UnmatchedSecondRuneClearsOnNextEvent(t *testing.T) {
	kb := &KeyBindings{sequenceState: sequenceState{
		vimMode:          true,
		sequencePrefixes: map[string]struct{}{"g": {}},
		pending:          "g",
	}}
	var notified []string
	kb.OnPendingChanged = func(s string) { notified = append(notified, s) }

	fn := kb.WrapInputCapture(func(ev *tcell.EventKey) *tcell.EventKey { return ev })

	out := fn(mkRune('q'))
	assert.NotNil(t, out, "unmatched final rune must pass through")
	assert.Equal(t, "g", kb.pending, "pending must remain after unmatched final rune")
	assert.NotNil(t, kb.inFlightEvent, "inFlightEvent must be set so deeper wrappers reuse pending")
	assert.Empty(t, notified, "no notification yet — pending unchanged")

	out = fn(mkRune('x'))
	assert.NotNil(t, out)
	assert.Equal(t, "", kb.pending, "stale pending cleared on next event")
	assert.Nil(t, kb.inFlightEvent)
	assert.Equal(t, []string{""}, notified, "OnPendingChanged(\"\") fires when stale pending is cleared")
}

func TestWrapInputCapture_SameEventReusedDoesNotResetPending(t *testing.T) {
	ev := mkRune('g')
	kb := &KeyBindings{sequenceState: sequenceState{
		vimMode:          true,
		sequencePrefixes: map[string]struct{}{"g": {}},
		pending:          "g",
		inFlightEvent:    ev,
	}}

	innerSawPending := ""
	fn := kb.WrapInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		innerSawPending = kb.pending
		return nil
	})

	out := fn(ev)
	assert.Nil(t, out, "matched final rune must be consumed")
	assert.Equal(t, "g", innerSawPending, "inner must observe pending intact (not reset by stale check)")
	assert.Equal(t, "", kb.pending, "pending cleared after successful sequence match")
	assert.Nil(t, kb.inFlightEvent)
}

func TestWrapInputCapture_EscapeCancelsPendingAndConsumes(t *testing.T) {
	kb := &KeyBindings{sequenceState: sequenceState{
		vimMode:          true,
		sequencePrefixes: map[string]struct{}{"g": {}},
		pending:          "g",
	}}
	var notified []string
	kb.OnPendingChanged = func(s string) { notified = append(notified, s) }

	innerCalled := false
	fn := kb.WrapInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		innerCalled = true
		return ev
	})

	out := fn(mkKey(tcell.KeyEsc))
	assert.Nil(t, out, "Escape while pending must be consumed, not forwarded")
	assert.False(t, innerCalled, "inner handler must not be called")
	assert.Equal(t, "", kb.pending, "pending must be cleared")
	assert.Equal(t, []string{""}, notified, "OnPendingChanged(\"\") must fire")
}

func TestWrapInputCapture_EscapeWithNoPendingForwards(t *testing.T) {
	kb := &KeyBindings{sequenceState: sequenceState{
		vimMode:          true,
		sequencePrefixes: map[string]struct{}{"g": {}},
	}}

	innerCalled := false
	fn := kb.WrapInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		innerCalled = true
		return ev
	})

	out := fn(mkKey(tcell.KeyEsc))
	assert.NotNil(t, out, "Escape with no pending must be forwarded")
	assert.True(t, innerCalled)
}

func TestWrapInputCapture_ThreeRuneMatch(t *testing.T) {
	kb := &KeyBindings{sequenceState: sequenceState{
		vimMode:          true,
		sequencePrefixes: map[string]struct{}{"y": {}, "yr": {}},
	}}
	var notified []string
	kb.OnPendingChanged = func(s string) { notified = append(notified, s) }

	var innerReceivedRune rune
	fn := kb.WrapInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyRune {
			innerReceivedRune = ev.Rune()
		}
		return nil
	})

	out := fn(mkRune('y'))
	assert.Nil(t, out)
	assert.Equal(t, "y", kb.pending)
	assert.Equal(t, rune(0), innerReceivedRune, "inner must not be called for prefix extension")

	out = fn(mkRune('r'))
	assert.Nil(t, out)
	assert.Equal(t, "yr", kb.pending)
	assert.Equal(t, rune(0), innerReceivedRune, "inner must not be called for prefix extension")

	out = fn(mkRune('j'))
	assert.Nil(t, out, "third rune consumed by inner match")
	assert.Equal(t, 'j', innerReceivedRune)
	assert.Equal(t, "", kb.pending, "pending cleared after sequence complete")
	assert.Equal(t, []string{"y", "yr", ""}, notified)
}

func TestWrapInputCapture_EscapeMidThreeRuneSequence(t *testing.T) {
	kb := &KeyBindings{sequenceState: sequenceState{
		vimMode:          true,
		sequencePrefixes: map[string]struct{}{"y": {}, "yr": {}},
		pending:          "yr",
	}}
	var notified []string
	kb.OnPendingChanged = func(s string) { notified = append(notified, s) }

	innerCalled := false
	fn := kb.WrapInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		innerCalled = true
		return ev
	})

	out := fn(mkKey(tcell.KeyEsc))
	assert.Nil(t, out, "Esc mid-sequence must be consumed")
	assert.False(t, innerCalled)
	assert.Equal(t, "", kb.pending)
	assert.Equal(t, []string{""}, notified)
}
