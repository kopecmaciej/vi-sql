//go:build wezterm

package wezterm_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestInlineEditShortValueEnter(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name text")
	db.Exec(fmt.Sprintf("INSERT INTO %s (name) VALUES ('old_value')", db.Qualified(schema, table)))

	s := harness.SpawnConnected(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPane("⏱")

	s.MoveRight(1)
	s.Edit()
	s.WaitForPane(" Inline Edit ")

	s.ClearField()
	s.Paste("new_value")
	s.Send("Enter")

	s.AssertPaneNotContains(" Inline Edit ")
	s.AssertPaneContains("new_value")
	harness.WaitFor(t, 8*time.Second, "name updated in DB", func() bool {
		rows := db.Query("SELECT name FROM " + db.Qualified(schema, table))
		return len(rows) == 1 && rows[0]["name"] == "new_value"
	})
}

func TestInlineEditShortValueConfirmKey(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name text")
	db.Exec(fmt.Sprintf("INSERT INTO %s (name) VALUES ('old_value')", db.Qualified(schema, table)))

	s := harness.SpawnConnected(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPane("⏱")

	s.MoveRight(1)
	s.Edit()
	s.WaitForPane(" Inline Edit ")

	s.ClearField()
	s.Paste("new_value")
	s.Confirm()

	s.AssertPaneNotContains(" Inline Edit ")
	s.AssertPaneContains("new_value")
	harness.WaitFor(t, 8*time.Second, "name updated in DB", func() bool {
		rows := db.Query("SELECT name FROM " + db.Qualified(schema, table))
		return len(rows) == 1 && rows[0]["name"] == "new_value"
	})
}

func TestInlineEditShortValueCancel(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name text")
	db.Exec(fmt.Sprintf("INSERT INTO %s (name) VALUES ('original_name')", db.Qualified(schema, table)))

	s := harness.SpawnConnected(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPane("⏱")

	s.MoveRight(1)
	s.Edit()
	s.WaitForPane(" Inline Edit ")

	s.ClearField()
	s.Paste("discarded")
	s.Close()

	s.AssertPaneNotContains(" Inline Edit ")
	rows := db.Query("SELECT name FROM " + db.Qualified(schema, table))
	if len(rows) != 1 || rows[0]["name"] != "original_name" {
		t.Errorf("cancel: expected name=%q unchanged, got %v", "original_name", rows)
	}
}

func TestInlineEditLongValueConfirmKey(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, content text")
	longX := strings.Repeat("x", 150)
	db.Exec(fmt.Sprintf("INSERT INTO %s (content) VALUES ('%s')", db.Qualified(schema, table), longX))

	s := harness.SpawnConnected(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPane("⏱")

	s.MoveRight(1)
	s.Edit()
	s.WaitForPane(" Inline Edit ")

	// Enter inserts a newline in TextArea — modal must stay open.
	s.Send("Enter")
	s.AssertPaneContains(" Inline Edit ")

	// Undo the newline, clear the line, paste replacement, then confirm.
	s.Send("Backspace")
	s.ClearField()
	longY := strings.Repeat("y", 150)
	s.Paste(longY)
	s.Confirm()

	s.AssertPaneNotContains(" Inline Edit ")
	harness.WaitFor(t, 8*time.Second, "content updated in DB", func() bool {
		rows := db.Query("SELECT content FROM " + db.Qualified(schema, table))
		if len(rows) != 1 {
			return false
		}
		content, ok := rows[0]["content"].(string)
		return ok && strings.HasPrefix(content, "y") && len(content) == 150
	})
}
