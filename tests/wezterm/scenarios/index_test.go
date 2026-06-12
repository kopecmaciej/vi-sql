//go:build wezterm

package wezterm_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestIndexView(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name varchar(255)")
	db.Exec(fmt.Sprintf(
		"CREATE INDEX vi_sql_test_idx ON %s (name)",
		db.Qualified(schema, table),
	))

	s := openSchemaTreeAtFixture(t, schema)

	s.OpenIndexes()
	s.WaitForPane(" Indexes ")
	s.AssertPaneContains("vi_sql_test_idx")
	s.CloseTab()
	s.AssertPaneNotContains(" Indexes ")
}

func TestAddIndex(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name varchar(255)")
	s := openSchemaTreeAtFixture(t, schema)

	s.OpenIndexes()
	s.WaitForPane(" Indexes ")
	s.Add()
	s.WaitForPane("Column 1")
	s.Paste("name")
	s.FocusDown(1) // order
	s.Select()     // order dropdown
	s.MoveDown(1)
	s.Select()
	s.WaitForPane("DESC")
	s.FocusDown(1)
	s.Paste("vi_sql_test_new_idx")
	s.Confirm()

	harness.WaitFor(t, 8*time.Second, fmt.Sprintf("index vi_sql_test_new_idx on %s.%s", schema, table),
		func() bool { return db.IndexExists(schema, table, "vi_sql_test_new_idx") })
}

func TestDeleteIndex(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name varchar(255)")
	db.Exec(fmt.Sprintf(
		"CREATE INDEX vi_sql_test_del_idx ON %s (name)",
		db.Qualified(schema, table),
	))
	s := openSchemaTreeAtFixture(t, schema)

	s.OpenIndexes()
	s.WaitForPane("vi_sql_test_del_idx")
	s.MoveDown(1)
	s.Delete()
	s.WaitForPane("Drop index")
	s.Select()

	s.Delete()
	s.WaitForPane("Drop index")
	s.Select()
	s.CloseError("Error dropping index") // primary index cannot be dropped

	s.AssertPaneNotContains("vi_sql_test_del_idx")
	harness.WaitFor(t, 8*time.Second, fmt.Sprintf("index vi_sql_test_del_idx dropped from %s.%s", schema, table),
		func() bool { return !db.IndexExists(schema, table, "vi_sql_test_del_idx") })
}

func TestDeleteIndexViaSQLEditor(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name varchar(255)")
	db.Exec(fmt.Sprintf(
		"CREATE INDEX vi_sql_test_sqldrop_idx ON %s (name)",
		db.Qualified(schema, table),
	))
	s := openSchemaTreeAtFixture(t, schema)

	s.RunQueryInNewTab(db.DropIndexSQL(schema, table, "vi_sql_test_sqldrop_idx"))
	s.WaitForPane("Execute this statement?")
	s.Send("Enter")
	s.WaitForQueryResult()

	harness.WaitFor(t, 8*time.Second, fmt.Sprintf("index vi_sql_test_sqldrop_idx dropped from %s.%s", schema, table),
		func() bool { return !db.IndexExists(schema, table, "vi_sql_test_sqldrop_idx") })
}

func TestAddIndexViaSQLMode(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name varchar(255)")
	s := openSchemaTreeAtFixture(t, schema)

	s.OpenIndexes()
	s.WaitForPane(" Indexes ")
	s.Add()
	s.WaitForPane("Column 1")
	s.ToggleSQLMode()
	s.WaitForPane("Create Index")
	s.ClearEditor()
	s.WriteToEditor(fmt.Sprintf(
		`CREATE INDEX vi_sql_test_sql_idx ON %s (name)`,
		db.Qualified(schema, table),
	))
	s.RunQuery()

	harness.WaitFor(t, 8*time.Second, fmt.Sprintf("index vi_sql_test_sql_idx on %s.%s", schema, table),
		func() bool { return db.IndexExists(schema, table, "vi_sql_test_sql_idx") })
}
