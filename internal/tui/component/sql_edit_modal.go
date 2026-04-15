package component

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

const SQLEditModalId = "SQLEditModal"

// SQLEditModal is a centered overlay (≈80% of screen) containing a
// SQLQueryEditor with line numbers. It is used for add/edit/duplicate row
// operations when UseBuiltin=true. The title reflects the current operation.
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
	styles := m.App.GetStyles()

	// Outer container provides the border and title.
	m.Flex.SetDirection(tview.FlexRow)
	m.Flex.SetBorder(true)
	core.SetCommonStyle(m.Flex, styles)

	// Editor fills the space; remove its own border to avoid double borders.
	m.editor.TextArea.SetBorder(false)
	m.editor.TextArea.SetBorderPadding(0, 0, 1, 1)
	m.editor.TextArea.SetLineNumbers(true)

	m.Flex.AddItem(m.editor, 0, 1, true)
}

func (m *SQLEditModal) setStyle() {
	styles := m.App.GetStyles()
	core.SetCommonStyle(m.Flex, styles)
	m.setLineNumberStyle(styles)
}

func (m *SQLEditModal) setLineNumberStyle(styles *config.Styles) {
	lineNumStyle := tcell.StyleDefault.
		Foreground(styles.Global.BorderColor.Color()).
		Background(styles.Global.BackgroundColor.Color())
	m.editor.TextArea.SetLineNumberStyle(lineNumStyle)
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

// Open pre-fills the editor with initialSQL, sets the operation title,
// wires the onExecute callback, and pushes the modal onto app.Pages.
// The modal closes itself after execution or when Esc is pressed.
func (m *SQLEditModal) Open(title, initialSQL string, onExecute func(sql string)) {
	m.onExecute = onExecute

	m.Flex.SetTitle(fmt.Sprintf(" %s ", title))

	m.editor.SetOnExecute(func(sql string) {
		m.close()
		if m.onExecute != nil {
			m.onExecute(sql)
		}
	})

	m.editor.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			m.close()
			return nil
		}
		return event
	})

	m.editor.SetText(initialSQL, true)
	// false = do not resize to fill screen; Draw() controls the rect.
	m.App.Pages.AddPage(SQLEditModalId, m, false, true)
	m.App.SetFocusInternal(m.editor)
}

func (m *SQLEditModal) close() {
	m.App.Pages.RemovePage(SQLEditModalId)
}

// Draw positions the modal at ~80% of the screen, centered.
func (m *SQLEditModal) Draw(screen tcell.Screen) {
	sw, sh := screen.Size()
	w := sw * 4 / 5
	h := sh * 4 / 5
	if w < 40 {
		w = sw
	}
	if h < 10 {
		h = sh
	}
	x := (sw - w) / 2
	y := (sh - h) / 2
	m.Flex.SetRect(x, y, w, h)
	m.Flex.Draw(screen)
}

// Focus delegates focus to the inner editor.
func (m *SQLEditModal) Focus(delegate func(p tview.Primitive)) {
	delegate(m.editor)
}

// HasFocus returns true when the editor has focus.
func (m *SQLEditModal) HasFocus() bool {
	return m.editor.HasFocus()
}
