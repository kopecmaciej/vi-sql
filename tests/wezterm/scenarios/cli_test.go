//go:build wezterm

package wezterm_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

// TestCLIHelp verifies that --help lists the documented flags.
func TestCLIHelp(t *testing.T) {
	out := runBinary(t, "--help")
	for _, flag := range []string{"--connect", "--connection-name", "--jump", "--debug", "--config"} {
		if !strings.Contains(out, flag) {
			t.Errorf("--help: expected %q in output, got:\n%s", flag, out)
		}
	}
}

// TestCLIConnectionList verifies that -l prints available connections and marks
// the current one with *.
func TestCLIConnectionList(t *testing.T) {
	out := runBinary(t, "-l")
	if out == "" {
		t.Skip("no connections configured — skipping connection-list test")
	}
	if !strings.Contains(out, "*") {
		t.Errorf("-l: expected current connection to be marked with *, got:\n%s", out)
	}
}

// TestCLIPaths verifies that --paths prints the locations of the main config files.
func TestCLIPaths(t *testing.T) {
	out := runBinary(t, "--paths")
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

// TestCLIConnectionPage verifies that --connection-page causes the connection page
// to render on startup.
func TestCLIConnectionPage(t *testing.T) {
	s := harness.Spawn(t, "--connection-page", "--debug")
	s.AssertPaneContains("Connection")
}

// runBinary runs the vi-sql binary with args and returns combined stdout+stderr.
// The test is skipped if the binary is not found.
func runBinary(t *testing.T, args ...string) string {
	t.Helper()
	binary := os.Getenv("VI_SQL_WEZTERM_BINARY")
	if binary == "" {
		_, file, _, _ := runtime.Caller(0)
		// tests/wezterm/scenarios/ → three levels up → project root
		root := filepath.Join(filepath.Dir(file), "..", "..", "..")
		binary = filepath.Join(root, ".build", "vi-sql")
	}
	abs, err := filepath.Abs(binary)
	if err != nil || func() bool { _, e := os.Stat(abs); return e != nil }() {
		t.Skipf("binary %q not found (run 'make build' first)", binary)
	}
	cmd := exec.Command(abs, args...)
	out, _ := cmd.CombinedOutput() // ignore exit code — --help/--version may exit non-zero
	return string(out)
}
