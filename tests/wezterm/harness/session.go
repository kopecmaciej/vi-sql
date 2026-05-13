//go:build wezterm

// Package harness provides primitives for driving vi-sql via wezterm
// keystroke injection. Use the "wezterm" build tag.
//
// Usage:
//
//	s := harness.Spawn(t, "--connection-name", "mydb", "--debug")
//	s.Send("Ctrl+/")       // focus schema tree
//	s.Send("Down", "Down") // navigate to 3rd schema
//	s.Send("e")            // expand schema
//	s.Send("Enter")        // open table
//	s.WaitForLog("SELECT", 10*time.Second)
//	s.AssertPaneContains("Rows:")
//
// Environment variables:
//
//	VI_SQL_WEZTERM_BINARY      path to vi-sql binary (default: .build/vi-sql)
//	VI_SQL_WEZTERM_LOG_PATH    path to vi-sql log file (default: /tmp/vi-sql.log)
//	VI_SQL_WEZTERM_KEY_DELAY_MS delay in ms between keystrokes (default: 40)
package harness

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	defaultLogPath   = "/tmp/vi-sql.log"
	defaultKeyDelay  = 40 * time.Millisecond
	slowKeyDelay     = 300 * time.Millisecond
	readyMarker      = "Vi-SQL started"
	connectedMarker  = "Connected to database"
	startupTimeout   = 15 * time.Second
	assertionTimeout = 3 * time.Second
)

// Session represents a running vi-sql instance in a wezterm pane.
type Session struct {
	t        *testing.T
	PaneID   string
	logPath  string
	logStart int64
	keyDelay time.Duration
}

// Spawn builds the path to vi-sql, starts it in a new wezterm pane with the
// given extra args, and waits until the app logs "Vi-SQL started" before
// returning. The pane is killed automatically via t.Cleanup.
//
// The test is skipped (not failed) if wezterm is unavailable or the binary
// doesn't exist.
func Spawn(t *testing.T, args ...string) *Session {
	t.Helper()

	if err := weztermCheckAvailable(); err != nil {
		t.Skipf("skipping wezterm: %v", err)
	}

	binary := os.Getenv("VI_SQL_WEZTERM_BINARY")
	if binary == "" {
		binary = filepath.Join(projectRoot(), ".build", "vi-sql")
	}
	abs, err := filepath.Abs(binary)
	if err != nil || !fileExists(abs) {
		t.Skipf("skipping wezterm: binary %q not found (run 'make build' first)", binary)
	}

	logPath := os.Getenv("VI_SQL_WEZTERM_LOG_PATH")
	if logPath == "" {
		logPath = defaultLogPath
	}

	// Snapshot log size before spawning so assertions only scan new lines.
	logStart := currentFileSize(logPath)

	paneID, err := weztermSpawn(abs, args)
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	s := &Session{
		t:        t,
		PaneID:   paneID,
		logPath:  logPath,
		logStart: logStart,
		keyDelay: keyDelayFromEnv(),
	}
	t.Cleanup(s.Close)

	s.WaitForLog(readyMarker, startupTimeout)

	// When --connection-name/-n is passed, wait for the DB connection result.
	// "Vi-SQL started" is logged before the tview event loop runs; the actual
	// connection attempt is async and happens later.
	for i, a := range args {
		if (a == "--connection-name" || a == "-n") && i+1 < len(args) {
			found := s.waitForOneOf(startupTimeout, connectedMarker, "Failed to connect to database")
			if found != connectedMarker {
				s.t.Fatalf("Spawn: DB connection failed for %q — check stored password or config", args[i+1])
			}
			break
		}
	}

	return s
}

// Send injects one or more named keys into the pane with a small delay
// between each. Key names follow vi-sql's keybindings format:
// "Enter", "Esc", "Ctrl+t", "Down", "/", "e", etc.
func (s *Session) Send(keys ...string) {
	s.t.Helper()
	for _, k := range keys {
		seq, err := Encode(k)
		if err != nil {
			s.t.Fatalf("Send: %v", err)
		}
		if err := weztermSendText(s.PaneID, seq); err != nil {
			s.t.Fatalf("Send %q: %v", k, err)
		}
		time.Sleep(s.keyDelay)
	}
}

// Type injects literal text character by character (not as a bracketed paste).
func (s *Session) Type(text string) {
	s.t.Helper()
	for _, ch := range text {
		if err := weztermSendText(s.PaneID, string(ch)); err != nil {
			s.t.Fatalf("Type char %q: %v", ch, err)
		}
		time.Sleep(s.keyDelay)
	}
}

// WaitForLog blocks until substr appears in the log (from the offset captured
// at Spawn time) or fails the test after timeout.
func (s *Session) WaitForLog(substr string, timeout time.Duration) {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s.logContains(substr) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	s.t.Fatalf("WaitForLog: %q not found in %s within %v", substr, s.logPath, timeout)
}

// AssertLogContains fails the test if substr does not appear in the log within
// assertionTimeout.
func (s *Session) AssertLogContains(substr string) {
	s.t.Helper()
	deadline := time.Now().Add(assertionTimeout)
	for time.Now().Before(deadline) {
		if s.logContains(substr) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	s.t.Errorf("AssertLogContains: %q not found in log", substr)
}

// AssertPaneContains fails the test if substr is not visible in the pane
// within assertionTimeout. Retries to allow for render latency.
func (s *Session) AssertPaneContains(substr string) {
	s.t.Helper()
	s.waitForPane(substr, assertionTimeout, false)
}

// WaitForPane blocks until substr is visible in the pane or fails the test
// after timeout. Use this (instead of AssertPaneContains) when the operation
// may take several seconds — e.g. waiting for a DB query to return.
func (s *Session) WaitForPane(substr string, timeout time.Duration) {
	s.t.Helper()
	s.waitForPane(substr, timeout, true)
}

func (s *Session) waitForPane(substr string, timeout time.Duration, fatal bool) {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		text, err := weztermGetText(s.PaneID)
		if err != nil {
			s.t.Fatalf("pane assertion: %v", err)
		}
		if strings.Contains(text, substr) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	if fatal {
		s.t.Fatalf("WaitForPane: %q not visible in pane within %v", substr, timeout)
	} else {
		s.t.Errorf("AssertPaneContains: %q not visible in pane", substr)
	}
}

// TypeQuery types a multi-line SQL query into the active editor.
// Each \n in sql is sent as an Enter keystroke.
// When using vim mode, the caller must ensure the editor is in insert mode first (send "i").
func (s *Session) TypeQuery(sql string) {
	s.t.Helper()
	lines := strings.Split(sql, "\n")
	for i, line := range lines {
		s.Type(line)
		if i < len(lines)-1 {
			s.Send("Enter")
		}
	}
}

// GetPaneText returns the current visible text of the pane.
func (s *Session) GetPaneText() string {
	s.t.Helper()
	text, err := weztermGetText(s.PaneID)
	if err != nil {
		s.t.Fatalf("GetPaneText: %v", err)
	}
	return text
}

// Close kills the wezterm pane. Registered automatically by Spawn via t.Cleanup.
func (s *Session) Close() {
	_ = weztermKillPane(s.PaneID)
}

// waitForOneOf polls the log until one of substrs appears and returns the match,
// or fails the test after timeout. Used to detect either success or failure logs.
func (s *Session) waitForOneOf(timeout time.Duration, substrs ...string) string {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, sub := range substrs {
			if s.logContains(sub) {
				return sub
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	s.t.Fatalf("waitForOneOf: none of %v found in log within %v", substrs, timeout)
	return ""
}

func (s *Session) logContains(substr string) bool {
	f, err := os.Open(s.logPath)
	if err != nil {
		return false
	}
	defer f.Close()
	if _, err := f.Seek(s.logStart, 0); err != nil {
		return false
	}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), substr) {
			return true
		}
	}
	return false
}

func currentFileSize(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// projectRoot returns the absolute path of the module root by walking up from
// this source file's location (tests/wezterm/harness/ → three levels up).
func projectRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

// CurrentConnectionName returns the name of the connection currently marked as
// active in vi-sql's config (the one shown with * in `vi-sql -l`). Returns an
// empty string if the binary is unavailable or no current connection is found.
func CurrentConnectionName() string {
	binary := os.Getenv("VI_SQL_WEZTERM_BINARY")
	if binary == "" {
		binary = filepath.Join(projectRoot(), ".build", "vi-sql")
	}
	abs, err := filepath.Abs(binary)
	if err != nil || !fileExists(abs) {
		return ""
	}
	out, err := exec.Command(abs, "-l").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "*") {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(strings.TrimSpace(line), "*"))
		if len(fields) > 0 {
			return fields[0]
		}
	}
	return ""
}

func keyDelayFromEnv() time.Duration {
	if os.Getenv("VI_SQL_WEZTERM_SLOW") == "1" {
		return slowKeyDelay
	}
	if v := os.Getenv("VI_SQL_WEZTERM_KEY_DELAY_MS"); v != "" {
		var ms int
		if _, err := fmt.Sscan(v, &ms); err == nil && ms > 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return defaultKeyDelay
}
