package modal

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

const CreateTableModalId = "CreateTable"

// ColumnDef represents a single column definition in the table creator.
type ColumnDef struct {
	Name     string
	DataType string
	IsPK     bool
	IsNull   bool
}

// focusArea tracks which section of the modal has focus.
type focusArea int

const (
	focusTableName focusArea = iota
	focusColumns
)

// CreateTableModal is a structured table creator modal.
// It shows a table name input, a column definitions table with
// name/type/pk/null fields, and a live SQL preview.
type CreateTableModal struct {
	*core.BaseElement
	*core.Flex

	tableNameInput *core.InputField
	columnsTable   *core.Table
	sqlPreview     *core.TextView
	editField      *core.InputField

	columns       []ColumnDef
	schema        string
	focus         focusArea
	selectedCol   int
	selectedField int // 0=name, 1=type, 2=pk, 3=null
	editing       bool

	applyCallback  func(ddl string) error
	cancelCallback func()
}

func NewCreateTableModal() *CreateTableModal {
	m := &CreateTableModal{
		BaseElement:    core.NewBaseElement(),
		Flex:           core.NewFlex(),
		tableNameInput: core.NewInputField(),
		columnsTable:   core.NewTable(),
		sqlPreview:     core.NewTextView(),
		editField:      core.NewInputField(),
		columns: []ColumnDef{
			{Name: "id", DataType: "SERIAL", IsPK: true, IsNull: false},
		},
	}
	m.SetIdentifier(CreateTableModalId)
	m.SetAfterInitFunc(m.init)
	return m
}

func (m *CreateTableModal) init() error {
	m.setLayout()
	m.setStyle()
	m.setKeybindings()
	m.handleEvents()
	return nil
}

func (m *CreateTableModal) setLayout() {
	m.Flex.SetBorder(true)
	m.Flex.SetTitle(" Create New Table ")
	m.Flex.SetTitleAlign(tview.AlignLeft)
	m.Flex.SetDirection(tview.FlexRow)
	m.Flex.SetBorderPadding(0, 0, 1, 1)

	m.tableNameInput.SetLabel(" Table Name: ")
	m.tableNameInput.SetFieldWidth(40)
	m.tableNameInput.SetChangedFunc(func(text string) {
		m.updatePreview()
	})

	m.columnsTable.SetSelectable(false, false)
	m.columnsTable.SetBorders(false)
	m.columnsTable.SetSeparator(' ')
	m.columnsTable.SetFixed(1, 0)

	m.sqlPreview.SetDynamicColors(true)
	m.sqlPreview.SetScrollable(true)
	m.sqlPreview.SetBorder(true)
	m.sqlPreview.SetTitle(" SQL Preview ")

	m.editField.SetFieldWidth(30)
	m.editField.SetInputCapture(core.DropdownInputCapture(m.App.GetKeys(), nil))
}

func (m *CreateTableModal) setStyle() {
	styles := m.App.GetStyles()

	bgColor := styles.Global.BackgroundColor.Color()
	borderColor := styles.Global.BorderColor.Color()
	textColor := styles.Global.TextColor.Color()
	contrastBg := styles.Global.ContrastBackgroundColor.Color()
	secondaryText := styles.Global.SecondaryTextColor.Color()
	accentColor := styles.Global.MoreContrastBackgroundColor.Color()

	m.Flex.SetBackgroundColor(bgColor)
	m.Flex.SetBorderColor(borderColor)
	m.Flex.SetTitleColor(styles.Global.TitleColor.Color())

	m.tableNameInput.SetBackgroundColor(bgColor)
	m.tableNameInput.SetFieldBackgroundColor(contrastBg)
	m.tableNameInput.SetFieldTextColor(textColor)
	m.tableNameInput.SetLabelColor(secondaryText)

	m.columnsTable.SetBackgroundColor(bgColor)
	m.columnsTable.SetSelectedStyle(tcell.StyleDefault.
		Foreground(textColor).
		Background(accentColor))

	m.sqlPreview.SetBackgroundColor(contrastBg)
	m.sqlPreview.SetTextColor(textColor)
	m.sqlPreview.SetBorderColor(borderColor)

	m.editField.SetBackgroundColor(bgColor)
	m.editField.SetFieldBackgroundColor(contrastBg)
	m.editField.SetFieldTextColor(textColor)
}

// rebuildFlex repopulates the Flex layout, adjusting the columns table height.
func (m *CreateTableModal) rebuildFlex() {
	m.Flex.Clear()

	tableH := len(m.columns) + 1 // header row + data rows
	if tableH < 2 {
		tableH = 2
	}
	if tableH > 8 {
		tableH = 8
	}

	m.Flex.AddItem(m.tableNameInput, 1, 0, false)
	m.Flex.AddItem(tview.NewBox(), 1, 0, false) // spacer
	m.Flex.AddItem(m.columnsTable, tableH, 0, false)
	m.Flex.AddItem(tview.NewBox(), 1, 0, false) // spacer
	m.Flex.AddItem(m.sqlPreview, 0, 1, false)
}

func (m *CreateTableModal) updateSelectable() {
	m.columnsTable.SetSelectable(m.focus == focusColumns, m.focus == focusColumns)
}

func (m *CreateTableModal) setKeybindings() {
	k := m.App.GetKeys()

	// Table name input: handle global and focus-switching keys; pass rest through.
	m.tableNameInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Contains(k.CreateTable.Cancel, event.Name()):
			if m.cancelCallback != nil {
				m.cancelCallback()
			}
			return nil
		case k.Contains(k.CreateTable.Execute, event.Name()):
			m.handleApply()
			return nil
		case k.Contains(k.Navigation.FocusDown, event.Name()),
			k.Contains(k.Navigation.FocusUp, event.Name()):
			m.focus = focusColumns
			m.updateSelectable()
			m.App.SetFocusInternal(m)
			return nil
		}
		return event
	})

	// Flex capture: active when columns section has focus.
	m.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Inline edit mode — forward events to the edit field.
		if m.editing {
			if event.Key() == tcell.KeyEscape {
				m.editing = false
				m.rebuildColumnsTable()
				return nil
			}
			if handler := m.editField.InputHandler(); handler != nil {
				handler(event, func(p tview.Primitive) {})
			}
			return nil
		}

		// Global keys
		switch {
		case k.Contains(k.CreateTable.Cancel, event.Name()):
			if m.cancelCallback != nil {
				m.cancelCallback()
			}
			return nil
		case k.Contains(k.CreateTable.Execute, event.Name()):
			m.handleApply()
			return nil
		case k.Contains(k.Navigation.FocusDown, event.Name()),
			k.Contains(k.Navigation.FocusUp, event.Name()):
			m.focus = focusTableName
			m.updateSelectable()
			m.App.SetFocusInternal(m)
			return nil
		}

		// Column table: action keys
		switch {
		case k.Contains(k.CreateTable.AddColumn, event.Name()):
			m.addColumn()
			return nil
		case k.Contains(k.CreateTable.DeleteColumn, event.Name()):
			m.deleteColumn()
			return nil
		}

		// Column table: navigation
		switch {
		case k.Contains(k.Navigation.MoveUp, event.Name()):
			if m.selectedCol > 0 {
				m.selectedCol--
				m.columnsTable.Select(m.selectedCol+1, m.selectedField)
			}
			return nil
		case k.Contains(k.Navigation.MoveDown, event.Name()):
			if m.selectedCol < len(m.columns)-1 {
				m.selectedCol++
				m.columnsTable.Select(m.selectedCol+1, m.selectedField)
			}
			return nil
		case k.Contains(k.Navigation.MoveLeft, event.Name()):
			if m.selectedField > 0 {
				m.selectedField--
				m.columnsTable.Select(m.selectedCol+1, m.selectedField)
			}
			return nil
		case k.Contains(k.Navigation.MoveRight, event.Name()):
			if m.selectedField < 3 {
				m.selectedField++
				m.columnsTable.Select(m.selectedCol+1, m.selectedField)
			}
			return nil
		}

		switch event.Key() {
		case tcell.KeyEnter:
			m.startEditing()
			return nil
		}

		if event.Rune() == ' ' && (m.selectedField == 2 || m.selectedField == 3) {
			m.startEditing()
			return nil
		}

		return event
	})
}

func (m *CreateTableModal) handleEvents() {
	go m.HandleEvents(m.GetIdentifier(), func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			m.setStyle()
		}
	})
}

// SetApplyCallback sets the function called when the user executes the DDL.
func (m *CreateTableModal) SetApplyCallback(callback func(ddl string) error) {
	m.applyCallback = callback
}

// SetCancelCallback sets the function called when the user cancels.
func (m *CreateTableModal) SetCancelCallback(callback func()) {
	m.cancelCallback = callback
}

func (m *CreateTableModal) handleApply() {
	ddl := m.generateDDL()
	if ddl == "" {
		return
	}
	if m.applyCallback != nil {
		if err := m.applyCallback(ddl); err != nil {
			ShowError(m.App.Pages, "Error creating table", err)
		}
	}
}

func (m *CreateTableModal) addColumn() {
	m.columns = append(m.columns, ColumnDef{
		Name:     "",
		DataType: "TEXT",
		IsPK:     false,
		IsNull:   true,
	})
	m.selectedCol = len(m.columns) - 1
	m.selectedField = 0
	m.focus = focusColumns
	m.updateSelectable()
	m.rebuildColumnsTable()
	m.updatePreview()
	m.startEditing()
}

func (m *CreateTableModal) deleteColumn() {
	if len(m.columns) <= 1 {
		return
	}
	m.columns = append(m.columns[:m.selectedCol], m.columns[m.selectedCol+1:]...)
	if m.selectedCol >= len(m.columns) {
		m.selectedCol = len(m.columns) - 1
	}
	m.rebuildColumnsTable()
	m.updatePreview()
}

func (m *CreateTableModal) startEditing() {
	if m.selectedCol < 0 || m.selectedCol >= len(m.columns) {
		return
	}

	col := m.columns[m.selectedCol]
	switch m.selectedField {
	case 0: // name
		m.editing = true
		m.editField.SetText(col.Name)
		m.editField.SetLabel("")
		m.editField.SetAutocompleteFunc(nil)
		m.editField.SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter {
				m.columns[m.selectedCol].Name = m.editField.GetText()
			}
			m.editing = false
			m.rebuildColumnsTable()
			m.updatePreview()
		})
	case 1: // data type — input with autocomplete
		m.editing = true
		m.editField.SetText(col.DataType)
		m.editField.SetLabel("")
		m.editField.SetAutocompleteFunc(func(currentText string) []tview.AutocompleteItem {
			var items []tview.AutocompleteItem
			for _, dt := range m.Driver.CommonDataTypes() {
				if currentText == "" || strings.HasPrefix(strings.ToUpper(dt), strings.ToUpper(currentText)) {
					items = append(items, tview.AutocompleteItem{Main: dt})
				}
			}
			return items
		})
		m.editField.SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter {
				m.columns[m.selectedCol].DataType = m.editField.GetText()
			}
			m.editing = false
			m.editField.SetAutocompleteFunc(nil)
			m.rebuildColumnsTable()
			m.updatePreview()
		})
	case 2: // PK toggle
		m.columns[m.selectedCol].IsPK = !m.columns[m.selectedCol].IsPK
		if m.columns[m.selectedCol].IsPK {
			m.columns[m.selectedCol].IsNull = false
		}
		m.rebuildColumnsTable()
		m.updatePreview()
	case 3: // NULL toggle
		if !m.columns[m.selectedCol].IsPK {
			m.columns[m.selectedCol].IsNull = !m.columns[m.selectedCol].IsNull
		}
		m.rebuildColumnsTable()
		m.updatePreview()
	}
}

func (m *CreateTableModal) rebuildColumnsTable() {
	m.columnsTable.Clear()

	styles := m.App.GetStyles()
	headerColor := styles.Global.SecondaryTextColor.Color()

	headers := []string{"  NAME", "DATA TYPE", "PK", "NULL"}
	for i, h := range headers {
		cell := tview.NewTableCell(h).
			SetTextColor(headerColor).
			SetSelectable(false)
		if i < 2 {
			cell.SetExpansion(1)
		} else {
			cell.SetMaxWidth(6)
		}
		m.columnsTable.SetCell(0, i, cell)
	}

	textColor := styles.Global.TextColor.Color()
	accentColor := styles.Global.MoreContrastBackgroundColor.Color()

	for i, col := range m.columns {
		row := i + 1

		nameCell := tview.NewTableCell("  " + col.Name).
			SetTextColor(textColor).
			SetExpansion(1)
		m.columnsTable.SetCell(row, 0, nameCell)

		typeCell := tview.NewTableCell(col.DataType).
			SetTextColor(accentColor).
			SetExpansion(1)
		m.columnsTable.SetCell(row, 1, typeCell)

		pkText := "[ ]"
		if col.IsPK {
			pkText = "[x]"
		}
		pkCell := tview.NewTableCell(pkText).
			SetTextColor(textColor).
			SetAlign(tview.AlignCenter).
			SetMaxWidth(6)
		m.columnsTable.SetCell(row, 2, pkCell)

		nullText := "[ ]"
		if col.IsNull {
			nullText = "[x]"
		}
		nullCell := tview.NewTableCell(nullText).
			SetTextColor(textColor).
			SetAlign(tview.AlignCenter).
			SetMaxWidth(6)
		m.columnsTable.SetCell(row, 3, nullCell)
	}

	m.columnsTable.Select(m.selectedCol+1, m.selectedField)
	m.columnsTable.ScrollToEnd()
	m.rebuildFlex()
}

func (m *CreateTableModal) updatePreview() {
	m.sqlPreview.SetText(m.generateDDL())
}

func (m *CreateTableModal) generateDDL() string {
	tableName := m.tableNameInput.GetText()
	if tableName == "" {
		return ""
	}

	var b strings.Builder
	fqName := tableName
	if m.schema != "" {
		fqName = m.schema + "." + tableName
	}

	fmt.Fprintf(&b, "CREATE TABLE %s (\n", fqName)

	var pkCols []string
	for _, col := range m.columns {
		if col.IsPK {
			pkCols = append(pkCols, col.Name)
		}
	}

	for i, col := range m.columns {
		b.WriteString("    ")
		b.WriteString(col.Name)
		b.WriteString(" ")
		b.WriteString(col.DataType)

		if !col.IsNull && !col.IsPK {
			b.WriteString(" NOT NULL")
		}

		isLast := i == len(m.columns)-1
		if !isLast || len(pkCols) > 0 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}

	if len(pkCols) > 0 {
		b.WriteString("    PRIMARY KEY (")
		b.WriteString(strings.Join(pkCols, ", "))
		b.WriteString(")\n")
	}

	b.WriteString(");")
	return b.String()
}

// GetTableName returns the current value of the table name input.
func (m *CreateTableModal) GetTableName() string {
	return m.tableNameInput.GetText()
}

// SetSchema sets the schema context for DDL generation.
func (m *CreateTableModal) SetSchema(schema string) {
	m.schema = schema
}

// Render shows the modal with the given schema context.
func (m *CreateTableModal) Render(defaultDDL string) {
	m.focus = focusTableName
	m.selectedCol = 0
	m.selectedField = 0
	m.editing = false

	m.columns = []ColumnDef{
		{Name: "id", DataType: "SERIAL", IsPK: true, IsNull: false},
	}
	m.tableNameInput.SetText("")

	m.updateSelectable()
	m.rebuildColumnsTable()
	m.updatePreview()

	m.columnsTable.SetSelectionChangedFunc(func(row, col int) {
		if row < 1 {
			return
		}
		m.selectedCol = row - 1
		m.selectedField = col
	})

	m.App.Pages.AddPage(CreateTableModalId, m, true, true)
}

// Draw positions the modal and draws its content.
// The Flex handles internal layout; we only overlay the edit field when active.
func (m *CreateTableModal) Draw(screen tcell.Screen) {
	screenW, screenH := screen.Size()
	const maxW = 100
	const padX = 4

	width := screenW - padX*2
	if width > maxW {
		width = maxW
	}
	height := screenH * 6 / 10

	x := (screenW - width) / 2
	y := (screenH - height) / 2

	m.Flex.SetRect(x, y, width, height)
	m.Flex.Draw(screen)

	styles := m.App.GetStyles()
	labelColor := styles.Global.SecondaryTextColor.Color()
	bgColor := styles.Global.BackgroundColor.Color()

	// Schema info in top-right corner of the border
	if m.schema != "" {
		schemaLabel := fmt.Sprintf("Schema: %s", m.schema)
		col := x + width - len(schemaLabel) - 3
		for i, ch := range schemaLabel {
			screen.SetContent(col+i, y, ch, nil, tcell.StyleDefault.
				Foreground(labelColor).Background(bgColor))
		}
	}

	// Edit field overlay positioned over the selected table row
	if m.editing {
		tx, ty, tw, _ := m.columnsTable.GetRect()
		editY := ty + m.selectedCol + 1 // +1 to skip header row
		if editY < ty+len(m.columns)+1 {
			m.editField.SetRect(tx, editY, tw/2, 1)
			m.editField.Draw(screen)
		}
	}
}

// Focus delegates to tableNameInput when in table-name mode,
// otherwise to the Flex (which routes to its Box for our InputCapture).
func (m *CreateTableModal) Focus(delegate func(p tview.Primitive)) {
	if m.focus == focusTableName {
		m.tableNameInput.Focus(delegate)
	} else {
		m.Flex.Focus(delegate)
	}
}

func (m *CreateTableModal) Hide() {
	m.App.Pages.RemovePage(CreateTableModalId)
}
