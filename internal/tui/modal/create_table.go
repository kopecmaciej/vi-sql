package modal

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

const CreateTableModalId = "CreateTable"

type columnDef struct {
	name     string
	dataType string
	pk       bool
	nullable bool
}

type CreateTableModal struct {
	*core.BaseElement
	*core.Flex

	tableNameInput *core.InputField
	columnsTable   *core.Table
	preview        *core.TextView
	schemaLabel    *core.TextView

	schema         string
	columns        []columnDef
	focusedRow     int
	focusedCol     int
	editing        bool
	editInput      *core.InputField
	dataTypes      []string
	applyCallback  func(ddl string) error
	cancelCallback func()
}

func NewCreateTableModal() *CreateTableModal {
	m := &CreateTableModal{
		BaseElement:    core.NewBaseElement(),
		Flex:           core.NewFlex(),
		tableNameInput: core.NewInputField(),
		columnsTable:   core.NewTable(),
		preview:        core.NewTextView(),
		schemaLabel:    core.NewTextView(),
		columns:        []columnDef{{name: "id", dataType: "SERIAL", pk: true, nullable: false}},
	}

	m.SetIdentifier(CreateTableModalId)
	m.tableNameInput.SetIdentifier(CreateTableModalId)
	m.columnsTable.SetIdentifier(CreateTableModalId)
	m.preview.SetIdentifier(CreateTableModalId)
	m.SetAfterInitFunc(m.init)

	return m
}

func (m *CreateTableModal) init() error {
	m.setStyle()
	m.setLayout()
	m.setKeybindings()
	m.handleEvents()

	if m.Driver != nil {
		m.dataTypes = m.Driver.CommonDataTypes()
	}

	return nil
}

func (m *CreateTableModal) setStyle() {
	styles := m.App.GetStyles()
	m.SetStyle(styles)
	m.tableNameInput.SetStyle(styles)
	m.tableNameInput.SetFieldBackgroundColor(styles.Global.ContrastBackgroundColor.Color())
	m.columnsTable.SetStyle(styles)
	m.preview.SetStyle(styles)
	m.schemaLabel.SetStyle(styles)
	m.schemaLabel.SetTextColor(styles.Global.SecondaryTextColor.Color())
}

func (m *CreateTableModal) setLayout() {
	m.Flex.SetDirection(tview.FlexColumn)
	m.Flex.SetBorder(true)

	m.SetTitle(" Create Table ")
	m.tableNameInput.SetLabel(" Table Name: ")
	m.tableNameInput.SetFieldWidth(40)

	m.columnsTable.SetBorder(true)
	m.columnsTable.SetTitle(" Columns ")
	m.columnsTable.SetBorderPadding(0, 0, 1, 1)
	m.columnsTable.SetSelectable(true, true)
	m.columnsTable.SetFixed(1, 0)

	m.preview.SetBorder(true)
	m.preview.SetTitle(" SQL Preview ")
	m.preview.SetBorderPadding(0, 0, 1, 1)
	m.preview.SetDynamicColors(true)
}

// focusableItems are: 0=tableNameInput, 1=columnsTable, 2=preview
// We track which one is focused via focusIndex
type focusTarget int

const (
	focusTableName focusTarget = iota
	focusColumns
	focusPreview
)

func (m *CreateTableModal) focusTarget(t focusTarget) {
	switch t {
	case focusTableName:
		m.App.SetFocusOnly(m.tableNameInput)
	case focusColumns:
		m.App.SetFocusOnly(m.columnsTable)
	case focusPreview:
		m.App.SetFocusOnly(m.preview)
	}
}

func (m *CreateTableModal) setKeybindings() {
	k := m.App.GetKeys()

	m.tableNameInput.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Match(k.Navigation.FocusDown, event):
			m.focusTarget(focusColumns)
			return nil
		case k.Match(k.Common.Close, event):
			m.handleCancel()
			return nil
		}
		return event
	}))

	m.columnsTable.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if m.editing {
			return event
		}
		switch {
		case k.Match(k.Navigation.FocusUp, event):
			m.focusTarget(focusTableName)
			return nil
		case k.Match(k.Navigation.FocusDown, event):
			m.focusTarget(focusPreview)
			return nil
		case k.Match(k.Common.Add, event):
			m.addColumn()
			return nil
		case k.Match(k.Common.Delete, event):
			m.deleteColumn()
			return nil
		case k.Match(k.Common.Confirm, event):
			m.handleExecute()
			return nil
		case k.Match(k.Common.Copy, event):
			util.Copy(m.buildDDL())
			return nil
		case k.Match(k.Common.Close, event):
			m.handleCancel()
			return nil
		}

		switch event.Key() {
		case tcell.KeyEnter:
			m.startEditing()
			return nil
		}

		return event
	}))

	m.preview.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Match(k.Navigation.FocusUp, event):
			m.focusTarget(focusColumns)
			return nil
		case k.Match(k.Common.Copy, event):
			util.Copy(m.buildDDL())
			return nil
		case k.Match(k.Common.Confirm, event):
			m.handleExecute()
			return nil
		case k.Match(k.Common.Close, event):
			m.handleCancel()
			return nil
		}
		return event
	}))

	m.columnsTable.SetSelectionChangedFunc(func(row, col int) {
		if row >= 1 {
			m.focusedRow = row - 1
			m.focusedCol = col
		}
	})
}

func (m *CreateTableModal) handleEvents() {
	go m.HandleEvents(CreateTableModalId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			m.setStyle()
			m.App.QueueUpdateDraw(func() {
				m.renderColumns()
				m.updatePreview()
				if !m.editing {
					m.rebuildLayout(nil)
				}
			})
		}
	})
}

func (m *CreateTableModal) startEditing() {
	row, col := m.columnsTable.GetSelection()
	if row < 1 || row-1 >= len(m.columns) {
		return
	}
	colIdx := row - 1

	switch col {
	case 0: // name
		m.editCell(colIdx, col, m.columns[colIdx].name)
	case 1: // type
		m.editCell(colIdx, col, m.columns[colIdx].dataType)
	case 2: // PK toggle
		m.columns[colIdx].pk = !m.columns[colIdx].pk
		m.renderColumns()
		m.updatePreview()
	case 3: // Nullable toggle
		m.columns[colIdx].nullable = !m.columns[colIdx].nullable
		m.renderColumns()
		m.updatePreview()
	}
}

func (m *CreateTableModal) editCell(colIdx, tableCol int, currentValue string) {
	m.editing = true

	input := core.NewInputField()
	input.SetText(currentValue)
	input.SetFieldWidth(0)
	input.SetBorder(true)

	if tableCol == 1 {
		input.SetAutocompleteFunc(m.autocompleteTypes)
		// SetAutocompletedFunc handles Enter on the autocomplete list; SetDoneFunc handles
		// Enter/Escape when autocomplete is closed. This order is required because
		// SetInputCapture fires before InputHandler, so intercepting Enter there would
		// call finishEditing with the typed prefix instead of the highlighted item.
		input.SetAutocompletedFunc(func(text string, index int, source int) bool {
			if source == tview.AutocompletedEnter {
				m.finishEditing(colIdx, tableCol, text)
				return true
			}
			return false
		})
		input.SetDoneFunc(func(key tcell.Key) {
			switch key {
			case tcell.KeyEnter:
				m.finishEditing(colIdx, tableCol, input.GetText())
			case tcell.KeyEscape:
				m.cancelEditing()
			}
		})
		input.SetInputCapture(core.DropdownInputCapture(m.App.GetKeys(), nil))
	} else {
		input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			switch event.Key() {
			case tcell.KeyEnter:
				m.finishEditing(colIdx, tableCol, input.GetText())
				return nil
			case tcell.KeyEscape:
				m.cancelEditing()
				return nil
			}
			return event
		})
	}

	m.editInput = input
	m.columnsTable.SetCell(colIdx+1, tableCol, tview.NewTableCell("").
		SetReference(input))

	m.renderWithEditInput()
}

func (m *CreateTableModal) renderWithEditInput() {
	editFlex := core.NewFlex().SetDirection(tview.FlexRow)
	editFlex.AddItem(m.columnsTable, 0, 1, false)
	editFlex.AddItem(m.editInput, 3, 0, true)

	m.rebuildLayout(editFlex)
	m.App.SetFocusOnly(m.editInput)
}

func (m *CreateTableModal) finishEditing(colIdx, tableCol int, value string) {
	switch tableCol {
	case 0:
		m.columns[colIdx].name = value
	case 1:
		m.columns[colIdx].dataType = strings.ToUpper(value)
	}
	m.editing = false
	m.editInput = nil
	m.renderColumns()
	m.updatePreview()
	m.rebuildLayout(nil)
	m.App.SetFocusOnly(m.columnsTable)
	m.columnsTable.Select(colIdx+1, tableCol)
}

func (m *CreateTableModal) cancelEditing() {
	m.editing = false
	m.editInput = nil
	m.renderColumns()
	m.rebuildLayout(nil)
	m.App.SetFocusOnly(m.columnsTable)
}

func (m *CreateTableModal) autocompleteTypes(currentText string) []tview.AutocompleteItem {
	if len(m.dataTypes) == 0 {
		return nil
	}
	var entries []tview.AutocompleteItem
	for _, dt := range m.dataTypes {
		if matched, _ := regexp.MatchString("(?i)^"+regexp.QuoteMeta(currentText), dt); matched {
			entries = append(entries, tview.AutocompleteItem{Main: dt})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Main < entries[j].Main
	})
	return entries
}

func (m *CreateTableModal) addColumn() {
	m.columns = append(m.columns, columnDef{name: "", dataType: "TEXT", nullable: true})
	m.renderColumns()
	m.updatePreview()
	// Focus the new row's name cell
	m.columnsTable.Select(len(m.columns), 0)
	m.startEditing()
}

func (m *CreateTableModal) deleteColumn() {
	if len(m.columns) <= 1 {
		return
	}
	row, _ := m.columnsTable.GetSelection()
	idx := row - 1
	if idx < 0 || idx >= len(m.columns) {
		return
	}
	m.columns = append(m.columns[:idx], m.columns[idx+1:]...)
	m.renderColumns()
	m.updatePreview()
	if row > len(m.columns) {
		row = len(m.columns)
	}
	m.columnsTable.Select(row, 0)
}

func (m *CreateTableModal) renderColumns() {
	styles := m.App.GetStyles()
	m.columnsTable.Clear()

	headers := []string{"NAME", "DATA TYPE", "PK", "NULL"}
	for i, h := range headers {
		m.columnsTable.SetCell(0, i, tview.NewTableCell(" "+h+" ").
			SetSelectable(false).
			SetTextColor(styles.Global.SecondaryTextColor.Color()).
			SetBackgroundColor(styles.Global.ContrastBackgroundColor.Color()).
			SetAlign(tview.AlignCenter))
	}

	checkmark := "✓"
	empty := " "

	for r, col := range m.columns {
		m.columnsTable.SetCell(r+1, 0, tview.NewTableCell(" "+col.name+" ").
			SetTextColor(styles.Global.TextColor.Color()))

		m.columnsTable.SetCell(r+1, 1, tview.NewTableCell(" "+col.dataType+" ").
			SetTextColor(styles.Global.MoreContrastBackgroundColor.Color()))

		pkText := empty
		if col.pk {
			pkText = checkmark
		}
		m.columnsTable.SetCell(r+1, 2, tview.NewTableCell(pkText).
			SetAlign(tview.AlignCenter).
			SetTextColor(styles.Global.MoreContrastBackgroundColor.Color()))

		nullText := empty
		if col.nullable {
			nullText = checkmark
		}
		m.columnsTable.SetCell(r+1, 3, tview.NewTableCell(nullText).
			SetAlign(tview.AlignCenter).
			SetTextColor(styles.Global.MoreContrastBackgroundColor.Color()))
	}

	if len(m.columns) > 0 {
		sel := m.focusedRow + 1
		if sel > len(m.columns) {
			sel = len(m.columns)
		}
		m.columnsTable.Select(sel, m.focusedCol)
	}
}

func (m *CreateTableModal) updatePreview() {
	tableName := m.tableNameInput.GetText()
	if tableName == "" {
		tableName = "<table_name>"
	}

	var qualifiedName string
	if m.schema != "" {
		qualifiedName = fmt.Sprintf("%s.%s", m.schema, tableName)
	} else {
		qualifiedName = tableName
	}

	var lines []string
	for _, col := range m.columns {
		parts := []string{fmt.Sprintf("  %s %s", col.name, col.dataType)}
		if col.pk {
			parts = append(parts, "PRIMARY KEY")
		}
		if !col.nullable && !col.pk {
			parts = append(parts, "NOT NULL")
		}
		lines = append(lines, strings.Join(parts, " "))
	}

	ddl := fmt.Sprintf("CREATE TABLE %s (\n%s\n);", qualifiedName, strings.Join(lines, ",\n"))
	s := m.App.GetStyles()
	m.preview.SetText(core.ColorizeSQLText(ddl, &s.SQLEditor))
}

func (m *CreateTableModal) buildDDL() string {
	tableName := m.tableNameInput.GetText()
	if tableName == "" {
		return ""
	}

	var qualifiedName string
	if m.schema != "" {
		qualifiedName = fmt.Sprintf("%s.%s", m.schema, tableName)
	} else {
		qualifiedName = tableName
	}

	var lines []string
	for _, col := range m.columns {
		parts := []string{fmt.Sprintf("  %s %s", col.name, col.dataType)}
		if col.pk {
			parts = append(parts, "PRIMARY KEY")
		}
		if !col.nullable && !col.pk {
			parts = append(parts, "NOT NULL")
		}
		lines = append(lines, strings.Join(parts, " "))
	}

	return fmt.Sprintf("CREATE TABLE %s (\n%s\n)", qualifiedName, strings.Join(lines, ",\n"))
}

func (m *CreateTableModal) handleExecute() {
	if m.applyCallback == nil {
		return
	}
	ddl := m.buildDDL()
	if ddl == "" {
		ShowError(m.App.Pages, "Validation error", fmt.Errorf("table name is required"))
		return
	}
	if err := m.applyCallback(ddl); err != nil {
		ShowError(m.App.Pages, "Error creating table", err)
	}
}

func (m *CreateTableModal) handleCancel() {
	if m.cancelCallback != nil {
		m.cancelCallback()
	}
}

func (m *CreateTableModal) rebuildLayout(editSection tview.Primitive) {
	m.Flex.Clear()
	m.Flex.SetBorderPadding(0, 0, 2, 2)

	styles := m.App.GetStyles()
	bg := styles.Global.BackgroundColor.Color()

	titleBar := core.NewFlex()
	titleBar.SetBackgroundColor(bg)

	m.schemaLabel.SetText(fmt.Sprintf("SCHEMA: %s ", strings.ToUpper(m.schema)))
	m.schemaLabel.SetTextAlign(tview.AlignRight)

	titleBar.AddItem(m.tableNameInput, 0, 1, false)
	titleBar.AddItem(m.schemaLabel, 0, 1, false)

	spacer := tview.NewBox()
	spacer.SetBackgroundColor(bg)

	rows := core.NewFlex().SetDirection(tview.FlexRow)
	rows.SetBackgroundColor(bg)
	rows.SetBorderPadding(1, 0, 0, 0)
	rows.AddItem(titleBar, 1, 0, false)
	rows.AddItem(spacer, 1, 0, false)
	if editSection != nil {
		rows.AddItem(editSection, 0, 3, true)
	} else {
		rows.AddItem(m.columnsTable, 0, 3, false)
	}
	rows.AddItem(m.preview, 0, 2, false)

	m.Flex.AddItem(rows, 0, 1, true)
}

// SetSchema sets the schema name for the new table.
func (m *CreateTableModal) SetSchema(schema string) {
	m.schema = schema
	if m.Driver != nil {
		m.dataTypes = m.Driver.CommonDataTypes()
	}
}

// SetApplyCallback sets the function called when the user executes.
func (m *CreateTableModal) SetApplyCallback(cb func(ddl string) error) {
	m.applyCallback = cb
}

// SetCancelCallback sets the function called when the user cancels.
func (m *CreateTableModal) SetCancelCallback(cb func()) {
	m.cancelCallback = cb
}

// GetTableName returns the current table name input value.
func (m *CreateTableModal) GetTableName() string {
	return m.tableNameInput.GetText()
}

// Render builds the modal layout and shows it as a page overlay.
// The defaultDDL parameter is ignored; we build DDL from the column definitions.
func (m *CreateTableModal) Render(defaultDDL string) {
	m.columns = []columnDef{{name: "id", dataType: "SERIAL", pk: true, nullable: false}}
	m.tableNameInput.SetText("")
	m.focusedRow = 0
	m.focusedCol = 0
	m.editing = false

	m.renderColumns()
	m.updatePreview()
	m.rebuildLayout(nil)

	// Center the modal
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(m.Flex, 0, 4, true).
			AddItem(nil, 0, 1, false), 0, 4, true).
		AddItem(nil, 0, 1, false)

	m.App.Pages.AddPage(CreateTableModalId, modal, true, true)
	m.App.SetFocusOnly(m.tableNameInput)

	// Update preview when table name changes
	m.tableNameInput.SetChangedFunc(func(text string) {
		m.updatePreview()
	})
}

// Hide removes the modal from the page stack.
func (m *CreateTableModal) Hide() {
	m.App.Pages.RemovePage(CreateTableModalId)
}
