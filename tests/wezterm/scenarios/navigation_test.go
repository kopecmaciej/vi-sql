//go:build wezterm

package wezterm_test

import (
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestSchemaTreeFilterByTable(t *testing.T) {
	table := "users"
	s := harness.SpawnConnected(t, "--debug")

	s.FocusSchemaTree()
	s.Filter()
	s.Type(table)
	s.Select()

	s.AssertPaneContains(table)
}

func TestOpenTableViaTree(t *testing.T) {
	jump := harness.DefaultJump()

	s := harness.SpawnConnected(t, "--jump", jump, "--debug")
	s.WaitForPaneTimeout("⏱", 10*time.Second)
}
