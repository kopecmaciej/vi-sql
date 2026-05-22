//go:build wezterm

package wezterm_test

import (
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

// via --connect
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

	configPath, logPath := harness.NewTestConfig(t)

	s1 := harness.SpawnWithConfig(t, configPath, logPath, "--connect", dsn, "--jump", jump, "--debug")
	s1.WaitForPane("⏱", 10*time.Second)
	s1.AssertPaneContains(table)
	s1.KillPane()

	s2 := harness.SpawnWithConfig(t, configPath, logPath, "--connect", dsn, "--jump", jump, "--debug")
	s2.WaitForPane("⏱", 10*time.Second)
	s2.AssertPaneContains(table)
}

func TestConnectSameNameDifferentDSNErrors(t *testing.T) {
	dsn := os.Getenv(harness.EnvDSN)
	if dsn == "" {
		t.Skipf("%s not set — skipping reconnect scenario", harness.EnvDSN)
	}
	jump := harness.DefaultJump()
	_, table, ok := harness.ParseJump(jump)
	if !ok {
		t.Skipf("jump target %q is not in schema.table format", jump)
	}

	u, err := url.Parse(dsn)
	if err != nil || u.Scheme == "" {
		t.Skipf("cannot build conflicting DSN from %q", dsn)
	}
	u.Path = "/vis_sql_conflict_test"
	conflictDSN := u.String()

	configPath, logPath := harness.NewTestConfig(t)

	s1 := harness.SpawnWithConfig(t, configPath, logPath, "--connect", "test-conn="+dsn, "--jump", jump)
	s1.WaitForPane("⏱", 10*time.Second)
	s1.AssertPaneContains(table)
	s1.KillPane()

	out := harness.RunBinary(t, "--config", configPath, "--connect", "test-conn="+conflictDSN)
	if !strings.Contains(out, "already exists with a different DSN") {
		t.Errorf("expected conflict error in output, got:\n%s", out)
	}
}
