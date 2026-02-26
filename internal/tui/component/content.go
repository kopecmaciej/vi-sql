package component

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
)

const (
	ContentId            = "Content"
	QueryBarId           = "QueryBar"
	SortBarId            = "SortBar"
	ContentDeleteModalId = "ContentDeleteModal"
)

// Content displays table rows in a grid with pagination, filtering,
// sorting, column hide/show, and row CRUD.
type Content struct {
	*core.BaseElement
	*core.Flex

	tableFlex    *core.Flex
	tableHeader  *core.TextView
	table        *core.Table
	style        *config.ContentStyle
	queryBar     *InputBar
	sortBar      *InputBar
	confirmModal *modal.Confirm
	peeker       *Peeker
	columns      []database.ColumnInfo
	state        *database.TableState
	stateMap     *database.StateMap
}

func NewContent() *Content {
	c := &Content{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),

		tableFlex:    core.NewFlex(),
		tableHeader:  core.NewTextView(),
		table:        core.NewTable(),
		queryBar:     NewInputBar(QueryBarId, "WHERE"),
		sortBar:      NewInputBar(SortBarId, "ORDER BY"),
		confirmModal: modal.NewConfirm(ContentDeleteModalId),
		peeker:       NewPeeker(),
		state:        &database.TableState{},
		stateMap:     database.NewStateMap(),
	}

	c.SetIdentifier(ContentId)
	c.table.SetIdentifier(ContentId)
	c.SetAfterInitFunc(c.init)

	return c
}

func (c *Content) init() error {
	ctx := context.Background()

	c.setLayout()
	c.setStyle()
	c.setKeybindings(ctx)

	if err := c.confirmModal.Init(c.App); err != nil {
		return err
	}
	if err := c.peeker.Init(c.App); err != nil {
		return err
	}
	if err := c.queryBar.Init(c.App); err != nil {
		return err
	}
	if err := c.sortBar.Init(c.App); err != nil {
		return err
	}

	c.queryBar.EnableAutocomplete()
	c.sortBar.EnableAutocomplete()

	c.queryBarHandler(ctx)
	c.sortBarHandler(ctx)

	c.handleEvents(ctx)

	return nil
}

func (c *Content) handleEvents(ctx context.Context) {
	go c.HandleEvents(ContentId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			c.setStyle()
			c.updateContent(ctx, true)
		}
	})
}

func (c *Content) setStyle() {
	c.style = &c.App.GetStyles().Content
	styles := c.App.GetStyles()

	c.tableFlex.SetStyle(styles)
	c.tableHeader.SetStyle(styles)
	c.Flex.SetStyle(styles)
	c.table.SetStyle(styles)

	c.tableFlex.SetBorderColor(styles.Others.SeparatorColor.Color())
	c.tableHeader.SetTextColor(c.style.StatusTextColor.Color())

	c.table.SetBordersColor(styles.Others.SeparatorColor.Color())
	c.table.SetSeparator(styles.Others.SeparatorSymbol.Rune())

	multiSelectedStyle := tcell.StyleDefault.
		Background(c.style.MultiSelectedRowColor.Color()).
		Foreground(tcell.ColorWhite)
	c.table.SetMultiSelectedStyle(multiSelectedStyle)
}

func (c *Content) setLayout() {
	c.tableFlex.SetBorder(true)
	c.tableFlex.SetDirection(tview.FlexRow)
	c.tableFlex.SetTitle(" Content ")
	c.tableFlex.SetTitleAlign(tview.AlignCenter)
	c.tableFlex.SetBorderPadding(0, 0, 1, 1)

	c.tableHeader.SetText("Rows: 0, Page: 0/0, Limit: 0")

	c.Flex.SetDirection(tview.FlexRow)
}

func (c *Content) setKeybindings(ctx context.Context) {
	k := c.App.GetKeys()

	c.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, col := c.table.GetSelection()
		switch {
		case k.Contains(k.Content.PeekRow, event.Name()):
			return c.handlePeekRow(ctx, row, false)
		case k.Contains(k.Content.FullPagePeek, event.Name()):
			return c.handlePeekRow(ctx, row, true)
		case k.Contains(k.Content.DeleteRow, event.Name()):
			return c.handleDeleteRow(ctx, row, col)
		case k.Contains(k.Content.CopyHighlight, event.Name()):
			return c.handleCopyCell(row, col)
		case k.Contains(k.Content.CopyRow, event.Name()):
			return c.handleCopyRow(row)
		case k.Contains(k.Content.Refresh, event.Name()):
			return c.handleRefresh(ctx)
		case k.Contains(k.Content.ToggleQueryBar, event.Name()):
			return c.handleToggleQuery()
		case k.Contains(k.Content.ToggleSortBar, event.Name()):
			return c.handleToggleSort()
		case k.Contains(k.Content.SortByColumn, event.Name()):
			return c.handleSortByColumn(ctx, col)
		case k.Contains(k.Content.HideColumn, event.Name()):
			return c.handleHideColumn(ctx, col)
		case k.Contains(k.Content.ResetHiddenColumns, event.Name()):
			return c.handleResetHiddenColumns(ctx)
		case k.Contains(k.Content.NextPage, event.Name()):
			return c.handleNextPage(ctx)
		case k.Contains(k.Content.PreviousPage, event.Name()):
			return c.handlePreviousPage(ctx)
		case k.Contains(k.Content.MultipleSelect, event.Name()):
			return c.handleMultipleSelect(row)
		case k.Contains(k.Content.ClearSelection, event.Name()):
			return c.handleClearSelection()
		}
		return event
	})
}

// HandleTableSelection is called when a schema/table is selected in the SchemaTree.
func (c *Content) HandleTableSelection(ctx context.Context, schema, table string) error {
	c.queryBar.SetText("")
	c.sortBar.SetText("")

	state, ok := c.stateMap.Get(c.stateMap.Key(schema, table))
	if ok {
		c.state = state
	} else {
		c.state = database.NewTableState(schema, table)

		conn := c.App.GetConfig().GetCurrentConnection()
		if conn != nil && conn.Options.Limit != nil {
			c.state.Limit = *conn.Options.Limit
		} else {
			_, _, _, height := c.table.GetInnerRect()
			c.state.Limit = int64(height - 1)
			if c.state.Limit <= 0 {
				c.state.Limit = 50
			}
		}
	}

	columns, err := c.Driver.GetTableColumns(ctx, schema, table)
	if err == nil {
		c.columns = columns
	}

	err = c.updateContent(ctx, false)
	if err != nil {
		return err
	}

	c.App.SetFocus(c)
	return nil
}

func (c *Content) Render() {
	c.Flex.Clear()
	c.tableFlex.Clear()

	var focusPrimitive tview.Primitive
	focusPrimitive = c

	if c.queryBar.IsEnabled() {
		c.Flex.AddItem(c.queryBar, 3, 0, false)
		focusPrimitive = c.queryBar
	}
	if c.sortBar.IsEnabled() {
		c.Flex.AddItem(c.sortBar, 3, 0, false)
		focusPrimitive = c.sortBar
	}

	c.tableFlex.AddItem(c.tableHeader, 2, 0, false)
	c.tableFlex.AddItem(c.table, 0, 1, true)

	c.Flex.AddItem(c.tableFlex, 0, 1, true)

	c.App.SetFocus(focusPrimitive)
}

func (c *Content) listRows(ctx context.Context) ([]database.Row, error) {
	countCallback := func(count int64) {
		c.state.Count = count
		c.App.QueueUpdateDraw(func() {
			c.tableHeader.SetText(c.buildHeaderInfo())
		})
	}

	rows, err := c.Driver.ListRows(ctx, c.state, c.state.Where, c.state.OrderBy, nil, countCallback)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	c.state.PopulateRows(rows)
	c.loadAutocompleteKeys(ctx)

	return rows, nil
}

func (c *Content) loadAutocompleteKeys(ctx context.Context) {
	cols, err := c.Driver.GetTableColumnNames(ctx, c.state.Schema, c.state.Table)
	if err != nil {
		return
	}
	c.queryBar.LoadAutocompleteKeys(cols)
	c.sortBar.LoadAutocompleteKeys(cols)

	c.App.GetManager().Broadcast(manager.EventMsg{
		Sender:  c.GetIdentifier(),
		Message: manager.Message{Type: manager.UpdateAutocompleteKeys, Data: cols},
	})
}

func (c *Content) updateContent(ctx context.Context, useState bool) error {
	c.table.ClearSelection()
	var rows []database.Row

	if useState {
		rows = c.state.GetAllRows()
	} else {
		r, err := c.listRows(ctx)
		if err != nil {
			return err
		}
		rows = r
	}

	c.table.Clear()
	c.tableHeader.SetText(c.buildHeaderInfo())
	c.stateMap.Set(c.stateMap.Key(c.state.Schema, c.state.Table), c.state)

	if len(rows) == 0 {
		c.table.SetCell(0, 0, tview.NewTableCell("No rows found"))
		return nil
	}

	c.renderTableView(rows)
	return nil
}

func (c *Content) renderTableView(rows []database.Row) {
	c.table.SetFixed(1, 0)
	c.table.SetSelectable(true, true)

	allCols := database.GetSortedColumnNames(rows[0])

	// Filter hidden columns
	hiddenCols := c.stateMap.GetHiddenColumns(c.state.Schema, c.state.Table)
	var visibleCols []string
	for _, col := range allCols {
		if !slices.Contains(hiddenCols, col) {
			visibleCols = append(visibleCols, col)
		}
	}

	// Build column type map for header display
	typeMap := make(map[string]string)
	for _, col := range c.columns {
		typeMap[col.Name] = col.DataType
	}

	// Header row: name (type)
	for col, name := range visibleCols {
		headerText := name
		if t, ok := typeMap[name]; ok {
			headerText = fmt.Sprintf("[%s]%s [%s]%s",
				c.style.ColumnKeyColor.String(), name,
				c.style.ColumnTypeColor.String(), t)
		}
		c.table.SetCell(0, col, tview.NewTableCell(headerText).
			SetReference(name).
			SetSelectable(false).
			SetBackgroundColor(c.style.HeaderRowBackgroundColor.Color()).
			SetAlign(tview.AlignCenter))
	}

	// Data rows
	for row, rowData := range rows {
		for col, colName := range visibleCols {
			cellText := database.StringifyValue(rowData[colName])
			if len(cellText) > 35 {
				cellText = cellText[:35] + "..."
			}

			cell := tview.NewTableCell(cellText).
				SetAlign(tview.AlignLeft).
				SetMaxWidth(35)

			c.table.SetCell(row+1, col, cell)
		}
	}
	c.table.Select(1, 0)
}

func (c *Content) buildHeaderInfo() string {
	headerInfo := fmt.Sprintf("Rows: %d, Page: %d/%d, Limit: %d",
		c.state.Count, c.state.GetCurrentPage(), c.state.GetTotalPages(), c.state.Limit)

	if c.state.Where != "" {
		headerInfo += fmt.Sprintf(" | WHERE: %s", c.state.Where)
	}
	if c.state.OrderBy != "" {
		headerInfo += fmt.Sprintf(" | ORDER BY: %s", c.state.OrderBy)
	}

	return headerInfo
}

// --- Query / Sort bar handlers ---

func (c *Content) queryBarHandler(ctx context.Context) {
	acceptFunc := func(text string) {
		c.state.SetWhere(text)
		err := c.updateContent(ctx, false)
		if err != nil {
			c.state.SetWhere("")
			modal.ShowError(c.App.Pages, "Error applying WHERE filter", err)
		} else {
			c.Flex.RemoveItem(c.queryBar)
			c.App.SetFocus(c.table)
		}
	}
	rejectFunc := func() {
		c.Flex.RemoveItem(c.queryBar)
		c.App.SetFocus(c.table)
	}
	c.queryBar.DoneFuncHandler(acceptFunc, rejectFunc)
}

func (c *Content) sortBarHandler(ctx context.Context) {
	acceptFunc := func(text string) {
		c.state.SetOrderBy(text)
		err := c.updateContent(ctx, false)
		if err != nil {
			c.state.SetOrderBy("")
			modal.ShowError(c.App.Pages, "Error applying ORDER BY", err)
		} else {
			c.Flex.RemoveItem(c.sortBar)
			c.App.SetFocus(c.table)
		}
	}
	rejectFunc := func() {
		c.Flex.RemoveItem(c.sortBar)
		c.App.SetFocus(c.table)
	}
	c.sortBar.DoneFuncHandler(acceptFunc, rejectFunc)
}

// --- Keybinding handlers ---

func (c *Content) handlePeekRow(_ context.Context, row int, fullScreen bool) *tcell.EventKey {
	if row < 1 {
		return nil
	}
	rows := c.state.GetAllRows()
	dataRow := row - 1
	if dataRow < 0 || dataRow >= len(rows) {
		return nil
	}

	c.peeker.ViewModal.SetFullScreen(fullScreen)
	c.peeker.Render(rows[dataRow], c.columns)
	return nil
}

func (c *Content) handleDeleteRow(ctx context.Context, row, col int) *tcell.EventKey {
	if row < 1 {
		return nil
	}

	pk := c.rowPrimaryKey(row)
	if pk == nil {
		return nil
	}

	c.confirmModal.SetConfirmButtonLabel("Delete")
	c.confirmModal.SetText("Are you sure you want to delete this row?")
	c.confirmModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		defer c.App.Pages.RemovePage(c.confirmModal.GetIdentifier())
		if buttonLabel == "Delete" {
			err := c.Driver.DeleteRows(ctx, c.state.Schema, c.state.Table, []database.PrimaryKey{*pk})
			if err != nil {
				modal.ShowError(c.App.Pages, "Error deleting row", err)
				return
			}
			c.state.DeleteRow(*pk)
			c.table.ClearSelection()
			c.updateContent(ctx, true)
			if row >= c.table.GetRowCount() {
				c.table.Select(row-1, col)
			} else {
				c.table.Select(row, col)
			}
		}
	})
	c.App.Pages.AddPage(c.confirmModal.GetIdentifier(), c.confirmModal, true, true)
	return nil
}

func (c *Content) rowPrimaryKey(row int) *database.PrimaryKey {
	pkCols := c.state.GetPrimaryKey()
	if len(pkCols) == 0 {
		return nil
	}

	allCols := c.getVisibleColumns()
	rows := c.state.GetAllRows()
	dataRow := row - 1 // account for header
	if dataRow < 0 || dataRow >= len(rows) {
		return nil
	}

	_ = allCols
	rowData := rows[dataRow]
	pk := database.PrimaryKey{Columns: make(map[string]any)}
	for _, col := range pkCols {
		pk.Columns[col] = rowData[col]
	}
	return &pk
}

func (c *Content) getVisibleColumns() []string {
	rows := c.state.GetAllRows()
	if len(rows) == 0 {
		return nil
	}
	allCols := database.GetSortedColumnNames(rows[0])
	hiddenCols := c.stateMap.GetHiddenColumns(c.state.Schema, c.state.Table)
	var visible []string
	for _, col := range allCols {
		if !slices.Contains(hiddenCols, col) {
			visible = append(visible, col)
		}
	}
	return visible
}

func (c *Content) handleCopyCell(row, col int) *tcell.EventKey {
	cell := c.table.GetCell(row, col)
	if cell == nil {
		return nil
	}
	clipboard.WriteAll(cell.Text)
	return nil
}

func (c *Content) handleCopyRow(row int) *tcell.EventKey {
	if row < 1 {
		return nil
	}
	cols := c.getVisibleColumns()
	rows := c.state.GetAllRows()
	dataRow := row - 1
	if dataRow < 0 || dataRow >= len(rows) {
		return nil
	}
	rowData := rows[dataRow]

	var parts []string
	for _, col := range cols {
		parts = append(parts, fmt.Sprintf("%s: %s", col, database.StringifyValue(rowData[col])))
	}
	clipboard.WriteAll(strings.Join(parts, ", "))
	return nil
}

func (c *Content) handleRefresh(ctx context.Context) *tcell.EventKey {
	err := c.updateContent(ctx, false)
	if err != nil {
		modal.ShowError(c.App.Pages, "Error refreshing rows", err)
	}
	return nil
}

func (c *Content) handleToggleQuery() *tcell.EventKey {
	if c.state.Where != "" {
		c.queryBar.Toggle(c.state.Where)
	} else {
		c.queryBar.Toggle("")
	}
	c.Render()
	return nil
}

func (c *Content) handleToggleSort() *tcell.EventKey {
	if c.state.OrderBy != "" {
		c.sortBar.Toggle(c.state.OrderBy)
	} else {
		c.sortBar.Toggle("")
	}
	c.Render()
	return nil
}

func (c *Content) handleSortByColumn(ctx context.Context, col int) *tcell.EventKey {
	headerCell := c.table.GetCell(0, col)
	if headerCell == nil {
		return nil
	}
	columnName, _ := headerCell.GetReference().(string)
	if columnName == "" {
		columnName = headerCell.Text
	}
	currentSort := c.state.OrderBy

	var newSort string
	if currentSort == columnName+" ASC" {
		newSort = columnName + " DESC"
	} else {
		newSort = columnName + " ASC"
	}

	c.state.SetOrderBy(newSort)
	c.updateContent(ctx, false)
	c.table.Select(1, col)
	return nil
}

func (c *Content) handleHideColumn(ctx context.Context, col int) *tcell.EventKey {
	headerCell := c.table.GetCell(0, col)
	if headerCell == nil {
		return nil
	}
	columnName, _ := headerCell.GetReference().(string)
	if columnName == "" {
		columnName = headerCell.Text
	}
	c.stateMap.AddHiddenColumn(c.state.Schema, c.state.Table, columnName)
	c.updateContent(ctx, true)
	return nil
}

func (c *Content) handleResetHiddenColumns(ctx context.Context) *tcell.EventKey {
	c.stateMap.ResetHiddenColumns(c.state.Schema, c.state.Table)
	c.updateContent(ctx, true)
	return nil
}

func (c *Content) handleNextPage(ctx context.Context) *tcell.EventKey {
	if c.state.Offset+c.state.Limit >= c.state.Count {
		return nil
	}
	c.state.SetOffset(c.state.Offset + c.state.Limit)
	c.stateMap.Set(c.stateMap.Key(c.state.Schema, c.state.Table), c.state)
	c.updateContent(ctx, false)
	return nil
}

func (c *Content) handlePreviousPage(ctx context.Context) *tcell.EventKey {
	if c.state.Offset == 0 {
		return nil
	}
	c.state.SetOffset(c.state.Offset - c.state.Limit)
	c.stateMap.Set(c.stateMap.Key(c.state.Schema, c.state.Table), c.state)
	c.updateContent(ctx, false)
	return nil
}

func (c *Content) handleMultipleSelect(row int) *tcell.EventKey {
	c.table.ToggleRowSelection(row)
	return nil
}

func (c *Content) handleClearSelection() *tcell.EventKey {
	c.table.ClearSelection()
	return nil
}
