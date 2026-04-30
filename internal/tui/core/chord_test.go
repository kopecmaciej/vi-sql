package core

import (
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestChordPrefixConsumed(t *testing.T) {
	cr := NewChordResolver()
	fired := false
	cr.Register("gg", func() { fired = true })

	if !cr.Feed('g') {
		t.Fatal("prefix rune should be consumed")
	}
	if fired {
		t.Fatal("handler should not fire on prefix alone")
	}
}

func TestChordMatchDispatch(t *testing.T) {
	cr := NewChordResolver()
	fired := false
	cr.Register("gg", func() { fired = true })

	cr.Feed('g')
	if !cr.Feed('g') {
		t.Fatal("second rune should be consumed")
	}
	if !fired {
		t.Fatal("handler should fire on match")
	}
}

func TestChordUnknownSecondConsumed(t *testing.T) {
	cr := NewChordResolver()
	cr.Register("gg", func() {})

	cr.Feed('g')
	if !cr.Feed('x') {
		t.Fatal("second rune after known prefix should always be consumed")
	}
}

func TestChordUnknownPrefixPassThrough(t *testing.T) {
	cr := NewChordResolver()
	cr.Register("gg", func() {})

	if cr.Feed('x') {
		t.Fatal("non-prefix rune should not be consumed")
	}
}

func TestChordReset(t *testing.T) {
	cr := NewChordResolver()
	fired := false
	cr.Register("gg", func() { fired = true })

	cr.Feed('g')
	cr.Reset()
	// After reset, 'g' acts as a fresh prefix, not the second rune.
	if !cr.Feed('g') {
		t.Fatal("after reset, prefix rune should be consumed again")
	}
	if fired {
		t.Fatal("handler should not fire — reset cleared the pending prefix")
	}
}

func TestChordMultipleBindings(t *testing.T) {
	cr := NewChordResolver()
	var result string
	cr.Register("gg", func() { result = "gg" })
	cr.Register("gd", func() { result = "gd" })

	cr.Feed('g')
	cr.Feed('d')
	if result != "gd" {
		t.Fatalf("expected gd, got %q", result)
	}

	cr.Feed('g')
	cr.Feed('g')
	if result != "gg" {
		t.Fatalf("expected gg, got %q", result)
	}
}

func TestChordSequentialUse(t *testing.T) {
	cr := NewChordResolver()
	count := 0
	cr.Register("gg", func() { count++ })

	for i := range 3 {
		cr.Feed('g')
		cr.Feed('g')
		if count != i+1 {
			t.Fatalf("iteration %d: expected count %d, got %d", i, i+1, count)
		}
	}
}

func TestChordOnPendingCallback(t *testing.T) {
	cr := NewChordResolver()
	cr.Register("gg", func() {})

	var events []rune
	cr.OnPending = func(r rune) { events = append(events, r) }

	cr.Feed('g') // sets pending
	if len(events) != 1 || events[0] != 'g' {
		t.Fatalf("expected OnPending('g'), got %v", events)
	}

	cr.Feed('g') // clears pending (completes chord)
	if len(events) != 2 || events[1] != 0 {
		t.Fatalf("expected OnPending(0) after completion, got %v", events)
	}
}

func TestChordOnPendingCallback_Reset(t *testing.T) {
	cr := NewChordResolver()
	cr.Register("gg", func() {})

	var events []rune
	cr.OnPending = func(r rune) { events = append(events, r) }

	cr.Feed('g')
	cr.Reset()

	if len(events) != 2 || events[0] != 'g' || events[1] != 0 {
		t.Fatalf("expected [g, 0] from set+reset, got %v", events)
	}
}

func TestChordOnPendingCallback_NoFireWhenAlreadyClear(t *testing.T) {
	cr := NewChordResolver()
	cr.Register("gg", func() {})

	called := 0
	cr.OnPending = func(rune) { called++ }

	cr.Reset() // pending is already 0 — must not fire
	if called != 0 {
		t.Fatalf("OnPending must not fire when Reset on an already-clear resolver")
	}
}

func TestRegister_InvalidLength(t *testing.T) {
	cr := NewChordResolver()
	cr.Register("g", func() {})   // 1 rune — silently ignored
	cr.Register("ggg", func() {}) // 3 runes — silently ignored

	// 'g' must not be treated as a prefix after the invalid registrations.
	if cr.Feed('g') {
		t.Fatal("single-rune chord should be ignored; 'g' must not act as a prefix")
	}
}

func TestWithChords_NilResolver(t *testing.T) {
	innerCalled := false
	fn := WithChords(nil, func(ev *tcell.EventKey) *tcell.EventKey {
		innerCalled = true
		return ev
	})

	ev := tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone)
	out := fn(ev)

	if !innerCalled {
		t.Fatal("inner should be called when cr is nil")
	}
	if out != ev {
		t.Fatal("inner return value should be passed through unchanged")
	}
}

func TestWithChords_RuneConsumed_InnerNotCalled(t *testing.T) {
	cr := NewChordResolver()
	cr.Register("gg", func() {})

	innerCalled := false
	fn := WithChords(cr, func(ev *tcell.EventKey) *tcell.EventKey {
		innerCalled = true
		return ev
	})

	ev := tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone)
	out := fn(ev)

	if innerCalled {
		t.Fatal("inner must not be called when Feed absorbs the rune")
	}
	if out != nil {
		t.Fatal("WithChords must return nil when the rune is consumed")
	}
}

func TestWithChords_UnknownRune_InnerCalled(t *testing.T) {
	cr := NewChordResolver()
	cr.Register("gg", func() {})

	innerCalled := false
	fn := WithChords(cr, func(ev *tcell.EventKey) *tcell.EventKey {
		innerCalled = true
		return ev
	})

	ev := tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone)
	out := fn(ev)

	if !innerCalled {
		t.Fatal("inner should be called for a non-prefix rune")
	}
	if out != ev {
		t.Fatal("inner's return value should be forwarded")
	}
}

func TestWithChords_NonRune_ResetsAndContinues(t *testing.T) {
	cr := NewChordResolver()
	fired := false
	cr.Register("gg", func() { fired = true })

	innerCalled := false
	fn := WithChords(cr, func(ev *tcell.EventKey) *tcell.EventKey {
		innerCalled = true
		return ev
	})

	// Set up pending state.
	fn(tcell.NewEventKey(tcell.KeyRune, 'g', tcell.ModNone))
	if cr.pending == 0 {
		t.Fatal("pending should be set after first 'g'")
	}

	// A non-rune event should reset pending and still call inner.
	escEv := tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone)
	fn(escEv)

	if cr.pending != 0 {
		t.Fatal("non-rune event must reset pending prefix")
	}
	if !innerCalled {
		t.Fatal("inner must still be called for non-rune events")
	}
	if fired {
		t.Fatal("chord handler must not fire after Escape interrupts the sequence")
	}
}
