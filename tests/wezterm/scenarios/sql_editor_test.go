//go:build wezterm

package wezterm_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

// TestSQLEditorTypeAndRun opens a query tab, types a multi-line SQL query into
// the editor, executes it, and checks that results appear.
//
// Prerequisites:
//   - VI_SQL_WEZTERM_CONNECTION must be set to a pre-configured PostgreSQL connection.
func TestSQLEditorTypeAndRun(t *testing.T) {
	conn := os.Getenv("VI_SQL_WEZTERM_CONNECTION")
	if conn == "" {
		t.Skip("VI_SQL_WEZTERM_CONNECTION not set — skipping SQL editor scenario")
	}

	s := harness.Spawn(t, "--connection-name", conn, "--debug")

	// Open a new query tab.
	s.Send("Ctrl+t")
	s.AssertPaneContains("Query")

	// Enter insert mode — the vim editor starts in normal mode.
	s.Send("i")

	// Type a multi-line query. information_schema.tables exists on any PostgreSQL connection.
	s.TypeQuery("SELECT\n    table_schema,\n    table_name\nFROM information_schema.tables\nLIMIT 10")

	// Execute (Ctrl+s = Common.Confirm; falls through vim mode unchanged).
	s.Send("Ctrl+s")

	// Wait for the results bar — confirms the query ran and data loaded.
	s.WaitForPane(" rows", 10*time.Second)

	pane := s.GetPaneText()
	if !strings.Contains(pane, "information_schema") {
		t.Errorf("expected result pane to contain 'information_schema', got:\n%s", pane)
	}
}
