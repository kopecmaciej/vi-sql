//go:build wezterm

package wezterm_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestStructureView(t *testing.T) {
	schema, _, _ := harness.NewFixtureTable(t,
		"id serial primary key, name text not null, created_at timestamptz default now()")

	s := openSchemaTreeAtFixture(t, schema)

	s.OpenStructure()
	s.WaitForPane(" Structure ")
	s.AssertPaneContains("id")
	s.AssertPaneContains("name")
	s.AssertPaneContains("created_at")
	s.CloseTab()
	s.AssertPaneNotContains(" Structure ")
}

func TestRenameColumn(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name text not null")
	s := openSchemaTreeAtFixture(t, schema)

	s.OpenStructure()
	s.WaitForPane(" Structure ")
	s.MoveDown(1) // move past "id" to "name"
	s.RenameColumn()
	s.WaitForPane(" Inline Edit ")
	s.ClearField()
	s.Paste("name_renamed")
	s.Confirm()

	s.AssertPaneContains("name_renamed")
	harness.WaitFor(t, 8*time.Second, fmt.Sprintf("column name_renamed in %s.%s", schema, table),
		func() bool { return db.ColumnExists(schema, table, "name_renamed") })
	if db.ColumnExists(schema, table, "name") {
		t.Errorf("old column 'name' still exists after rename")
	}
}
