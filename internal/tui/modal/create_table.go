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
	*tview.Box

	tableNameInput *tview.InputField
	columnsTable   *tview.Table
	sqlPreview     *tview.TextView
	editField      *tview.InputField
	typeDropdown   *tview.List

	columns       []ColumnDef
	schema        string
	focus         focusArea
	selectedCol   int
	selectedField int // 0=name, 1=type, 2=pk, 3=null
	editing       bool
	showDropdown  bool

	applyCallback  func(ddl string) error
	cancelCallback func()
}

func NewCreateTableModal() *CreateTableModal {
	m := &CreateTableModal{
		BaseElement:  core.NewBaseElement(),
		Box:          tview.NewBox(),
		columnsTable: tview.NewTable(),
		sqlPreview:   tview.NewTextView(),
		editField:    tview.NewInputField(),
		typeDropdown: tview.NewList(),
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
	m.tableNameInput = tview.NewInputField()
	m.tableNameInput.SetLabel(" Table Name: ")
	m.tableNameInput.SetFieldWidth(40)
	m.tableNameInput.SetText("")

	m.columnsTable.SetSelectable(false, false)
	m.columnsTable.SetBorders(false)
	m.columnsTable.SetSeparator(' ')
	m.columnsTable.SetFixed(1, 0)

	m.sqlPreview.SetDynamicColors(true)
	m.sqlPreview.SetScrollable(true)

	m.typeDropdown.ShowSecondaryText(false)
	m.typeDropdown.SetHighlightFullLine(true)

	m.editField.SetFieldWidth(30)
}

func (m *CreateTableModal) setStyle() {
	styles := m.App.GetStyles()

	bgColor := styles.Global.BackgroundColor.Color()
	borderColor := styles.Global.BorderColor.Color()
	textColor := styles.Global.TextColor.Color()
	contrastBg := styles.Global.ContrastBackgroundColor.Color()
	secondaryText := styles.Global.SecondaryTextColor.Color()
	accentColor := styles.Global.MoreContrastBackgroundColor.Color()

	m.Box.SetBackgroundColor(bgColor)
	m.Box.SetBorderColor(borderColor)
	m.Box.SetTitleColor(styles.Global.TitleColor.Color())

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

	m.editField.SetBackgroundColor(bgColor)
	m.editField.SetFieldBackgroundColor(contrastBg)
	m.editField.SetFieldTextColor(textColor)

	m.typeDropdown.SetBackgroundColor(accentColor)
	m.typeDropdown.SetMainTextColor(textColor)
	m.typeDropdown.SetSelectedStyle(tcell.StyleDefault.
		Foreground(bgColor).
		Background(secondaryText))
	m.typeDropdown.SetBorderColor(secondaryText)
}

func (m *CreateTableModal) updateSelectable() {
	m.columnsTable.SetSelectable(m.focus == focusColumns, m.focus == focusColumns)
}

func (m *CreateTableModal) setKeybindings() {
	m.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		k := m.App.GetKeys()

		// Type dropdown mode
		if m.showDropdown {
			switch {
			case event.Key() == tcell.KeyEscape:
				m.showDropdown = false
				return nil
			case event.Key() == tcell.KeyEnter:
				if handler := m.typeDropdown.InputHandler(); handler != nil {
					handler(event, func(p tview.Primitive) {})
				}
				return nil
			case k.Contains(k.Navigation.MoveUp, event.Name()):
				synth := tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
				if handler := m.typeDropdown.InputHandler(); handler != nil {
					handler(synth, func(p tview.Primitive) {})
				}
				return nil
			case k.Contains(k.Navigation.MoveDown, event.Name()):
				synth := tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
				if handler := m.typeDropdown.InputHandler(); handler != nil {
					handler(synth, func(p tview.Primitive) {})
				}
				return nil
			default:
				if handler := m.typeDropdown.InputHandler(); handler != nil {
					handler(event, func(p tview.Primitive) {})
				}
				return nil
			}
		}

		// Inline edit mode
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
			if m.focus == focusTableName {
				m.focus = focusColumns
			} else {
				m.focus = focusTableName
			}
			m.updateSelectable()
			return nil
		}

		// Route to focused component
		if m.focus == focusTableName {
			if handler := m.tableNameInput.InputHandler(); handler != nil {
				handler(event, func(p tview.Primitive) {})
			}
			m.updatePreview()
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
	m.rebuildColumnsTable()
	m.updatePreview()
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
		m.editField.SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter {
				m.columns[m.selectedCol].Name = m.editField.GetText()
			}
			m.editing = false
			m.rebuildColumnsTable()
			m.updatePreview()
		})
	case 1: // data type - show dropdown
		m.showDropdown = true
		m.typeDropdown.Clear()
		for _, dt := range m.Driver.CommonDataTypes() {
			dt := dt
			m.typeDropdown.AddItem(dt, "", 0, func() {
				m.columns[m.selectedCol].DataType = dt
				m.showDropdown = false
				m.rebuildColumnsTable()
				m.updatePreview()
			})
		}
		for i, dt := range m.Driver.CommonDataTypes() {
			if dt == col.DataType {
				m.typeDropdown.SetCurrentItem(i)
				break
			}
		}
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

// Render shows the modal with the given schema context.
func (m *CreateTableModal) Render(defaultDDL string) {
	m.focus = focusTableName
	m.selectedCol = 0
	m.selectedField = 0
	m.editing = false
	m.showDropdown = false

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
	m.BroadcastEvent(manager.EventMsg{
		Message: manager.Message{
			Type: manager.FocusChanged,
			Data: tview.Identifier(CreateTableModalId),
		},
	})
}

// SetSchema sets the schema context for DDL generation.
func (m *CreateTableModal) SetSchema(schema string) {
	m.schema = schema
}

// Draw renders the entire modal with custom layout.
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

	m.SetRect(x, y, width, height)
	m.Box.SetBorder(true)
	m.Box.SetTitle(" Create New Table ")
	m.Box.SetTitleAlign(tview.AlignLeft)
	m.Box.DrawForSubclass(screen, m)

	innerX := x + 2
	innerY := y + 1
	innerW := width - 4
	innerH := height - 2

	styles := m.App.GetStyles()
	labelColor := styles.Global.SecondaryTextColor.Color()
	bgColor := styles.Global.BackgroundColor.Color()

	// Schema info in top-right
	if m.schema != "" {
		schemaLabel := fmt.Sprintf("Schema: %s", m.schema)
		col := x + width - len(schemaLabel) - 3
		for i, ch := range schemaLabel {
			screen.SetContent(col+i, y, ch, nil, tcell.StyleDefault.
				Foreground(labelColor).Background(bgColor))
		}
	}

	currentY := innerY + 1

	// Section: TABLE NAME
	m.drawLabel(screen, innerX, currentY, "TABLE NAME", labelColor, bgColor)
	currentY++

	m.tableNameInput.SetRect(innerX, currentY, innerW, 1)
	m.tableNameInput.Draw(screen)
	currentY += 2

	// Section: COLUMN DEFINITIONS
	m.drawLabel(screen, innerX, currentY, "COLUMN DEFINITIONS", labelColor, bgColor)
	currentY++

	// Column table height: header + rows, capped
	tableRows := len(m.columns) + 1
	remainingH := innerH - (currentY - innerY) - 6
	tableH := tableRows
	if tableH > remainingH/2 {
		tableH = remainingH / 2
	}
	if tableH < 3 {
		tableH = 3
	}

	columnsTableY := currentY
	m.columnsTable.SetRect(innerX, columnsTableY, innerW, tableH)
	m.columnsTable.Draw(screen)

	// Draw edit field overlay if editing
	if m.editing {
		editRow := m.selectedCol + 1
		editY := columnsTableY + editRow
		if editY < columnsTableY+tableH {
			m.editField.SetRect(innerX, editY, innerW/2, 1)
			m.editField.Draw(screen)
		}
	}

	currentY += tableH + 1

	// Section: SQL PREVIEW
	m.drawLabel(screen, innerX, currentY, "SQL PREVIEW", labelColor, bgColor)
	currentY++

	previewH := innerH - (currentY - innerY) - 1
	if previewH < 3 {
		previewH = 3
	}
	m.sqlPreview.SetRect(innerX, currentY, innerW, previewH)
	m.sqlPreview.SetBorder(true)
	m.sqlPreview.SetBorderColor(styles.Global.BorderColor.Color())
	m.sqlPreview.Draw(screen)

	// Draw type dropdown as last overlay so it renders on top of everything
	if m.showDropdown {
		editRow := m.selectedCol + 1
		dropX := innerX + innerW/3
		dropY := columnsTableY + editRow
		dropW := innerW / 3
		if dropW < 22 {
			dropW = 22
		}
		dropH := len(m.Driver.CommonDataTypes()) + 2 // +2 for border
		maxDropH := screenH - dropY - 1
		if dropH > maxDropH {
			dropH = maxDropH
		}
		if dropH < 5 {
			dropH = 5
		}
		m.typeDropdown.SetRect(dropX, dropY, dropW, dropH)
		m.typeDropdown.SetBorder(true)
		m.typeDropdown.Draw(screen)
	}
}

func (m *CreateTableModal) drawLabel(screen tcell.Screen, x, y int, text string, fg, bg tcell.Color) {
	style := tcell.StyleDefault.Foreground(fg).Background(bg)
	for i, ch := range text {
		screen.SetContent(x+i, y, ch, nil, style)
	}
}

// Focus delegates focus to the appropriate sub-component.
func (m *CreateTableModal) Focus(delegate func(p tview.Primitive)) {
	// Always focus self so SetInputCapture receives events
	m.Box.Focus(delegate)
}

// HasFocus returns true if the modal has focus.
func (m *CreateTableModal) HasFocus() bool {
	return m.Box.HasFocus()
}

func (m *CreateTableModal) Hide() {
	m.App.Pages.RemovePage(CreateTableModalId)
}
