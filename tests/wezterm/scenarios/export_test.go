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
	s.WaitForPane(" Export ")
	s.WaitForFocus("ExportModal", 2*time.Second)

	s.Close()

	s.AssertPaneNotContains(" Export ")
}

func TestExportCSV(t *testing.T) {
	s, _ := harness.SpawnWithTable(t)

	tmp := t.TempDir()

	s.OpenExportModal()
	s.WaitForPane(" Export ")
	s.WaitForFocus("ExportModal", 2*time.Second)

	s.FocusDown(1) // → Filename
	s.ClearField()
	s.Paste("export.csv")
	s.FocusDown(1) // → Path
	s.ClearField()
	s.Paste(tmp + "/")
	s.FocusDown(3) // -> till buttons
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
	s.WaitForPane(" Export ")
	s.WaitForFocus("ExportModal", 2*time.Second)

	s.MoveDown(1)  // Format: CSV → JSON
	s.FocusDown(1) // → Filename
	s.ClearField()
	s.Paste("export.json")
	s.FocusDown(1) // → Path
	s.ClearField()
	s.Paste(tmp + "/")
	s.FocusDown(3) // → Pretty Print
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
