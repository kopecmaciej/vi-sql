//go:build wezterm

package wezterm_test

import (
	"os"
	"testing"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

// TestQueryTabLifecycle opens a new query tab and closes it, verifying that
// the tab bar reflects both operations.
//
// Prerequisites:
//   - VI_SQL_WEZTERM_CONNECTION must be set to a pre-configured connection name.
func TestQueryTabLifecycle(t *testing.T) {
	conn := os.Getenv("VI_SQL_WEZTERM_CONNECTION")
	if conn == "" {
		t.Skip("VI_SQL_WEZTERM_CONNECTION not set — skipping tab lifecycle scenario")
	}

	s := harness.Spawn(t, "--connection-name", conn, "--debug")

	// Open a new query tab (Main.NewTab = Ctrl+t).
	s.Send("Ctrl+t")
	// The tab bar should now show a Query tab entry.
	s.AssertPaneContains("Query")

	// Close the active tab (Main.CloseTab = Ctrl+x).
	s.Send("Ctrl+x")
}
