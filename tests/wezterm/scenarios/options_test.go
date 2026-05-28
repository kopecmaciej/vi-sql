//go:build wezterm

package wezterm_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestStyleChangePersists(t *testing.T) {
	dsn := os.Getenv(harness.EnvDSN)
	if dsn == "" {
		t.Skipf("%s not set — skipping style test", harness.EnvDSN)
	}

	configPath, logPath := harness.NewTestConfig(t)
	s := harness.SpawnWithConfig(t, configPath, logPath, "--connect", dsn, "--debug")

	s.ChangeStyle()
	s.WaitForPaneTimeout(" Change Style ", 3*time.Second)
	s.MoveDown(1)
	s.Send("Enter")
	s.AssertPaneNotContains(" Change Style ")
	s.KillPane()

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if !strings.Contains(string(raw), "currentStyle:") {
		t.Errorf("style change not persisted: 'currentStyle' key missing from config")
	}
}
