//go:build wezterm

package wezterm_test

import (
	"testing"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestStructureView(t *testing.T) {
	schema, _, _ := harness.NewFixtureTable(t,
		"id serial primary key, name text not null, created_at timestamptz default now()")

	s := openSchemaTreeAtFixture(t, schema)

	s.OpenStructure()
	s.WaitForPane(" Structure ")
	s.AssertPaneContains("id")
	s.AssertPaneContains("name")
	s.AssertPaneContains("created_at")
	s.CloseTab()
	s.AssertPaneNotContains(" Structure ")
}
