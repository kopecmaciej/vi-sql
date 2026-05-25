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
	s := harness.SpawnConnected(t, "--debug")

	s.FocusSchemaTree()
	s.ExpandSchemaNode()
	s.MoveDown(1)
	s.CollapseAll()
	s.FocusSchemaTree()
}

// TestSchemaTreeFilterByTable verifies the schema-tree filter matches table names.
func TestSchemaTreeFilterByTable(t *testing.T) {
	table := "users"
	s := harness.SpawnConnected(t, "--debug")

	s.FocusSchemaTree()
	s.Filter()
	s.Type(table)
	s.Select()

	s.AssertPaneContains(table)
}

// TestOpenTableViaTree uses --jump to pre-open a table and verifies the data grid loads.
func TestOpenTableViaTree(t *testing.T) {
	jump := harness.DefaultJump()

	s := harness.SpawnConnected(t, "--jump", jump, "--debug")
	s.WaitForPane("⏱", 10*time.Second)
}
