//go:build wezterm

package wezterm_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestImportCSV(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "name text, email text")

	csvPath := filepath.Join(t.TempDir(), "import.csv")
	csvContent := "name,email\nAlice,alice@test.com\nBob,bob@test.com\n"
	if err := os.WriteFile(csvPath, []byte(csvContent), 0600); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	s := harness.SpawnConnected(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPane("⏱")

	s.OpenImportModal()
	s.WaitForPane(" Import CSV ")

	s.ClearField()
	s.Paste(fmt.Sprintf("%s.%s", schema, table))
	s.FocusDown(1)
	s.ClearField()
	s.Paste(csvPath)
	s.FocusDown(3)
	s.Select()

	harness.WaitForRowCount(t, db, schema, table, 2)
	s.AssertPaneNotContains(" Import CSV ")
	s.AssertPaneNotContains(" No rows found ")
	s.Select()
}
