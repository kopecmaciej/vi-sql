package core

import "testing"

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
