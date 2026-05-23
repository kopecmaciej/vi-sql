//go:build wezterm

package wezterm_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestRowLifecycle(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t,
		"id serial primary key, name text, email text")

	s := harness.SpawnConnected(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPane("⏱", 10*time.Second)

	s.Add()
	s.WaitForPane(" Add Row ", 5*time.Second)
	s.Confirm()
	harness.WaitForRowCount(t, db, schema, table, 1)

	s.MoveRight(1)
	s.Edit()
	s.WaitForPane(" Inline Edit ", 5*time.Second)
	s.ClearField()
	s.Paste("lifecycle_edit")
	s.Send("Enter")
	s.AssertPaneNotContains(" Inline Edit ")
	harness.WaitFor(t, 8*time.Second, "name persisted to DB", func() bool {
		rows := db.Query("SELECT name FROM " + db.Qualified(schema, table))
		return len(rows) == 1 && rows[0]["name"] == "lifecycle_edit"
	})
	s.AssertPaneContains("lifecycle_edit")

	s.DuplicateRow()
	s.WaitForPane(" Duplicate Row ", 5*time.Second)
	s.Confirm()
	harness.WaitForRowCount(t, db, schema, table, 2)

	s.Delete()
	s.WaitForPane("delete this row", 5*time.Second)
	s.Send("Enter")
	harness.WaitForRowCount(t, db, schema, table, 1)
}

func TestRowDeleteCancel(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name text")
	db.Exec(fmt.Sprintf("INSERT INTO %s (name) VALUES ('keep_me')", db.Qualified(schema, table)))

	s := harness.SpawnConnected(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPane("⏱", 10*time.Second)

	s.MoveDown(1)
	s.Delete()
	s.WaitForPane("delete this row", 5*time.Second)
	s.MoveRight(1)
	s.Send("Enter")
	s.AssertPaneNotContains("delete this row")

	if got := db.Count(schema, table); got != 1 {
		t.Errorf("delete cancelled: want 1 row, got %d", got)
	}
}
