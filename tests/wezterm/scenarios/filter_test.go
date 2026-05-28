//go:build wezterm

package wezterm_test

import (
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestSchemaTableFilter(t *testing.T) {
	s := harness.SpawnConnected(t, "--debug")

	s.FocusSchemaTree()
	s.MoveDown(2) // 3rd schema
	s.Select()    // expand
	s.MoveDown(2) // 2nd table
	s.Select()    // open as data tab
	// SELECT queries aren't logged, so we assert on pane content.
	s.WaitForPaneTimeout("⏱", 10*time.Second)

	s.Filter()
	s.Type("1=1")
	s.Select()
	s.AssertPaneContains("WHERE: 1=1")
}

func TestFilterClear(t *testing.T) {
	schema, table, _ := harness.NewFixtureTable(t, "id serial primary key, name text")

	s := harness.SpawnConnected(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPaneTimeout("⏱", 10*time.Second)

	s.Filter()
	s.Paste("1=1")
	s.Confirm()
	s.AssertPaneContains("WHERE: 1=1")

	s.Filter()
	s.ClearField()
	s.Confirm()
	s.AssertPaneNotContains("WHERE:")
}
