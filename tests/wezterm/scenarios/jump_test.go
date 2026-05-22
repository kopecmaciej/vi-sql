//go:build wezterm

package wezterm_test

import (
	"os"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

// TestJumpFlag verifies that --jump schema/table bypasses the schema tree and
// opens the named table directly.
//
// Prerequisites:
//   - VI_SQL_WEZTERM_CONNECTION must be set to a pre-configured connection name.
//   - VI_SQL_WEZTERM_JUMP must be set to a valid "schema/table" target, e.g.:
//     export VI_SQL_WEZTERM_JUMP=public/users
func TestJumpFlag(t *testing.T) {
	conn := os.Getenv("VI_SQL_WEZTERM_CONNECTION")
	if conn == "" {
		t.Skip("VI_SQL_WEZTERM_CONNECTION not set — skipping jump scenario")
	}
	jump := os.Getenv("VI_SQL_WEZTERM_JUMP")
	if jump == "" {
		t.Skip("VI_SQL_WEZTERM_JUMP not set (e.g. export VI_SQL_WEZTERM_JUMP=auth/users)")
	}

	s := harness.Spawn(t, "--connection-name", conn, "--jump", jump, "--debug")

	// Wait for the results bar — ⏱ appears after any query completes.
	s.WaitForPane("⏱", 10*time.Second)

	// The tab title should contain the table name.
	if _, table, ok := harness.ParseJump(jump); ok {
		s.AssertPaneContains(table)
	}
}
