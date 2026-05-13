//go:build wezterm

package wezterm_test

import (
	"os"
	"testing"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

// TestMain sets project-specific defaults for env vars that control the
// harness. Values already set in the environment take precedence, so you
// can still override any of these on the command line.
func TestMain(m *testing.M) {
	setDefault("VI_SQL_WEZTERM_CONNECTION", harness.CurrentConnectionName())
	setDefault("VI_SQL_WEZTERM_JUMP", "auth/users")
	os.Exit(m.Run())
}

func setDefault(key, value string) {
	if os.Getenv(key) == "" {
		os.Setenv(key, value)
	}
}
