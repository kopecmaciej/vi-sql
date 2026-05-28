//go:build wezterm

package wezterm_test

import (
	"fmt"
	"testing"

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
