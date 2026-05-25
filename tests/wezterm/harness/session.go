//go:build wezterm

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
	defaultTable     = "auth.users"
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

func Spawn(t *testing.T, args ...string) *Session {
	t.Helper()

	binary := requireBinary(t)

	logPath := os.Getenv(EnvLogPath)
	if logPath == "" {
		logPath = defaultLogPath
	}
	logStart := currentFileSize(logPath)

	if !hasArg(args, "--config", "-c") {
		configPath, testLogPath := newTestConfig(t)
		args = append([]string{"--config", configPath}, args...)
		logPath = testLogPath
		logStart = 0
	}

	return spawnSession(t, binary, args, logPath, logStart)
}

func NewTestConfig(t *testing.T) (configPath, logPath string) {
	t.Helper()
	return newTestConfig(t)
}

func SpawnWithConfig(t *testing.T, configPath, logPath string, args ...string) *Session {
	t.Helper()
	binary := requireBinary(t)
	logStart := currentFileSize(logPath)
	args = append([]string{"--config", configPath}, args...)
	return spawnSession(t, binary, args, logPath, logStart)
}

func SpawnConnected(t *testing.T, args ...string) *Session {
	t.Helper()
	dsn := os.Getenv(EnvDSN)
	if dsn == "" {
		t.Skipf("%s not set — skipping test", EnvDSN)
		return nil
	}
	return Spawn(t, append([]string{"--connect", dsn}, args...)...)
}

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

func requireBinary(t *testing.T) string {
	t.Helper()
	if err := weztermCheckAvailable(); err != nil {
		t.Skipf("skipping wezterm: %v", err)
	}
	return findBinary(t)
}

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

	// "Vi-SQL started" is logged before the event loop; connection is async.
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

func (s *Session) Paste(text string) {
	s.t.Helper()
	util.Copy(text)
	s.Send("Ctrl+v")
}

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

func (s *Session) AssertPaneContains(substr string) {
	s.t.Helper()
	s.waitForPane(substr, assertionTimeout, false)
}

// WaitForPane is like AssertPaneContains but with a caller-specified timeout.
// Use it when the operation may take several seconds (e.g. a DB query).
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

func (s *Session) GetPaneText() string {
	s.t.Helper()
	text, err := weztermGetText(s.PaneID)
	if err != nil {
		s.t.Fatalf("GetPaneText: %v", err)
	}
	return text
}

func (s *Session) KillPane() {
	_ = weztermKillPane(s.PaneID)
}

func (s *Session) mustKeys() *config.KeyBindings {
	s.t.Helper()
	if s.keys == nil {
		s.t.Fatal("keybindings not loaded — check --config or config file path")
	}
	return s.keys
}

func (s *Session) sendAction(k config.Key) {
	s.t.Helper()
	for _, key := range sendableKeys(k) {
		s.Send(key)
	}
}

func (s *Session) Close() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Common.Close)
}

func (s *Session) Select() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Common.Select)
}

func (s *Session) Filter() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Common.Filter)
}

func (s *Session) ClearField() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Common.Clear)
}

func (s *Session) MoveUp(n int) {
	s.t.Helper()
	for range n {
		s.sendAction(s.mustKeys().Navigation.MoveUp)
	}
}

func (s *Session) MoveDown(n int) {
	s.t.Helper()
	for range n {
		s.sendAction(s.mustKeys().Navigation.MoveDown)
	}
}

func (s *Session) MoveLeft(n int) {
	s.t.Helper()
	for range n {
		s.sendAction(s.mustKeys().Navigation.MoveLeft)
	}

}

func (s *Session) MoveRight(n int) {
	s.t.Helper()
	for range n {
		s.sendAction(s.mustKeys().Navigation.MoveRight)
	}
}

// FocusDown advances to the next field in a tview Form. Tab is tview's
// built-in form-navigation key and is not part of the vi-sql keybindings.
func (s *Session) FocusDown(n int) {
	s.t.Helper()
	for range n {
		s.sendAction(s.mustKeys().Navigation.FocusDown)
	}

}

func (s *Session) FocusUp(n int) {
	s.t.Helper()
	for range n {
		s.sendAction(s.mustKeys().Navigation.FocusUp)
	}
}

func (s *Session) NewTab() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Main.NewTab)
}

func (s *Session) CloseTab() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Main.CloseTab)
}

func (s *Session) OpenServerInfo() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Main.ServerInfo)
}

func (s *Session) OpenHistory() {
	s.t.Helper()
	s.sendAction(s.mustKeys().SQLQueryEditor.OpenHistory)
}

func (s *Session) ChangeStyle() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Global.ChangeStyle)
}

func (s *Session) ExpandSchemaNode() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Schema.ExpandTable)
}

func (s *Session) CollapseAll() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Schema.CollapseAll)
}

func (s *Session) ExpandAll() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Schema.ExpandAll)
}

func (s *Session) Edit() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Common.Edit)
}

func (s *Session) EditRow() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Data.EditRow)
}

func (s *Session) DuplicateRow() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Data.DuplicateRow)
}

// Add sends Common.Add: opens the Add Row editor in a data view, or the
// Create Table modal when the schema tree is focused.
func (s *Session) Add() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Common.Add)
}

// Delete sends Common.Delete: deletes the selected row in a data view, or
// opens the drop-table confirmation when the schema tree is focused.
func (s *Session) Delete() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Common.Delete)
}

// Confirm sends Common.Confirm, which executes the form-style editor modals
// (Add/Edit/Duplicate Row, Create Table).
func (s *Session) Confirm() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Common.Confirm)
}

func (s *Session) RenameTable() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Schema.RenameTable)
}

// WriteToEditor pastes sql via clipboard. TextArea is always in insert mode — no mode switch needed.
func (s *Session) WriteToEditor(sql string) {
	s.t.Helper()
	if err := weztermSetClipboard(sql); err != nil {
		s.t.Fatalf("WriteToEditor: clipboard: %v", err)
	}
	s.sendAction(s.mustKeys().Common.Paste)
}

func (s *Session) RunQuery() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Common.Confirm)
}

// RunQueryInNewTab opens a new tab, writes sql, and executes it. Does not wait for results.
func (s *Session) RunQueryInNewTab(sql string) {
	s.t.Helper()
	s.sendAction(s.mustKeys().Main.NewTab)
	s.AssertPaneContains("Query")
	s.WriteToEditor(sql)
	s.RunQuery()
}

func (s *Session) WaitForQueryResult(timeout time.Duration) {
	s.t.Helper()
	s.WaitForPane("⏱", timeout)
}

func (s *Session) FocusSchemaTree() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Main.FocusSchemaTree)
}

func (s *Session) OpenActionsModal() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Main.OpenActions)
}

// GoToTable navigates to schema.table via the actions → go-to-table flow.
// Focus must be on the main page (not inside a modal).
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

func (s *Session) OpenExportModal() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Data.ExportData)
}

func (s *Session) OpenImportModal() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Main.ImportData)
}

func (s *Session) OpenStructure() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Schema.OpenStructure)
}

func (s *Session) OpenIndexes() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Schema.OpenIndexes)
}

func (s *Session) PeekRow() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Data.PeekRow)
}

func (s *Session) FollowForeignKey() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Data.FollowForeignKey)
}

func (s *Session) FindReferences() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Data.FindReferences)
}

func (s *Session) RenameTab() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Main.RenameTab)
}

func (s *Session) NextTab() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Navigation.FocusRight)
}

func (s *Session) PrevTab() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Navigation.FocusLeft)
}

func (s *Session) GoBottom() {
	s.t.Helper()
	s.sendAction(s.mustKeys().Navigation.GoBottom)
}

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

func (s *Session) LogFocus() {
	s.t.Helper()
	s.t.Logf("focus: %q", s.GetFocusedElement())
}

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

func projectRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..")
}

func DefaultJump() string {
	if v := os.Getenv(EnvJump); v != "" {
		return v
	}
	return defaultTable
}

func ParseJump(jump string) (schema, table string, ok bool) {
	parts := strings.SplitN(jump, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
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
