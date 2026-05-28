//go:build wezterm

package wezterm_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestMultiSelectCopy(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t,
		"id serial primary key, name text")
	for _, name := range []string{"alice", "bob", "carol"} {
		db.Exec(fmt.Sprintf("INSERT INTO %s (name) VALUES ('%s')", db.Qualified(schema, table), name))
	}

	s := harness.SpawnConnected(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPaneTimeout("⏱", 10*time.Second)

	s.MultipleSelect()
	s.MoveDown(2)
	s.CopyRow()

	got := util.Paste()
	for _, name := range []string{"alice", "bob", "carol"} {
		if !strings.Contains(got, name) {
			t.Errorf("clipboard missing %q; got: %s", name, got)
		}
	}
}

func TestMultiSelectExport(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t,
		"id serial primary key, name text")
	for _, name := range []string{"alpha", "beta", "gamma"} {
		db.Exec(fmt.Sprintf("INSERT INTO %s (name) VALUES ('%s')", db.Qualified(schema, table), name))
	}

	s := harness.SpawnConnected(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPaneTimeout("⏱", 10*time.Second)

	s.MultipleSelect()
	s.MoveDown(1)

	tmp := t.TempDir()
	s.OpenExportModal()
	s.WaitForPane(" Export ")

	s.FocusDown(1) // → Filename
	s.ClearField()
	s.Paste("multi.csv")
	s.FocusDown(1) // → Path
	s.ClearField()
	s.Paste(tmp + "/")
	s.FocusDown(3) // → Export button
	s.Select()

	s.AssertPaneNotContains(" Export ")
	s.WaitForFile(tmp+"/multi.csv", 3*time.Second)

	data, err := os.ReadFile(tmp + "/multi.csv")
	if err != nil {
		t.Fatalf("cannot read export file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	// header + 2 data rows selected (rows 1 and 2 out of 3)
	if len(lines) != 3 {
		t.Errorf("export: want 3 lines (header+2 rows), got %d:\n%s", len(lines), string(data))
	}
}

func TestMultiSelectDelete(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t,
		"id serial primary key, name text")
	for _, name := range []string{"keep", "drop1", "drop2"} {
		db.Exec(fmt.Sprintf("INSERT INTO %s (name) VALUES ('%s')", db.Qualified(schema, table), name))
	}

	s := harness.SpawnConnected(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPaneTimeout("⏱", 10*time.Second)

	s.MoveDown(1)
	s.MultipleSelect()
	s.MoveDown(1)
	s.Delete()
	s.WaitForPane("delete 2 rows")
	s.Send("Enter")

	harness.WaitForRowCount(t, db, schema, table, 1)
}
