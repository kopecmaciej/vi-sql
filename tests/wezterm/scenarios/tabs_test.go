//go:build wezterm

package wezterm_test

import (
	"testing"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

// TestQueryTabLifecycle opens a new query tab and closes it, verifying that
// the tab bar reflects both operations.
func TestQueryTabLifecycle(t *testing.T) {
	s := harness.SpawnConnected(t, "--debug")

	s.NewTab()
	s.AssertPaneContains("Query")

	s.CloseTab()
}

func TestMultipleTabsSwitching(t *testing.T) {
	s := harness.SpawnConnected(t, "--debug")

	s.NewTab()
	s.WriteToEditor("UNIQUETAB1")
	s.NewTab()
	s.WriteToEditor("UNIQUETAB2")
	s.NewTab()
	s.WriteToEditor("UNIQUETAB3")

	s.AssertPaneContains("UNIQUETAB3")

	s.PrevTab()
	s.AssertPaneContains("UNIQUETAB2")
	s.AssertPaneNotContains("UNIQUETAB3")

	s.PrevTab()
	s.AssertPaneContains("UNIQUETAB1")
	s.AssertPaneNotContains("UNIQUETAB2")

	s.NextTab()
	s.AssertPaneContains("UNIQUETAB2")
}
