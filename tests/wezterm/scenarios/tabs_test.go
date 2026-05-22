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
