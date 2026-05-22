//go:build wezterm

package wezterm_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

// TestSQLEditorTypeAndRun opens a query tab, pastes a SQL query into the
// editor via clipboard, executes it, and checks that results appear.
func TestSQLEditorTypeAndRun(t *testing.T) {
	s := harness.SpawnConnected(t, "--debug")

	s.RunQueryInNewTab("SELECT table_schema, table_name FROM information_schema.tables LIMIT 10")
	s.WaitForQueryResult(10 * time.Second)

	s.WaitForPane("table_schema", 10*time.Second)

	pane := s.GetPaneText()
	if !strings.Contains(pane, "information_schema") {
		t.Errorf("expected result pane to contain 'information_schema', got:\n%s", pane)
	}
}
