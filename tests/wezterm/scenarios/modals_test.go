//go:build wezterm

package wezterm_test

import (
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestModalActions(t *testing.T) {
	s := harness.SpawnConnected(t, "--debug")

	s.OpenActionsModal()
	s.AssertPaneContains(" Actions ")
	s.Close()
	s.AssertPaneNotContains(" Actions ")
}

func TestModalServerInfo(t *testing.T) {
	s := harness.SpawnConnected(t, "--debug")

	s.OpenServerInfo()
	s.AssertPaneContains(" Server Info ")
	s.Close()
	s.AssertPaneNotContains(" Server Info ")
}

func TestModalStyleChange(t *testing.T) {
	s := harness.SpawnConnected(t, "--debug")

	s.ChangeStyle()
	s.AssertPaneContains(" Change Style ")
	s.Close()
	s.AssertPaneNotContains(" Change Style ")
}

func TestModalExport(t *testing.T) {
	s, _ := harness.SpawnWithTable(t)

	s.OpenExportModal()
	s.AssertPaneContains(" Export ")
	s.Close()
	s.AssertPaneNotContains(" Export ")
}

func TestModalImport(t *testing.T) {
	s := harness.SpawnConnected(t, "--debug")

	s.OpenImportModal()
	s.AssertPaneContains(" Import CSV ")
	s.Close()
	s.AssertPaneNotContains(" Import CSV ")
}

func TestModalHistory(t *testing.T) {
	s := harness.SpawnConnected(t, "--debug")

	s.NewTab()
	s.AssertPaneContains("Query")

	s.OpenHistory()
	s.AssertPaneContains(" SQL History ")
	s.Close()
	s.AssertPaneNotContains(" SQL History ")
}

func TestModalGoToTable(t *testing.T) {
	s := harness.SpawnConnected(t, "--debug")

	s.OpenActionsModal()
	s.AssertPaneContains(" Actions ")
	s.Type("go to")
	s.Select()
	s.WaitForPane(" Go to table ", 5*time.Second)
	s.Close()
	s.AssertPaneNotContains(" Go to table ")
}

func TestModalEditRow(t *testing.T) {
	s, _ := harness.SpawnWithTable(t)

	s.MoveDown(1)
	s.EditRow()
	if s.IsVimMode() {
		s.AssertPaneContains(" Edit Row Normal ")
	} else {
		s.AssertPaneContains(" Edit Row ")
		s.AssertPaneNotContains(" Edit Row Normal ")
	}
	s.Close()

	if s.IsVimMode() {
		s.AssertPaneNotContains(" Edit Row Normal ")
	} else {
		s.AssertPaneNotContains(" Edit Row ")
	}
}

func TestModalErrorOnBadQuery(t *testing.T) {
	s := harness.SpawnConnected(t, "--debug")

	s.RunQueryInNewTab("THIS IS NOT VALID SQL")

	s.WaitForPane(" Error ", 5*time.Second)
	s.Close()
	s.AssertPaneNotContains(" Error ")
}
