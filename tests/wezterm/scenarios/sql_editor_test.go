//go:build wezterm

package wezterm_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestSQLHistoryRecall(t *testing.T) {
	s := harness.SpawnConnected(t, "--debug")

	s.RunQueryInNewTab("SELECT 99999 AS n")
	s.WaitForQueryResult(10 * time.Second)

	s.NewTab()
	s.AssertPaneContains("Query")

	s.OpenHistory()
	s.WaitForPane(" SQL History ")
	s.Select()
	s.AssertPaneNotContains(" SQL History ")
	s.AssertPaneContains("SELECT 99999")
}

func TestSQLInvalidQueryNotAppearInHistory(t *testing.T) {
	s := harness.SpawnConnected(t, "--debug")

	s.RunQueryInNewTab("select * from not.existing")
	s.CloseError("Query error")

	s.OpenHistory()
	s.WaitForPane(" SQL History ")
	s.AssertPaneNotContains("select * from not.existing")
	s.Close()
	s.AssertPaneNotContains(" SQL History ")
}

func TestSQLErrorOnBadQuery(t *testing.T) {
	s := harness.SpawnConnected(t, "--debug")

	s.RunQueryInNewTab("THIS IS NOT VALID SQL")
	s.CloseError("")
}

func TestSQLEditorTypeAndRun(t *testing.T) {
	s := harness.SpawnConnected(t, "--debug")

	s.RunQueryInNewTab("SELECT table_schema, table_name FROM information_schema.tables")
	s.WaitForQueryResult(10 * time.Second)

	s.WaitForPaneTimeout("table_schema", 10*time.Second)

	pane := s.GetPaneText()
	if !strings.Contains(pane, "information_schema") {
		t.Errorf("expected result pane to contain 'information_schema', got:\n%s", pane)
	}
}

func TestSQLEditorDelete(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name text")
	db.Exec(fmt.Sprintf("INSERT INTO %s (name) VALUES ('to_delete')", db.Qualified(schema, table)))

	s := harness.SpawnConnected(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPaneTimeout("⏱", 10*time.Second)

	s.RunQueryInNewTab(fmt.Sprintf("DELETE FROM %s WHERE id = 1", db.Qualified(schema, table)))
	s.WaitForPane("Execute this statement?")
	s.Send("Enter")
	s.WaitForQueryResult(10 * time.Second)

	harness.WaitForRowCount(t, db, schema, table, 0)
}

// TestSQLEditorUpdateWithoutWhere verifies that an UPDATE with no WHERE clause
// triggers the destructive-statement confirm modal and, when confirmed, updates
// all rows.
func TestSQLEditorUpdateWithoutWhere(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name text")
	db.Exec(fmt.Sprintf("INSERT INTO %s (name) VALUES ('a'),('b'),('c')", db.Qualified(schema, table)))

	s := harness.SpawnConnected(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPaneTimeout("⏱", 10*time.Second)

	s.RunQueryInNewTab(fmt.Sprintf("UPDATE %s SET name = 'updated'", db.Qualified(schema, table)))
	s.WaitForPane("Execute this statement?")
	s.Send("Enter")
	s.WaitForQueryResult(10 * time.Second)

	// All 3 rows should now have name='updated'.
	rows := db.Query(fmt.Sprintf("SELECT DISTINCT name FROM %s", db.Qualified(schema, table)))
	if len(rows) != 1 {
		t.Errorf("expected 1 distinct name after bulk update, got %d: %v", len(rows), rows)
	}
}

// TestSQLEditorTruncate verifies that TRUNCATE triggers the confirm modal and,
// when confirmed, removes all rows from the table.
func TestSQLEditorTruncate(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name text")
	for i := range 3 {
		db.Exec(fmt.Sprintf("INSERT INTO %s (name) VALUES ('row%d')", db.Qualified(schema, table), i))
	}

	s := harness.SpawnConnected(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPaneTimeout("⏱", 10*time.Second)

	s.RunQueryInNewTab(fmt.Sprintf("TRUNCATE TABLE %s", db.Qualified(schema, table)))
	s.WaitForPane("Execute this statement?")
	s.Send("Enter")
	s.WaitForQueryResult(10 * time.Second)

	harness.WaitForRowCount(t, db, schema, table, 0)
}

// TestSQLEditorClear verifies that the Clear keybinding wipes the editor
// text area, leaving it empty.
func TestSQLEditorClear(t *testing.T) {
	s := harness.SpawnConnected(t, "--debug")

	s.NewTab()
	s.AssertPaneContains("Query")
	s.WriteToEditor("SELECT 42 -- unique marker do not keep")

	// Clear the editor — Ctrl+u by default.
	s.ClearEditor()
	s.AssertPaneNotContains("unique marker do not keep")
}

// TestSQLEditorExplain verifies that prefixing a query with EXPLAIN opens the
// explain viewer overlay instead of a regular result grid.
func TestSQLEditorExplain(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name text")
	db.Exec(fmt.Sprintf("INSERT INTO %s (name) VALUES ('x')", db.Qualified(schema, table)))

	s := harness.SpawnConnected(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPaneTimeout("⏱", 10*time.Second)

	s.RunQueryInNewTab(fmt.Sprintf("EXPLAIN SELECT * FROM %s", db.Qualified(schema, table)))
	s.WaitForPaneTimeout("EXPLAIN", 10*time.Second)
}
