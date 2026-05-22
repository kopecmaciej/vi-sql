//go:build wezterm

package wezterm_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

// TestExportCSV opens a table via --jump, opens the export modal, sets a temp
// output path, exports as CSV, and verifies the file was created.
func TestExportModal(t *testing.T) {
	s, _ := spawnWithTable(t)
	if s == nil {
		return
	}

	s.OpenExportModal()
	s.WaitForPane(" Export ", 2*time.Second)

	s.Dismiss()

	s.AssertPaneNotContains(" Export ")
}

// Prerequisites:
//   - VI_SQL_WEZTERM_CONNECTION and VI_SQL_WEZTERM_JUMP must be set.
func TestExportCSV(t *testing.T) {
	s, _ := spawnWithTable(t)
	if s == nil {
		return
	}

	tmp := t.TempDir()

	s.OpenExportModal()
	s.WaitForPane(" Export ", 2*time.Second)

	// Form order: Format (0) → Filename (1) → Path (2) → Include Headers (3) →
	// Compress (4) → [Export button].
	s.FormNext() // Format → Filename
	s.ClearField()
	s.Paste("export.csv")
	s.FormNext() // Filename → Path
	s.ClearField()
	s.Paste(tmp + "/")
	s.FormNext() // Path → Include Headers checkbox
	s.FormNext() // Include Headers → Compress checkbox
	s.FormNext() // Compress → Export button
	s.Select()

	s.AssertPaneNotContains(" Export ")

	outFile := tmp + "/export.csv"
	if _, err := os.Stat(outFile); err != nil {
		t.Fatalf("export file %q not created: %v", outFile, err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("cannot read export file: %v", err)
	}
	if len(data) == 0 {
		t.Error("export file is empty")
	}
}

// TestExportJSON opens the export modal, picks JSON format, and verifies the
// exported file starts with a JSON array.
//
// Prerequisites:
//   - VI_SQL_WEZTERM_CONNECTION and VI_SQL_WEZTERM_JUMP must be set.
func TestExportJSON(t *testing.T) {
	s, _ := spawnWithTable(t)
	if s == nil {
		return
	}

	tmp := t.TempDir()

	s.OpenExportModal()
	s.WaitForPane(" Export ", 5*time.Second)

	// Navigate format dropdown from CSV (0) to JSON (1).
	s.MoveDown(1)
	// Form order for JSON: Format → Filename → Path → Pretty Print → Compress → [Export button].
	s.FormNext() // Format → Filename
	s.ClearField()
	s.Paste("export.json")
	s.FormNext() // Filename → Path
	s.ClearField()
	s.Paste(tmp + "/")
	s.FormNext() // Path → Pretty Print checkbox
	s.FormNext() // Pretty Print → Compress checkbox
	s.FormNext() // Compress → Export button
	s.Select()

	s.AssertPaneNotContains(" Export ")

	outFile := tmp + "/export.json"
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("cannot read JSON export file: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(data)), "[") {
		t.Errorf("JSON export: expected file to start with '[', got: %.50s", string(data))
	}
}

// spawnWithTable starts vi-sql with --jump pre-opening a table, waits for the
// data grid, and returns the session and table name. Returns nil if no
// connection or valid jump target is available.
func spawnWithTable(t *testing.T) (*harness.Session, string) {
	t.Helper()
	conn := harness.DefaultConnection()
	if conn == "" {
		t.Skip("no connection available — skipping scenario")
		return nil, ""
	}
	jump := harness.DefaultJump()
	_, table, ok := harness.ParseJump(jump)
	if !ok {
		t.Skipf("jump target %q is not in schema.table format", jump)
		return nil, ""
	}

	s := harness.Spawn(t, "--connection-name", conn, "--jump", jump, "--debug")
	s.WaitForPane("⏱", 10*time.Second)
	return s, table
}
