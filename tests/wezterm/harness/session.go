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
//	VI_SQL_TESTS_DSN          database DSN for SpawnWithTable (required; e.g. postgresql://user:pass@host/db)
//	VI_SQL_TESTS_JUMP         schema.table to open (default: auth.users)
//	VI_SQL_TESTS_BINARY       path to vi-sql binary (default: .build/vi-sql)
//	VI_SQL_TESTS_LOG_PATH     path to vi-sql log file (default: /tmp/vi-sql.log)
//	VI_SQL_TESTS_KEY_DELAY_MS delay in ms between keystrokes (default: 40)
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

	binary := requireBinary(t)

	logPath := os.Getenv(EnvLogPath)
	if logPath == "" {
		logPath = defaultLogPath
	}
	logStart := currentFileSize(logPath)

	// Inject a test-local config (trace level, plain JSON logs, isolated log file)
	// unless the caller already provided --config/-c.
	if !hasArg(args, "--config", "-c") {
		configPath, testLogPath := newTestConfig(t)
		args = append([]string{"--config", configPath}, args...)
		logPath = testLogPath
		logStart = 0
	}

	return spawnSession(t, binary, args, logPath, logStart)
}

// NewTestConfig creates a fresh isolated test config and returns its path and
// log path. Use it when a test needs to share one config across multiple Spawn
// calls (e.g. to test reconnect behaviour). Pass the paths to SpawnWithConfig.
func NewTestConfig(t *testing.T) (configPath, logPath string) {
	t.Helper()
	return newTestConfig(t)
}

// SpawnWithConfig spawns vi-sql using an already-created config at configPath,
// tracking logs at logPath from the current file offset. Use together with
// NewTestConfig when multiple sessions must share the same config.
func SpawnWithConfig(t *testing.T, configPath, logPath string, args ...string) *Session {
	t.Helper()
	binary := requireBinary(t)
	logStart := currentFileSize(logPath)
	args = append([]string{"--config", configPath}, args...)
	return spawnSession(t, binary, args, logPath, logStart)
}

// SpawnConnected spawns vi-sql connected directly via VI_SQL_TESTS_DSN.
// Skips if the env var is unset. Use this for tests that need a live DB
// but don't care about the connection-list UI.
func SpawnConnected(t *testing.T, args ...string) *Session {
	t.Helper()
	dsn := os.Getenv(EnvDSN)
	if dsn == "" {
		t.Skipf("%s not set — skipping test", EnvDSN)
		return nil
	}
	return Spawn(t, append([]string{"--connect", dsn}, args...)...)
}

// SpawnWithSavedConnection spawns vi-sql with a test config that has a saved
// connection seeded from VI_SQL_TESTS_DSN. The app shows the connection list
// on startup rather than the pick-driver page. Skips if the DSN env var is unset.
func SpawnWithSavedConnection(t *testing.T, args ...string) *Session {
	t.Helper()

	dsn := os.Getenv(EnvDSN)
	if dsn == "" {
		t.Skipf("%s not set — skipping test", EnvDSN)
		return nil
	}

	binary := requireBinary(t)
	configPath, logPath := newTestConfigWithDSN(t, dsn)
	args = append([]string{"--config", configPath}, args...)
	return spawnSession(t, binary, args, logPath, 0)
}

// findBinary returns the vi-sql binary path, skipping the test if not found.
func findBinary(t *testing.T) string {
	t.Helper()
	binary := os.Getenv(EnvBinary)
	if binary == "" {
		binary = filepath.Join(projectRoot(), ".build", "vi-sql")
	}
	abs, err := filepath.Abs(binary)
	if err != nil || !fileExists(abs) {
		t.Skipf("binary %q not found (run 'make build' first)", binary)
	}
	return abs
}

// requireBinary is like findBinary but also verifies wezterm is available.
func requireBinary(t *testing.T) string {
	t.Helper()
	if err := weztermCheckAvailable(); err != nil {
		t.Skipf("skipping wezterm: %v", err)
	}
	return findBinary(t)
}

// RunBinary runs the vi-sql binary with args and returns combined stdout+stderr.
// A fresh isolated test config is injected via --config unless already provided.
// Skips the test if the binary is not found.
func RunBinary(t *testing.T, args ...string) string {
	t.Helper()
	binary := findBinary(t)
	if !hasArg(args, "--config", "-c") {
		configPath, _ := newTestConfig(t)
		args = append([]string{"--config", configPath}, args...)
	}
	cmd := exec.Command(binary, args...)
	out, _ := cmd.CombinedOutput()
	return string(out)
}

// RunBinaryWithSavedConnection runs the vi-sql binary with a test config that
// has a saved connection seeded from VI_SQL_TESTS_DSN. Use for CLI commands
// that read the connection list (e.g. -l). Skips if the DSN env var is unset.
func RunBinaryWithSavedConnection(t *testing.T, args ...string) string {
	t.Helper()
	dsn := os.Getenv(EnvDSN)
	if dsn == "" {
		t.Skipf("%s not set — skipping test", EnvDSN)
		return ""
	}
	binary := findBinary(t)
	configPath, _ := newTestConfigWithDSN(t, dsn)
	args = append([]string{"--config", configPath}, args...)
	cmd := exec.Command(binary, args...)
	out, _ := cmd.CombinedOutput()
	return string(out)
}

// spawnSession creates the wezterm pane, builds a Session, and waits for the
// app to be ready. Binary and config args must already be resolved.
func spawnSession(t *testing.T, binary string, args []string, logPath string, logStart int64) *Session {
	t.Helper()

	paneID, err := weztermSpawn(binary, args)
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
	t.Cleanup(s.KillPane)

	s.WaitForLog(readyMarker, startupTimeout)

	// When a connection flag is passed, wait for the DB connection result.
	// "Vi-SQL started" is logged before the tview event loop runs; the actual
	// connection attempt is async and happens later.
	for i, a := range args {
		if (a == "--connection-name" || a == "-n" || a == "--connect") && i+1 < len(args) {
			found := s.waitForOneOf(startupTimeout, connectedMarker, "Failed to connect to database")
			if found != connectedMarker {
				s.t.Fatalf("Spawn: DB connection failed — check DSN or stored password")
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

func (s *Session) IsVimMode() bool {
	return s.vimMode
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
		time.Sleep(100 * time.Millisecond)
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

// KillPane kills the wezterm pane. Registered automatically by Spawn via t.Cleanup.
func (s *Session) KillPane() {
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

// Close closes the current modal or overlay using Common.Close.
func (s *Session) Close() {
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
	for range n {
		s.sendAction(s.mustKeys().Navigation.MoveDown)
	}
}

// MoveUp moves the cursor up n times using Navigation.MoveUp.
func (s *Session) MoveUp(n int) {
	s.t.Helper()
	for range n {
		s.sendAction(s.mustKeys().Navigation.MoveUp)
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

func (s *Session) Edit() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Common.Edit)
}

func (s *Session) EditRow() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Data.EditRow)
}

// FormNext advances to the next field in a tview Form. Tab is tview's
// built-in form-navigation key and is not part of the vi-sql keybindings.
func (s *Session) FormNext() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Navigation.FocusDown)
}

// WriteToEditor pastes sql into the active SQL editor via the system clipboard.
// The SQL editor (TextArea) is always in insert mode, so no mode switching is needed.
func (s *Session) WriteToEditor(sql string) {
	s.t.Helper()
	if err := weztermSetClipboard(sql); err != nil {
		s.t.Fatalf("WriteToEditor: clipboard: %v", err)
	}
	s.sendAction(s.mustKeys().Common.Paste)
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
	s.sendAction(s.mustKeys().Main.NewTab)
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
	s.sendAction(s.mustKeys().Data.ExportData)
}

// OpenImportModal opens the CSV import modal.
func (s *Session) OpenImportModal() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Main.ImportData)
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

// GetFocusedElement returns the identifier of the most recently focused element
// by scanning the log for focus-change JSON lines. Returns "" when none found.
func (s *Session) GetFocusedElement() string {
	f, err := os.Open(s.logPath)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Seek(s.logStart, 0); err != nil {
		return ""
	}
	last := ""
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, `"focus changed"`) {
			continue
		}
		if _, after, ok := strings.Cut(line, `"element":"`); ok {
			if val, _, ok := strings.Cut(after, `"`); ok && val != "" {
				last = val
			}
		}
	}
	return last
}

// WaitForFocus blocks until the element with the given identifier is focused
// or fails the test after timeout. This is useful before sending keystrokes
// that depend on a specific component having focus.
func (s *Session) WaitForFocus(element string, timeout time.Duration) {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s.GetFocusedElement() == element {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	s.t.Fatalf("WaitForFocus: element %q not focused within %v (currently focused: %q)", element, timeout, s.GetFocusedElement())
}

// LogFocus logs the currently focused element identifier. Drop this anywhere in
// a test to see what has focus at that point.
func (s *Session) LogFocus() {
	s.t.Helper()
	s.t.Logf("focus: %q", s.GetFocusedElement())
}

// WaitForFile blocks until the file at path exists or fails the test after timeout.
func (s *Session) WaitForFile(path string, timeout time.Duration) {
	s.t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	s.t.Fatalf("WaitForFile: %q not created within %v", path, timeout)
}

// SpawnWithTable starts vi-sql connected via VI_SQL_TESTS_DSN and jumps
// directly to VI_SQL_TESTS_JUMP (default: auth.users). Skips the test when
// either variable is unset or the jump format is invalid.
func SpawnWithTable(t *testing.T) (*Session, string) {
	t.Helper()
	dsn := os.Getenv(EnvDSN)
	if dsn == "" {
		t.Skipf("%s not set — skipping scenario", EnvDSN)
		return nil, ""
	}
	jump := DefaultJump()
	_, table, ok := ParseJump(jump)
	if !ok {
		t.Skipf("jump target %q is not in schema.table format", jump)
		return nil, ""
	}
	s := Spawn(t, "--connect", dsn, "--jump", jump)
	s.WaitForPane("⏱", 10*time.Second)
	return s, table
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

// DefaultConnection returns the value of VI_SQL_TESTS_CONNECTION, falling
// back to the currently active connection from `vi-sql -l` when the variable
// is unset. Returns "" when no connection is available.
func DefaultConnection() string {
	if v := os.Getenv(EnvConnection); v != "" {
		return v
	}
	return CurrentConnectionName()
}

// DefaultJump returns the value of VI_SQL_TESTS_JUMP, falling back to
// "auth.users" when the variable is unset.
func DefaultJump() string {
	if v := os.Getenv(EnvJump); v != "" {
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
	binary := os.Getenv(EnvBinary)
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
	for line := range strings.SplitSeq(string(out), "\n") {
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
	if os.Getenv(EnvSlow) == "1" {
		return slowKeyDelay
	}
	if v := os.Getenv(EnvKeyDelayMS); v != "" {
		var ms int
		if _, err := fmt.Sscan(v, &ms); err == nil && ms > 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return defaultKeyDelay
}
