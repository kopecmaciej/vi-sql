//go:build wezterm

package wezterm_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

// openSchemaTreeAtFixture connects, filters the schema tree down to the
// fixture schema, and leaves the cursor on the fixture table node.
// Filtering leaves the cursor on the matched schema node; ExpandAll
// guarantees that node is open so MoveDown reaches the table beneath it.
func openSchemaTreeAtFixture(t *testing.T, schema string) *harness.Session {
	t.Helper()
	s := harness.SpawnConnected(t, "--debug")
	s.FocusSchemaTree()
	s.Filter()
	s.Paste(schema)
	s.Send("Enter")
	s.ExpandAll()
	s.MoveDown(1)
	return s
}

func TestCreateTable(t *testing.T) {
	schema, _, db := harness.NewFixtureTable(t, "id serial primary key")
	s := openSchemaTreeAtFixture(t, schema)

	s.Add()
	s.WaitForPane(" Create Table ", 5*time.Second)
	s.Type("created_tbl")
	s.FocusDown(1)
	s.Confirm()

	s.AssertPaneNotContains(" Create Table ")
	waitForTable(t, db, schema, "created_tbl", true)
}

func TestRenameTable(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key")
	s := openSchemaTreeAtFixture(t, schema)

	s.RenameTable()
	s.WaitForPane("Rename table", 5*time.Second)
	s.ClearField()
	s.Type("fixture_renamed")
	s.Send("Enter")

	waitForTable(t, db, schema, "fixture_renamed", true)
	waitForTable(t, db, schema, table, false)
}

func TestDropTable(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key")
	s := openSchemaTreeAtFixture(t, schema)

	s.Delete()
	s.WaitForPane("drop", 5*time.Second)
	s.Send("Enter")

	waitForTable(t, db, schema, table, false)
}

func TestDropTableCancel(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key")
	s := openSchemaTreeAtFixture(t, schema)

	s.Delete()
	s.WaitForPane("drop", 5*time.Second)
	s.MoveRight(1)
	s.Send("Enter")
	s.AssertPaneNotContains("Are you sure")

	if !db.TableExists(schema, table) {
		t.Errorf("drop cancelled: table %s.%s should still exist", schema, table)
	}
}

func waitForTable(t *testing.T, db *harness.FixtureDB, schema, table string, want bool) {
	t.Helper()
	harness.WaitFor(t, 8*time.Second, fmt.Sprintf("table %s.%s exists=%v", schema, table, want),
		func() bool { return db.TableExists(schema, table) == want })
}
