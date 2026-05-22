//go:build wezterm

// Package wezterm_test modal smoke tests: open each modal, assert its title
// appears, close it with Esc, assert it's gone.
package wezterm_test

import (
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestModalActions(t *testing.T) {
	conn := harness.DefaultConnection()
	if conn == "" {
		t.Skip("no connection available — skipping modal smoke")
	}

	s := harness.Spawn(t, "--connection-name", conn, "--debug")

	s.OpenActionsModal()
	s.AssertPaneContains(" Actions ")
	s.Close()
	s.AssertPaneNotContains(" Actions ")
}

func TestModalServerInfo(t *testing.T) {
	conn := harness.DefaultConnection()
	if conn == "" {
		t.Skip("no connection available — skipping modal smoke")
	}

	s := harness.Spawn(t, "--connection-name", conn, "--debug")

	s.OpenServerInfo()
	s.AssertPaneContains(" Server Info ")
	s.Close()
	s.AssertPaneNotContains(" Server Info ")
}

func TestModalStyleChange(t *testing.T) {
	conn := harness.DefaultConnection()
	if conn == "" {
		t.Skip("no connection available — skipping modal smoke")
	}

	s := harness.Spawn(t, "--connection-name", conn, "--debug")

	s.ChangeStyle()
	s.AssertPaneContains(" Change Style ")
	s.Close()
	s.AssertPaneNotContains(" Change Style ")
}

func TestModalExport(t *testing.T) {
	s, _ := harness.SpawnWithTable(t)
	if s == nil {
		return
	}

	s.OpenExportModal()
	s.AssertPaneContains(" Export ")
	s.Close()
	s.AssertPaneNotContains(" Export ")
}

func TestModalImport(t *testing.T) {
	conn := harness.DefaultConnection()
	if conn == "" {
		t.Skip("no connection available — skipping modal smoke")
	}

	s := harness.Spawn(t, "--connection-name", conn, "--debug")

	s.OpenImportModal()
	s.AssertPaneContains(" Import CSV ")
	s.Close()
	s.AssertPaneNotContains(" Import CSV ")
}

func TestModalHistory(t *testing.T) {
	conn := harness.DefaultConnection()
	if conn == "" {
		t.Skip("no connection available — skipping modal smoke")
	}

	s := harness.Spawn(t, "--connection-name", conn, "--debug")

	s.NewTab()
	s.AssertPaneContains("Query")

	s.OpenHistory()
	s.AssertPaneContains(" SQL History ")
	s.Close()
	s.AssertPaneNotContains(" SQL History ")
}

func TestModalGoToTable(t *testing.T) {
	conn := harness.DefaultConnection()
	if conn == "" {
		t.Skip("no connection available — skipping modal smoke")
	}

	s := harness.Spawn(t, "--connection-name", conn, "--debug")

	s.OpenActionsModal()
	s.AssertPaneContains(" Actions ")
	s.Type("go to")
	s.Select()
	s.WaitForPane(" Go to table ", 5*time.Second)
	s.Close()
	s.AssertPaneNotContains(" Go to table ")
}

func TestModalInlineEdit(t *testing.T) {
	s, _ := harness.SpawnWithTable(t)
	if s == nil {
		return
	}

	s.MoveDown(1)
	s.EditRow()
	if s.IsVimMode() {
		s.AssertPaneContains(" Edit Normal ")
	} else {
		s.AssertPaneContains(" Edit ")
		s.AssertPaneNotContains(" Edit Normal ")
	}
	s.Close()

	if s.IsVimMode() {
		s.AssertPaneNotContains(" Edit Normal ")
	} else {
		s.AssertPaneNotContains(" Edit ")
	}
}

func TestModalErrorOnBadQuery(t *testing.T) {
	conn := harness.DefaultConnection()
	if conn == "" {
		t.Skip("no connection available — skipping modal smoke")
	}

	s := harness.Spawn(t, "--connection-name", conn, "--debug")

	s.RunQueryInNewTab("THIS IS NOT VALID SQL")

	s.WaitForPane(" Error ", 5*time.Second)
	s.Close()
	s.AssertPaneNotContains(" Error ")
}
