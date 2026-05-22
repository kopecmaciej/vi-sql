//go:build wezterm

// Package harness provides primitives for driving vi-sql via wezterm
// keystroke injection. Use the "wezterm" build tag.
//
// Usage:
//
//	s := harness.Spawn(t, "--connection-name", "mydb", "--debug")
//	s.FocusSchemaTree()
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

	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

const (
	defaultLogPath   = "/tmp/vi-sql.log"
	defaultKeyDelay  = 40 * time.Millisecond
	slowKeyDelay     = 200 * time.Millisecond
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
	keys     *config.KeyBindings // nil if config loading failed
	vimMode  bool
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

	configPath := extractConfigArg(args)
	vm := configVimMode(configPath)

	s := &Session{
		t:        t,
		PaneID:   paneID,
		logPath:  logPath,
		logStart: logStart,
		keyDelay: keyDelayFromEnv(),
		vimMode:  vm,
		keys:     loadKeyBindings(vm),
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

// Paste copies text to the system clipboard and sends Ctrl+v to the pane.
func (s *Session) Paste(text string) {
	s.t.Helper()
	util.Copy(text)
	s.Send("Ctrl+v")
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

// mustKeys returns the loaded keybindings or fatals if they are unavailable.
func (s *Session) mustKeys() *config.KeyBindings {
	s.t.Helper()
	if s.keys == nil {
		s.t.Fatal("keybindings not loaded — check --config or config file path")
	}
	return s.keys
}

// sendAction sends all key names for the given binding. Sequences (e.g. "ge")
// are split into individual runes and sent one at a time.
func (s *Session) sendAction(k config.Key) {
	s.t.Helper()
	for _, key := range sendableKeys(k) {
		s.Send(key)
	}
}

// Dismiss closes the current modal or overlay using Common.Close.
func (s *Session) Dismiss() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Common.Close)
}

// Select confirms or opens the focused item using Common.Select.
func (s *Session) Select() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Common.Select)
}

// Filter opens the filter bar using Common.Filter.
func (s *Session) Filter() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Common.Filter)
}

// ClearField clears the current input field using Common.Clear.
func (s *Session) ClearField() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Common.Clear)
}

// MoveDown moves the cursor down n times using Navigation.MoveDown.
func (s *Session) MoveDown(n int) {
	s.t.Helper()
	k := s.mustKeys().Navigation.MoveDown
	for i := 0; i < n; i++ {
		s.sendAction(k)
	}
}

// MoveUp moves the cursor up n times using Navigation.MoveUp.
func (s *Session) MoveUp(n int) {
	s.t.Helper()
	k := s.mustKeys().Navigation.MoveUp
	for i := 0; i < n; i++ {
		s.sendAction(k)
	}
}

// NewTab opens a new query tab using Main.NewTab.
func (s *Session) NewTab() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Main.NewTab)
}

// CloseTab closes the active tab using Main.CloseTab.
func (s *Session) CloseTab() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Main.CloseTab)
}

// OpenServerInfo opens the server info modal using Main.ServerInfo.
func (s *Session) OpenServerInfo() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Main.ServerInfo)
}

// OpenHistory opens the SQL query history modal using SQLQueryEditor.OpenHistory.
func (s *Session) OpenHistory() {
	s.t.Helper()
	s.sendAction(s.mustKeys().SQLQueryEditor.OpenHistory)
}

// ChangeStyle opens the style-change modal using Global.ChangeStyle.
func (s *Session) ChangeStyle() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Global.ChangeStyle)
}

// ExpandSchemaNode expands the focused schema-tree node using Schema.ExpandTable.
func (s *Session) ExpandSchemaNode() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Schema.ExpandTable)
}

// CollapseAll collapses all schema-tree nodes using Schema.CollapseAll.
func (s *Session) CollapseAll() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Schema.CollapseAll)
}

// EditRow opens the inline-edit modal for the focused row using Data.EditRow.
func (s *Session) EditRow() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Data.EditRow)
}

// FormNext advances to the next field in a tview Form. Tab is tview's
// built-in form-navigation key and is not part of the vi-sql keybindings.
func (s *Session) FormNext() {
	s.t.Helper()
	s.Send("Ctrl+j")
}

// WriteToEditor pastes sql into the active SQL editor via the system clipboard.
// The SQL editor (TextArea) is always in insert mode, so no mode switching is needed.
func (s *Session) WriteToEditor(sql string) {
	s.t.Helper()
	if err := weztermSetClipboard(sql); err != nil {
		s.t.Fatalf("WriteToEditor: clipboard: %v", err)
	}
	s.Send("Ctrl+v")
}

// RunQuery executes the current editor content using the mode-aware Confirm key.
func (s *Session) RunQuery() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Common.Confirm)
}

// RunQueryInNewTab opens a new query tab, types sql, and executes it.
// It does NOT wait for results — call WaitForQueryResult or WaitForPane
// afterward to assert on success or failure.
func (s *Session) RunQueryInNewTab(sql string) {
	s.t.Helper()
	s.Send("Ctrl+t")
	s.AssertPaneContains("Query")
	s.WriteToEditor(sql)
	s.RunQuery()
}

// WaitForQueryResult waits for the query-timing indicator (⏱) to appear,
// which signals a successful query completion.
func (s *Session) WaitForQueryResult(timeout time.Duration) {
	s.t.Helper()
	s.WaitForPane("⏱", timeout)
}

// FocusSchemaTree sends the mode-aware focus-schema-tree binding.
func (s *Session) FocusSchemaTree() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Main.FocusSchemaTree)
}

// OpenActionsModal opens the actions modal using the mode-aware binding.
func (s *Session) OpenActionsModal() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Main.OpenActions)
}

// GoToTable opens the named table via the go-to-table modal and waits for
// the data grid to show row results. It uses the actions modal to reach the
// go-to-table modal, so focus must be on the main page (not inside a modal).
func (s *Session) GoToTable(schema, table string) {
	s.t.Helper()
	s.OpenActionsModal()
	s.AssertPaneContains("Actions")
	s.Type("go to")
	s.Send("Enter")
	s.WaitForPane("Go to table", 5*time.Second)
	s.Type(schema + "." + table)
	s.sendAction(s.mustKeys().Common.Confirm)
	s.WaitForPane(schema+"."+table, 10*time.Second)
}

// OpenExportModal opens the export modal. A data tab must be active.
func (s *Session) OpenExportModal() {
	s.t.Helper()
	s.Send("Alt+m")
}

// OpenImportModal opens the CSV import modal.
func (s *Session) OpenImportModal() {
	s.t.Helper()
	s.Send("Alt+i")
}

// AssertPaneNotContains fails the test if substr is still visible in the pane
// after assertionTimeout. Use it to confirm a modal has closed.
func (s *Session) AssertPaneNotContains(substr string) {
	s.t.Helper()
	deadline := time.Now().Add(assertionTimeout)
	for time.Now().Before(deadline) {
		text, err := weztermGetText(s.PaneID)
		if err != nil {
			s.t.Fatalf("AssertPaneNotContains: %v", err)
		}
		if !strings.Contains(text, substr) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	s.t.Errorf("AssertPaneNotContains: %q still visible in pane", substr)
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
	defer func() { _ = f.Close() }()
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

// DefaultConnection returns the value of VI_SQL_WEZTERM_CONNECTION, falling
// back to the currently active connection from `vi-sql -l` when the variable
// is unset. Returns "" when no connection is available.
func DefaultConnection() string {
	if v := os.Getenv("VI_SQL_WEZTERM_CONNECTION"); v != "" {
		return v
	}
	return CurrentConnectionName()
}

// DefaultJump returns the value of VI_SQL_WEZTERM_JUMP, falling back to
// "auth.users" when the variable is unset.
func DefaultJump() string {
	if v := os.Getenv("VI_SQL_WEZTERM_JUMP"); v != "" {
		return v
	}
	return "auth.users"
}

// ParseJump splits a jump target of the form "schema.table" into its parts.
func ParseJump(jump string) (schema, table string, ok bool) {
	parts := strings.SplitN(jump, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
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
