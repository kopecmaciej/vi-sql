//go:build wezterm

// Package wezterm_test contains end-to-end scenarios for vi-sql, driven by
// wezterm keystroke injection. Scenarios are guarded by the "wezterm" build
// tag so they never run as part of the default test suite.
//
// Run with:
//
//	VI_SQL_WEZTERM_CONNECTION=<name> make test-wezterm
//
// or:
//
//	VI_SQL_WEZTERM_CONNECTION=<name> go test -tags=wezterm -count=1 -v ./tests/wezterm/scenarios/
package wezterm_test

import (
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

// TestSchemaTableFilter simulates a manual navigation session:
//  1. Connect to the database named in VI_SQL_WEZTERM_CONNECTION.
//  2. Focus the schema tree (Ctrl+/).
//  3. Navigate to the 3rd schema (Down x2) and expand it (e).
//  4. Navigate to the 2nd table (Down x2) and open it (Enter).
//  5. Apply a WHERE filter (/) with the clause "1=1".
//
// Prerequisites:
//   - VI_SQL_WEZTERM_CONNECTION (or active connection) must be available.
//   - The connection must expose at least 3 schemas, and the 3rd schema must
//     have at least 2 tables.
func TestSchemaTableFilter(t *testing.T) {
	conn := harness.DefaultConnection()
	if conn == "" {
		t.Skip("no connection available — skipping filter scenario")
	}

	s := harness.Spawn(t, "--connection-name", conn, "--debug")

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
	s.WaitForPane(" rows", 10*time.Second)

	// Open the filter bar and type the WHERE clause.
	s.Filter()
	s.Type("1=1")
	s.Select()

	// Results bar shows "⚑ WHERE: 1=1" on its second line after the query reruns.
	s.AssertPaneContains("WHERE: 1=1")
}
