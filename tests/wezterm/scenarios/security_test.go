//go:build wezterm

package wezterm_test

import (
	"os"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestMasterPasswordUnlock(t *testing.T) {
	configPath, logPath := harness.NewTestConfigWithMasterPassword(t, "test_master_pw_123")
	s := harness.SpawnWithConfig(t, configPath, logPath)

	s.WaitForPaneTimeout(" Unlock with master password ", 10*time.Second)
	s.Paste("test_master_pw_123")
	s.Send("Enter")
	s.Send("Enter")

	s.WaitForPane("Schemas")
	s.AssertPaneNotContains(" Unlock with master password ")
}

func TestMasterPasswordWrongPassword(t *testing.T) {
	configPath, logPath := harness.NewTestConfigWithMasterPassword(t, "correct_password")
	s := harness.SpawnWithConfig(t, configPath, logPath)

	s.WaitForPaneTimeout(" Unlock with master password ", 10*time.Second)
	s.Paste("wrong_password")
	s.Send("Enter")

	s.WaitForPane("Wrong password")
	s.AssertPaneContains(" Unlock with master password ")
}

func TestMasterPasswordSetup(t *testing.T) {
	configPath, logPath := harness.NewTestConfigWithSecurityMethod(t, "master")
	s := harness.SpawnWithConfig(t, configPath, logPath)

	s.WaitForPane(" Set master password ")
	s.Paste("brand_new_password")
	s.FocusDown(1)
	s.Paste("brand_new_password")
	s.Send("Enter")

	s.WaitForPane("Pick Driver")
	s.AssertPaneNotContains(" Set master password ")
}

func TestSecurityEnv(t *testing.T) {
	dsn := os.Getenv(harness.EnvDSN)
	if dsn == "" {
		t.Skipf("%s not set — skipping env security test", harness.EnvDSN)
	}
	configPath, logPath := harness.NewTestConfigWithSecurityMethod(t, "env")
	s := harness.SpawnWithConfig(t, configPath, logPath, "--connect", dsn)

	s.WaitForPaneTimeout("Schemas", 10*time.Second)
	s.AssertPaneNotContains("master password")
}

func TestSecurityOff(t *testing.T) {
	dsn := os.Getenv(harness.EnvDSN)
	if dsn == "" {
		t.Skipf("%s not set — skipping off security test", harness.EnvDSN)
	}
	configPath, logPath := harness.NewTestConfigWithSecurityMethod(t, "off")
	s := harness.SpawnWithConfig(t, configPath, logPath, "--connect", dsn)

	s.WaitForPaneTimeout("Schemas", 10*time.Second)
	s.AssertPaneNotContains("master password")
}
