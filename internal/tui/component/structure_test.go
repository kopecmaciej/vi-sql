package component

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStructureTestApp(t *testing.T, vim bool) *Structure {
	t.Helper()
	app, _ := testutil.NewTestApp(t)
	app.GetConfig().UI.VimMode = vim
	require.NoError(t, app.ReloadKeybindings())

	s := NewStructure()
	require.NoError(t, s.Init(app))
	return s
}

func pressKey(t *testing.T, p tview.Primitive, r rune) {
	t.Helper()
	handler := p.InputHandler()
	require.NotNil(t, handler, "primitive must have an input handler")
	handler(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone), func(tview.Primitive) {})
}

func TestStructure_ToggleDDLOpensAndFocusesDDLPane(t *testing.T) {
	for _, vim := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "vim"}[vim], func(t *testing.T) {
			s := newStructureTestApp(t, vim)
			s.showDDL = false // start hidden
			s.App.SetFocus(s.table)

			pressKey(t, s.table, 'p')

			assert.True(t, s.showDDL, "p must open the DDL pane")
			assert.Same(t, s.ddlView, s.App.GetFocus(),
				"opening the DDL pane must move focus to it")
		})
	}
}

func TestStructure_ToggleDDLFromDDLClosesAndReturnsFocus(t *testing.T) {
	for _, vim := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "vim"}[vim], func(t *testing.T) {
			s := newStructureTestApp(t, vim)
			s.App.SetFocus(s.ddlView)

			pressKey(t, s.ddlView, 'p')

			assert.False(t, s.showDDL, "p from the DDL pane must close it")
			assert.Same(t, s.table, s.App.GetFocus(),
				"closing the DDL pane must return focus to the structure table")
		})
	}
}

func TestStructure_ToggleDDLFromTableKeepsFocus(t *testing.T) {
	for _, vim := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "vim"}[vim], func(t *testing.T) {
			s := newStructureTestApp(t, vim)
			s.App.SetFocus(s.table)

			pressKey(t, s.table, 'p')

			assert.False(t, s.showDDL, "p from the table must close the DDL pane")
			assert.Same(t, s.table, s.App.GetFocus(),
				"focus must stay on the table when closing from it")
		})
	}
}
