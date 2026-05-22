//go:build wezterm

package wezterm_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestExportModal(t *testing.T) {
	s, _ := harness.SpawnWithTable(t)

	s.OpenExportModal()
	s.WaitForPane(" Export ", 2*time.Second)
	s.WaitForFocus("ExportModal", 2*time.Second)

	s.Close()

	s.AssertPaneNotContains(" Export ")
}

func TestExportCSV(t *testing.T) {
	s, _ := harness.SpawnWithTable(t)

	tmp := t.TempDir()

	s.OpenExportModal()
	s.WaitForPane(" Export ", 2*time.Second)
	s.WaitForFocus("ExportModal", 2*time.Second)

	s.FormNext() // → Filename
	s.ClearField()
	s.Paste("export.csv")
	s.FormNext() // → Path
	s.ClearField()
	s.Paste(tmp + "/")
	s.FormNext() // → Include Headers
	s.FormNext() // → Compress
	s.FormNext() // → Export button
	s.Select()

	s.AssertPaneNotContains(" Export ")
	s.WaitForFile(tmp+"/export.csv", 3*time.Second)

	data, err := os.ReadFile(tmp + "/export.csv")
	if err != nil {
		t.Fatalf("cannot read export file: %v", err)
	}
	if len(data) == 0 {
		t.Error("export file is empty")
	}
}

func TestExportJSON(t *testing.T) {
	s, _ := harness.SpawnWithTable(t)

	tmp := t.TempDir()

	s.OpenExportModal()
	s.WaitForPane(" Export ", 5*time.Second)
	s.WaitForFocus("ExportModal", 2*time.Second)

	s.MoveDown(1) // Format: CSV → JSON
	s.FormNext()  // → Filename
	s.ClearField()
	s.Paste("export.json")
	s.FormNext() // → Path
	s.ClearField()
	s.Paste(tmp + "/")
	s.FormNext() // → Pretty Print
	s.FormNext() // → Compress
	s.FormNext() // → Export button
	s.Select()

	s.AssertPaneNotContains(" Export ")
	s.WaitForFile(tmp+"/export.json", 3*time.Second)

	data, err := os.ReadFile(tmp + "/export.json")
	if err != nil {
		t.Fatalf("cannot read JSON export file: %v", err)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(data)), "[") {
		t.Errorf("JSON export: expected file to start with '[', got: %.50s", string(data))
	}
}
