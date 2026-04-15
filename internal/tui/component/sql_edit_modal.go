package component

import (
	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

const SQLEditModalId = "SQLEditModal"

// SQLEditModal is a full-screen overlay containing a SQLQueryEditor.
// It is used when UseBuiltin=true for add/edit/duplicate row operations.
// The caller sets a per-open onExecute callback; the modal closes after execution.
type SQLEditModal struct {
	*core.BaseElement
	*core.Flex

	editor    *SQLQueryEditor
	onExecute func(sql string)
}

func NewSQLEditModal() *SQLEditModal {
	m := &SQLEditModal{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		editor:      NewSQLQueryEditor(),
	}
	m.SetIdentifier(SQLEditModalId)
	m.SetAfterInitFunc(m.init)
	return m
}

func (m *SQLEditModal) init() error {
	if err := m.editor.Init(m.App); err != nil {
		return err
	}
	m.setLayout()
	m.setStyle()
	m.handleEvents()
	return nil
}

func (m *SQLEditModal) setLayout() {
	m.Flex.SetDirection(tview.FlexRow)
	m.editor.SetTitle(" SQL Editor — Execute to apply · Esc to cancel ")
	m.Flex.AddItem(m.editor, 0, 1, true)
}

func (m *SQLEditModal) setStyle() {
	m.Flex.SetStyle(m.App.GetStyles())
}

func (m *SQLEditModal) handleEvents() {
	go m.HandleEvents(SQLEditModalId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			m.setStyle()
		}
	})
}

// SetSchemas wires schema/table autocomplete into the editor.
func (m *SQLEditModal) SetSchemas(schemas []database.SchemaWithTables) {
	m.editor.SetSchemas(schemas)
}

// Open pre-fills the editor with initialSQL, wires the onExecute callback,
// and pushes the modal onto app.Pages. The modal closes itself after execution
// or when Esc is pressed.
func (m *SQLEditModal) Open(initialSQL string, onExecute func(sql string)) {
	m.onExecute = onExecute

	m.editor.SetOnExecute(func(sql string) {
		m.close()
		if m.onExecute != nil {
			m.onExecute(sql)
		}
	})

	// Esc cancels and closes before the editor's own handler runs.
	m.editor.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			m.close()
			return nil
		}
		return event
	})

	m.editor.SetText(initialSQL, true)
	m.App.Pages.AddPage(SQLEditModalId, m, true, true)
	m.App.SetFocusInternal(m.editor)
}

func (m *SQLEditModal) close() {
	m.App.Pages.RemovePage(SQLEditModalId)
}
