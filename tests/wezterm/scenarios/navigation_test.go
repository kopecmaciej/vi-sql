//go:build wezterm

package wezterm_test

import (
	"os"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

// TestSchemaTreeExpandCollapse verifies expand/collapse of a schema node.
func TestSchemaTreeExpandCollapse(t *testing.T) {
	conn := harness.DefaultConnection()
	if conn == "" {
		t.Skip("no connection available — skipping navigation scenario")
	}

	s := harness.Spawn(t, "--connection-name", conn, "--debug")

	s.FocusSchemaTree()
	s.ExpandSchemaNode()
	s.MoveDown(1)
	s.CollapseAll()
	s.FocusSchemaTree()
}

// TestSchemaTreeFilterByTable verifies the schema-tree filter matches table names.
//
// Prerequisites:
//   - The database must have a table named "users" (or set VI_SQL_WEZTERM_FILTER_TABLE).
func TestSchemaTreeFilterByTable(t *testing.T) {
	conn := harness.DefaultConnection()
	if conn == "" {
		t.Skip("no connection available — skipping navigation scenario")
	}
	table := os.Getenv("VI_SQL_WEZTERM_FILTER_TABLE")
	if table == "" {
		table = "users"
	}

	s := harness.Spawn(t, "--connection-name", conn, "--debug")

	s.FocusSchemaTree()
	s.Filter()
	s.Type(table)
	s.Select()

	s.AssertPaneContains(table)
}

// TestOpenTableViaTree uses --jump to pre-open a table and verifies the data grid loads.
func TestOpenTableViaTree(t *testing.T) {
	conn := harness.DefaultConnection()
	if conn == "" {
		t.Skip("no connection available — skipping open-table-via-tree scenario")
	}
	jump := harness.DefaultJump()

	s := harness.Spawn(t, "--connection-name", conn, "--jump", jump, "--debug")
	s.WaitForPane(" rows", 10*time.Second)
}
