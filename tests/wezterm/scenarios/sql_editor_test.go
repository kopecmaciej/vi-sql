//go:build wezterm

package wezterm_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestSQLQueryAppearsInHistory(t *testing.T) {
	s := harness.SpawnConnected(t, "--debug")

	s.RunQueryInNewTab("SELECT 1")
	s.WaitForQueryResult(10 * time.Second)

	s.OpenHistory()
	s.WaitForPane(" SQL History ", 5*time.Second)
	s.AssertPaneContains("SELECT 1")
	s.Close()
	s.AssertPaneNotContains(" SQL History ")
}

func TestSQLHistoryRecall(t *testing.T) {
	s := harness.SpawnConnected(t, "--debug")

	s.RunQueryInNewTab("SELECT 99999")
	s.WaitForQueryResult(10 * time.Second)

	s.NewTab()
	s.AssertPaneContains("Query")

	s.OpenHistory()
	s.WaitForPane(" SQL History ", 5*time.Second)
	s.Select()
	s.AssertPaneNotContains(" SQL History ")
	s.AssertPaneContains("SELECT 99999")
}

func TestSQLErrorOnBadQuery(t *testing.T) {
	s := harness.SpawnConnected(t, "--debug")

	s.RunQueryInNewTab("THIS IS NOT VALID SQL")

	s.WaitForPane(" Error ", 5*time.Second)
	s.Close()
	s.AssertPaneNotContains(" Error ")
}

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
