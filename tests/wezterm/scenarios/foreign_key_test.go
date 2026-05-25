//go:build wezterm

package wezterm_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestFollowForeignKey(t *testing.T) {
	schema, db := harness.NewFixtureSchema(t)
	db.Exec(fmt.Sprintf(`CREATE TABLE "%s".parent (id serial primary key, name text)`, schema))
	db.Exec(fmt.Sprintf(`CREATE TABLE "%s".child (id serial primary key, parent_id int references "%s".parent(id))`, schema, schema))
	db.Exec(fmt.Sprintf(`INSERT INTO "%s".parent (name) VALUES ('fk_target_alpha')`, schema))
	db.Exec(fmt.Sprintf(`INSERT INTO "%s".child (parent_id) VALUES (1)`, schema))

	s := harness.SpawnConnected(t, "--jump", schema+".child", "--debug")
	s.WaitForPane("⏱", 10*time.Second)

	s.MoveDown(1)
	s.MoveRight(1)
	s.FollowForeignKey()

	s.WaitForPane("fk_target_alpha", 10*time.Second)
}

func TestFindReferencesSingleTable(t *testing.T) {
	schema, db := harness.NewFixtureSchema(t)
	db.Exec(fmt.Sprintf(`CREATE TABLE "%s".parent (id serial primary key)`, schema))
	db.Exec(fmt.Sprintf(`CREATE TABLE "%s".child (id serial primary key, parent_id int references "%s".parent(id), tag text)`, schema, schema))
	db.Exec(fmt.Sprintf(`INSERT INTO "%s".parent DEFAULT VALUES`, schema))
	db.Exec(fmt.Sprintf(`INSERT INTO "%s".child (parent_id, tag) VALUES (1, 'ref_marker_xyz')`, schema))

	s := harness.SpawnConnected(t, "--jump", schema+".parent", "--debug")
	s.WaitForPane("⏱", 10*time.Second)

	s.MoveDown(1)
	s.FindReferences()

	s.WaitForPane("ref_marker_xyz", 10*time.Second)
}

func TestFindReferencesMultipleTablesList(t *testing.T) {
	schema, db := harness.NewFixtureSchema(t)
	db.Exec(fmt.Sprintf(`CREATE TABLE "%s".parent (id serial primary key)`, schema))
	db.Exec(fmt.Sprintf(`CREATE TABLE "%s".ref_alpha (id serial primary key, parent_id int references "%s".parent(id), tag text)`, schema, schema))
	db.Exec(fmt.Sprintf(`CREATE TABLE "%s".ref_beta (id serial primary key, parent_id int references "%s".parent(id), tag text)`, schema, schema))
	db.Exec(fmt.Sprintf(`INSERT INTO "%s".parent DEFAULT VALUES`, schema))
	db.Exec(fmt.Sprintf(`INSERT INTO "%s".ref_alpha (parent_id, tag) VALUES (1, 'alpha_marker')`, schema))
	db.Exec(fmt.Sprintf(`INSERT INTO "%s".ref_beta (parent_id, tag) VALUES (1, 'beta_marker')`, schema))

	s := harness.SpawnConnected(t, "--jump", schema+".parent", "--debug")
	s.WaitForPane("⏱", 10*time.Second)

	s.MoveDown(1)
	s.FindReferences()
	s.WaitForPane(" Referenced by ", 5*time.Second)
	s.AssertPaneContains("ref_alpha")
	s.AssertPaneContains("ref_beta")

	s.Send("Enter")
	s.AssertPaneNotContains(" Referenced by ")

	pane := s.GetPaneText()
	if !strings.Contains(pane, "alpha_marker") && !strings.Contains(pane, "beta_marker") {
		t.Errorf("expected new tab to show one of the referencing tables, pane:\n%s", pane)
	}
}

func TestFindReferencesListCancel(t *testing.T) {
	schema, db := harness.NewFixtureSchema(t)
	db.Exec(fmt.Sprintf(`CREATE TABLE "%s".parent (id serial primary key)`, schema))
	db.Exec(fmt.Sprintf(`CREATE TABLE "%s".ref_one (id serial primary key, parent_id int references "%s".parent(id))`, schema, schema))
	db.Exec(fmt.Sprintf(`CREATE TABLE "%s".ref_two (id serial primary key, parent_id int references "%s".parent(id))`, schema, schema))
	db.Exec(fmt.Sprintf(`INSERT INTO "%s".parent DEFAULT VALUES`, schema))

	s := harness.SpawnConnected(t, "--jump", schema+".parent", "--debug")
	s.WaitForPane("⏱", 10*time.Second)

	s.MoveDown(1)
	s.FindReferences()
	s.WaitForPane(" Referenced by ", 5*time.Second)

	s.Close()
	s.AssertPaneNotContains(" Referenced by ")
}
