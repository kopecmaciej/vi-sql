//go:build wezterm

package wezterm_test

import (
	"fmt"
	"testing"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestFollowForeignKey(t *testing.T) {
	schema, db := harness.NewFixtureSchema(t)
	db.CreateTable(schema, "parent", "id serial primary key, name text")
	db.CreateTable(schema, "child",
		"id serial primary key, parent_id int, "+
			db.FKConstraint("parent_id", schema, "parent", "id"))
	db.Exec(fmt.Sprintf("INSERT INTO %s (name) VALUES ('fk_target_alpha')", db.Qualified(schema, "parent")))
	db.Exec(fmt.Sprintf("INSERT INTO %s (parent_id) VALUES (1)", db.Qualified(schema, "child")))

	s := harness.SpawnConnected(t, "--jump", schema+".child", "--debug")
	s.WaitForPane("⏱")

	s.MoveDown(1)
	s.MoveRight(1)
	s.FollowForeignKey()

	s.WaitForPane("fk_target_alpha")
}

func TestFindReferencesSingleTable(t *testing.T) {
	schema, db := harness.NewFixtureSchema(t)
	db.CreateTable(schema, "parent", "id serial primary key")
	db.CreateTable(schema, "child",
		"id serial primary key, parent_id int, tag text, "+
			db.FKConstraint("parent_id", schema, "parent", "id"))
	db.InsertDefault(schema, "parent")
	db.Exec(fmt.Sprintf("INSERT INTO %s (parent_id, tag) VALUES (1, 'ref_marker_xyz')", db.Qualified(schema, "child")))

	s := harness.SpawnConnected(t, "--jump", schema+".parent", "--debug")
	s.WaitForPane("⏱")

	s.MoveDown(1)
	s.FindReferences()

	s.WaitForPane("ref_marker_xyz")
}

func TestFindReferencesMultipleTablesList(t *testing.T) {
	schema, db := harness.NewFixtureSchema(t)
	db.CreateTable(schema, "parent", "id serial primary key")
	db.CreateTable(schema, "ref_alpha",
		"id serial primary key, parent_id int, tag text, "+
			db.FKConstraint("parent_id", schema, "parent", "id"))
	db.CreateTable(schema, "ref_beta",
		"id serial primary key, parent_id int, tag text, "+
			db.FKConstraint("parent_id", schema, "parent", "id"))
	db.InsertDefault(schema, "parent")
	db.Exec(fmt.Sprintf("INSERT INTO %s (parent_id, tag) VALUES (1, 'alpha_marker')", db.Qualified(schema, "ref_alpha")))
	db.Exec(fmt.Sprintf("INSERT INTO %s (parent_id, tag) VALUES (1, 'beta_marker')", db.Qualified(schema, "ref_beta")))

	s := harness.SpawnConnected(t, "--jump", schema+".parent", "--debug")
	s.WaitForPane("⏱")

	s.MoveDown(1)
	s.FindReferences()
	s.WaitForPane(" Referenced by ")
	s.AssertPaneContains("ref_alpha")
	s.AssertPaneContains("ref_beta")

	s.Send("Enter")
	s.AssertPaneNotContains(" Referenced by ")
	s.WaitForPane("alpha_marker")
}

func TestFindReferencesListCancel(t *testing.T) {
	schema, db := harness.NewFixtureSchema(t)
	db.CreateTable(schema, "parent", "id serial primary key")
	db.CreateTable(schema, "ref_one",
		"id serial primary key, parent_id int, "+
			db.FKConstraint("parent_id", schema, "parent", "id"))
	db.CreateTable(schema, "ref_two",
		"id serial primary key, parent_id int, "+
			db.FKConstraint("parent_id", schema, "parent", "id"))
	db.InsertDefault(schema, "parent")

	s := harness.SpawnConnected(t, "--jump", schema+".parent", "--debug")
	s.WaitForPane("⏱")

	s.MoveDown(1)
	s.FindReferences()
	s.WaitForPane(" Referenced by ")

	s.Close()
	s.AssertPaneNotContains(" Referenced by ")
}
