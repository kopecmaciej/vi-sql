//go:build wezterm

package wezterm_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestPeekCell(t *testing.T) {
	longText := strings.Repeat("vi-sql-peeker-test-", 10)

	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name text")
	db.Exec(fmt.Sprintf(
		"INSERT INTO %s (name) VALUES ('%s')",
		db.Qualified(schema, table), longText,
	))

	s := harness.SpawnConnected(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPane("⏱")

	s.MoveDown(1)
	s.PeekRow()
	s.WaitForPane(" Row Details ")
	s.MoveDown(1)
	s.Select()
	s.AssertPaneContains(strings.Repeat("vi-sql-peeker-test-", 3))
	s.Close()
	s.AssertPaneNotContains(" Row Details ")
}
