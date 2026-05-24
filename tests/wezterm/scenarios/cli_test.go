//go:build wezterm

package wezterm_test

import (
	"strings"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

// TestCLIHelp verifies that --help lists the documented flags.
func TestCLIHelp(t *testing.T) {
	out := harness.RunBinary(t, "--help")
	for _, flag := range []string{"--connect", "--connection-name", "--jump", "--debug", "--config"} {
		if !strings.Contains(out, flag) {
			t.Errorf("--help: expected %q in output, got:\n%s", flag, out)
		}
	}
}

// TestCLIConnectionList verifies that -l prints available connections and marks
// the current one with *.
func TestCLIConnectionList(t *testing.T) {
	out := harness.RunBinaryWithSavedConnection(t, "-l")
	if !strings.Contains(out, "*") {
		t.Errorf("-l: expected current connection to be marked with *, got:\n%s", out)
	}
}

// TestCLIPaths verifies that --paths prints the locations of the main config files.
func TestCLIPaths(t *testing.T) {
	out := harness.RunBinary(t, "--paths")
	for _, keyword := range []string{"config", "keybinding", "style", "log"} {
		if !strings.Contains(strings.ToLower(out), keyword) {
			t.Errorf("--paths: expected %q keyword in output, got:\n%s", keyword, out)
		}
	}
}

// TestCLIOptionsPage verifies that --options-page causes the options page to render
// on startup instead of the connection page.
func TestCLIOptionsPage(t *testing.T) {
	s := harness.Spawn(t, "--options-page", "--debug")
	s.AssertPaneContains("Options")
}

// TestCLIConnectionPage verifies that --connection-page causes the connection list
// to render on startup (requires at least one saved connection).
func TestCLIConnectionPage(t *testing.T) {
	s := harness.SpawnWithSavedConnection(t, "--connection-page", "--debug")
	s.AssertPaneContains("Connection")
}

func TestCLIJumpToMissingTable(t *testing.T) {
	s := harness.SpawnConnected(t, "--jump", "nonexistent_schema.nonexistent_table", "--debug")
	s.WaitForPane("Unable to jump", 10*time.Second)
}
