//go:build wezterm

package wezterm_test

import (
	"testing"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestGoToTableModal(t *testing.T) {
	jump := harness.DefaultJump()
	schema, table, ok := harness.ParseJump(jump)
	if !ok {
		t.Skipf("jump target %q is not in schema.table format", jump)
	}

	s := harness.SpawnConnected(t, "--debug")

	s.GoToTable(schema, table)
	s.WaitForPane(table)
}
