//go:build wezterm

package wezterm_test

import (
	"os"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

// TestConnectTwiceSameDSN verifies that running --connect with the same DSN
// twice (across two separate processes sharing a config) opens the main page
// with data on both runs — i.e. the second run doesn't error on a duplicate
// connection and still navigates to the table.
func TestConnectTwiceSameDSN(t *testing.T) {
	dsn := os.Getenv(harness.EnvDSN)
	if dsn == "" {
		t.Skipf("%s not set — skipping reconnect scenario", harness.EnvDSN)
	}
	jump := harness.DefaultJump()
	_, table, ok := harness.ParseJump(jump)
	if !ok {
		t.Skipf("jump target %q is not in schema.table format", jump)
	}

	// Shared config — both sessions read and write the same file so the
	// second --connect sees the connection saved by the first.
	configPath, logPath := harness.NewTestConfig(t)

	s1 := harness.SpawnWithConfig(t, configPath, logPath, "--connect", dsn, "--jump", jump, "--debug")
	s1.WaitForPane("⏱", 10*time.Second)
	s1.AssertPaneContains(table)
	s1.KillPane()

	s2 := harness.SpawnWithConfig(t, configPath, logPath, "--connect", dsn, "--jump", jump, "--debug")
	s2.WaitForPane("⏱", 10*time.Second)
	s2.AssertPaneContains(table)
}
