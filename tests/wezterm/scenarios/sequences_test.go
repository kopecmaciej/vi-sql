//go:build wezterm

package wezterm_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestCopyRowJSONVimSequence(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name text")
	db.Exec(fmt.Sprintf("INSERT INTO %s (name) VALUES ('json_row')", db.Qualified(schema, table)))

	s := harness.SpawnConnectedVimMode(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPane("⏱")

	s.CopyRowJSON()

	got := util.Paste()
	t.Logf("GOT: %s", got)
	var obj map[string]any
	if err := json.Unmarshal([]byte(got), &obj); err != nil {
		t.Fatalf("clipboard is not valid JSON: %v\ngot: %s", err, got)
	}
	if !strings.Contains(got, "json_row") {
		t.Errorf("clipboard missing row value; got: %s", got)
	}
}

func TestCopyRowCSVVimSequence(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name text")
	db.Exec(fmt.Sprintf("INSERT INTO %s (name) VALUES ('csv_row')", db.Qualified(schema, table)))

	s := harness.SpawnConnectedVimMode(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPane("⏱")

	s.CopyRowCSV()

	got := util.Paste()
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) < 2 {
		t.Fatalf("CSV should have header + data row, got %d lines:\n%s", len(lines), got)
	}
	if !strings.Contains(got, "csv_row") {
		t.Errorf("clipboard missing row value; got: %s", got)
	}
}

func TestThreeKeyPrefixAbsorbed(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name text")
	db.Exec(fmt.Sprintf("INSERT INTO %s (name) VALUES ('prefix_test')", db.Qualified(schema, table)))

	s := harness.SpawnConnectedVimMode(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPane("⏱")

	util.Copy("sentinel")

	// `yr` is a proper prefix of both `yrj` and `yrc` — each rune should be
	// absorbed without triggering any copy action. Esc cancels the pending prefix.
	s.Send("y", "r", "Esc")

	if got := util.Paste(); got != "sentinel" {
		t.Errorf("partial prefix should not write to clipboard; want sentinel, got: %s", got)
	}
}

func TestCopyRowJSONViaActions(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name text")
	db.Exec(fmt.Sprintf("INSERT INTO %s (name) VALUES ('actions_json')", db.Qualified(schema, table)))

	s := harness.SpawnConnected(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPane("⏱")

	s.OpenActionsModal()
	s.WaitForPane("Actions")
	s.Type("Copy row as JSON")
	s.Send("Enter")

	got := util.Paste()
	var obj map[string]any
	if err := json.Unmarshal([]byte(got), &obj); err != nil {
		t.Fatalf("clipboard is not valid JSON: %v\ngot: %s", err, got)
	}
	if !strings.Contains(got, "actions_json") {
		t.Errorf("clipboard missing row value; got: %s", got)
	}
}

func TestCopyRowCSVViaActions(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name text")
	db.Exec(fmt.Sprintf("INSERT INTO %s (name) VALUES ('actions_csv')", db.Qualified(schema, table)))

	s := harness.SpawnConnected(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPane("⏱")

	s.OpenActionsModal()
	s.WaitForPane("Actions")
	s.Type("Copy row as CSV")
	s.Send("Enter")

	got := util.Paste()
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) < 2 {
		t.Fatalf("CSV should have header + data row, got %d lines:\n%s", len(lines), got)
	}
	if !strings.Contains(got, "actions_csv") {
		t.Errorf("clipboard missing row value; got: %s", got)
	}
}

func TestCopyMultipleRowsJSONVimSequence(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name text")
	for _, name := range []string{"alice", "bob"} {
		db.Exec(fmt.Sprintf("INSERT INTO %s (name) VALUES ('%s')", db.Qualified(schema, table), name))
	}

	s := harness.SpawnConnectedVimMode(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPane("⏱")

	s.MultipleSelect()
	s.MoveDown(1)
	s.CopyRowJSON()

	got := util.Paste()
	var arr []map[string]any
	if err := json.Unmarshal([]byte(got), &arr); err != nil {
		t.Fatalf("clipboard should be a JSON array for multiple rows: %v\ngot: %s", err, got)
	}
	if len(arr) != 2 {
		t.Errorf("want 2 JSON objects, got %d: %s", len(arr), got)
	}
	for _, name := range []string{"alice", "bob"} {
		if !strings.Contains(got, name) {
			t.Errorf("clipboard missing %q; got: %s", name, got)
		}
	}
}
