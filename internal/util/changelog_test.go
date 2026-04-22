package util

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeChangelogEntries(versions ...string) []ChangelogEntry {
	entries := make([]ChangelogEntry, len(versions))
	for i, v := range versions {
		entries[i] = ChangelogEntry{Version: v, Title: "v" + v}
	}
	return entries
}

func TestFilterPendingEntries(t *testing.T) {
	tests := []struct {
		name        string
		lastVersion string
		entries     []ChangelogEntry
		wantCount   int
	}{
		{
			name:        "no entries",
			lastVersion: "0.0.1",
			entries:     nil,
			wantCount:   0,
		},
		{
			name:        "all entries older than stored version",
			lastVersion: "0.0.3",
			entries:     makeChangelogEntries("0.0.1", "0.0.2"),
			wantCount:   0,
		},
		{
			name:        "entry equal to stored version — no show (loop prevention)",
			lastVersion: "0.0.2",
			entries:     makeChangelogEntries("0.0.2"),
			wantCount:   0,
		},
		{
			name:        "one entry newer than stored version",
			lastVersion: "0.0.1",
			entries:     makeChangelogEntries("0.0.2"),
			wantCount:   1,
		},
		{
			name:        "multiple entries, only newer ones returned",
			lastVersion: "0.0.2",
			entries:     makeChangelogEntries("0.0.1", "0.0.2", "0.0.3", "0.1.0"),
			wantCount:   2,
		},
		{
			name:        "v-prefix in stored version handled",
			lastVersion: "v0.0.1",
			entries:     makeChangelogEntries("0.0.2"),
			wantCount:   1,
		},
		{
			name:        "dirty dev build stored, entry for same tag — no show",
			lastVersion: "0.0.2",
			entries:     makeChangelogEntries("0.0.2"),
			wantCount:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterPendingEntries(tt.lastVersion, tt.entries)
			assert.Len(t, got, tt.wantCount)
		})
	}
}

// TestNoLoopAfterAcknowledge verifies the loop-prevention property: once the config
// version is updated to the current build version, FilterPendingEntries returns nothing
// so the changelog modal cannot re-trigger in the same session.
func TestNoLoopAfterAcknowledge(t *testing.T) {
	entries := makeChangelogEntries("0.0.2")

	pending := FilterPendingEntries("0.0.1", entries)
	assert.NotEmpty(t, pending, "changelog should appear on upgrade")

	pendingAfter := FilterPendingEntries("0.0.2", entries)
	assert.Empty(t, pendingAfter, "changelog must not appear again after acknowledging")
}

func TestRunMigrations(t *testing.T) {
	errBoom := errors.New("boom")

	t.Run("no entries", func(t *testing.T) {
		assert.NoError(t, RunMigrations(nil))
	})

	t.Run("entries without migration fns", func(t *testing.T) {
		assert.NoError(t, RunMigrations(makeChangelogEntries("0.0.1", "0.0.2")))
	})

	t.Run("all migrations succeed", func(t *testing.T) {
		called := 0
		entries := []ChangelogEntry{
			{Version: "0.0.1", MigrationFn: func() error { called++; return nil }},
			{Version: "0.0.2", MigrationFn: func() error { called++; return nil }},
		}
		require.NoError(t, RunMigrations(entries))
		assert.Equal(t, 2, called)
	})

	t.Run("first migration fails — stops immediately", func(t *testing.T) {
		secondCalled := false
		entries := []ChangelogEntry{
			{Version: "0.0.1", MigrationFn: func() error { return errBoom }},
			{Version: "0.0.2", MigrationFn: func() error { secondCalled = true; return nil }},
		}
		err := RunMigrations(entries)
		require.Error(t, err)
		assert.ErrorIs(t, err, errBoom)
		assert.False(t, secondCalled, "second migration must not run after first fails")
	})

	t.Run("error wraps version", func(t *testing.T) {
		entries := []ChangelogEntry{
			{Version: "0.1.0", MigrationFn: func() error { return errBoom }},
		}
		err := RunMigrations(entries)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "0.1.0")
	})
}
