//go:build wezterm

package wezterm_test

import (
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

// TestSchemaTableFilter simulates a manual navigation session:
//  1. Connect via VI_SQL_TESTS_DSN.
//  2. Focus the schema tree.
//  3. Navigate to the 3rd schema (Down x2) and expand it.
//  4. Navigate to the 2nd table (Down x2) and open it.
//  5. Apply a WHERE filter (/) with the clause "1=1".
func TestSchemaTableFilter(t *testing.T) {
	s := harness.SpawnConnected(t, "--debug")

	// Focus the schema tree (mode-aware: "ge" sequence in vim, Ctrl+/ in normal).
	s.FocusSchemaTree()

	// Navigate to the 3rd schema. The tree cursor starts on the 1st schema,
	// so two Down presses land on the 3rd.
	s.MoveDown(2)

	// Expand the 3rd schema.
	s.Select()

	// Navigate to the 2nd table inside the expanded schema.
	s.MoveDown(2)

	// Open the selected table as a TableMode data tab.
	s.Select()

	// Wait for the results bar — confirms data loaded (SELECT queries are not
	// written to the log, so we assert on pane content instead).
	s.WaitForPane("⏱", 10*time.Second)

	// Open the filter bar and type the WHERE clause.
	s.Filter()
	s.Type("1=1")
	s.Select()

	// Results bar shows "⚑ WHERE: 1=1" on its second line after the query reruns.
	s.AssertPaneContains("WHERE: 1=1")
}
