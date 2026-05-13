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

	s.Send("Ctrl+t")
	s.AssertPaneContains("Query")

	s.TypeQuery("SELECT table_schema, table_name FROM information_schema.tables LIMIT 10")

	s.Send("Ctrl+s")

	s.WaitForPane("⏱", 10*time.Second)
	s.WaitForPane("table_schema", 10*time.Second)

	pane := s.GetPaneText()
	if !strings.Contains(pane, "information_schema") {
		t.Errorf("expected result pane to contain 'information_schema', got:\n%s", pane)
	}
}
