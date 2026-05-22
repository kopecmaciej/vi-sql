//go:build wezterm

package wezterm_test

import (
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

// TestGoToTableModal opens the go-to-table modal via the actions modal, types a
// fully-qualified table name, confirms, and verifies the data grid loads.
func TestGoToTableModal(t *testing.T) {
	conn := harness.DefaultConnection()
	if conn == "" {
		t.Skip("no connection available — skipping go-to-table scenario")
	}
	jump := harness.DefaultJump()
	schema, table, ok := harness.ParseJump(jump)
	if !ok {
		t.Skipf("jump target %q is not in schema.table format", jump)
	}

	s := harness.Spawn(t, "--connection-name", conn, "--debug")

	s.GoToTable(schema, table)
	s.WaitForPane(table, 5*time.Second)
}
