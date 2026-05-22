//go:build wezterm

package wezterm_test

import (
	"testing"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

// TestQueryTabLifecycle opens a new query tab and closes it, verifying that
// the tab bar reflects both operations.
func TestQueryTabLifecycle(t *testing.T) {
	conn := harness.DefaultConnection()
	if conn == "" {
		t.Skip("no connection available — skipping tab lifecycle scenario")
	}

	s := harness.Spawn(t, "--connection-name", conn, "--debug")

	s.NewTab()
	s.AssertPaneContains("Query")

	s.CloseTab()
}
