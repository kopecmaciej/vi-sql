package component

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
	"github.com/kopecmaciej/vi-sql/internal/tui/widget"
)

const (
	ContentId            = "Content"
	FilterBarId          = "FilterBar"
	SortBarId            = "SortBar"
	QueryBarId           = "QueryBar"
	ContentDeleteModalId = "ContentDeleteModal"
	ContentEditModalId   = "ContentEditModal"
)

// Content displays table rows in a grid with pagination, filtering,
// sorting, column hide/show, and row CRUD.
type Content struct {
	*core.BaseElement
	*core.Flex

	tableFlex    *core.Flex
	resultsBar   *widget.ResultsBar
	table        *core.Table
	style        *config.ContentStyle
	filterBar    *InputBar
	sortBar      *InputBar
	queryBar     *InputBar
	sqlEditor    *SQLEditor
	inlineEdit   *modal.InlineEditModal
	confirmModal *modal.Confirm
	peeker       *Peeker
	columns      []database.ColumnInfo
	state        *database.TableState
	stateMap     *database.StateMap
	lastExecTime time.Duration
	countPending bool
}

func NewContent() *Content {
	c := &Content{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),

		tableFlex:    core.NewFlex(),
		resultsBar:   widget.NewResultsBar(),
		table:        core.NewTable(),
		filterBar:    NewInputBar(FilterBarId, "WHERE"),
		sortBar:      NewInputBar(SortBarId, "ORDER BY"),
		queryBar:     NewInputBar(QueryBarId, "SQL"),
		sqlEditor:    NewSQLEditor(),
		inlineEdit:   modal.NewInlineEditModal(),
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

	if err := c.sqlEditor.Init(c.App); err != nil {
		return err
	}
	if err := c.inlineEdit.Init(c.App); err != nil {
		return err
	}
	if err := c.confirmModal.Init(c.App); err != nil {
		return err
	}
	if err := c.peeker.Init(c.App); err != nil {
		return err
	}
	if err := c.filterBar.Init(c.App); err != nil {
		return err
	}
	if err := c.sortBar.Init(c.App); err != nil {
		return err
	}
	if err := c.queryBar.Init(c.App); err != nil {
		return err
	}

	c.filterBar.EnableAutocomplete()
	c.sortBar.EnableAutocomplete()
	c.queryBar.EnableAutocomplete()
	c.queryBar.EnableHistory()

	c.filterBarHandler(ctx)
	c.sortBarHandler(ctx)
	c.queryBarHandler(ctx)

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
	c.resultsBar.SetStyle(styles)
	c.Flex.SetStyle(styles)
	c.table.SetStyle(styles)

	c.tableFlex.SetBorderColor(styles.Others.SeparatorColor.Color())

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
		case k.Contains(k.Content.InlineEdit, event.Name()):
			return c.handleInlineEdit(ctx, row, col)
		case k.Contains(k.Content.EditRow, event.Name()):
			return c.handleEditRow(ctx, row)
		case k.Contains(k.Content.AddRow, event.Name()):
			c.handleAddRow(ctx)
			return nil
		case k.Contains(k.Content.DuplicateRow, event.Name()):
			c.handleDuplicateRow(ctx, row)
			return nil
		case k.Contains(k.Content.DeleteRow, event.Name()):
			return c.handleDeleteRow(ctx, row, col)
		case k.Contains(k.Content.CopyValue, event.Name()):
			return c.handleCopyCell(row, col)
		case k.Contains(k.Content.CopyRow, event.Name()):
			return c.handleCopyRow(row)
		case k.Contains(k.Content.Refresh, event.Name()):
			return c.handleRefresh(ctx)
		case k.Contains(k.Content.ToggleFilterBar, event.Name()):
			return c.handleToggleFilter()
		case k.Contains(k.Content.ToggleQueryBar, event.Name()):
			return c.handleToggleQueryBar()
		case k.Contains(k.Content.OpenEditor, event.Name()):
			c.handleOpenEditor(ctx)
			return nil
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
	c.filterBar.SetText("")
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
		var pkCols []string
		for _, col := range columns {
			if col.IsPK {
				pkCols = append(pkCols, col.Name)
			}
		}
		c.state.SetPrimaryKey(pkCols)
	}

	err = c.updateContent(ctx, false)
	if err != nil {
		return err
	}

	c.App.SetFocus(c)
	return nil
}

// Reset clears stale table data and state from a previous connection so a
// fresh table selection starts from a clean slate.
func (c *Content) Reset() {
	c.table.Clear()
	c.resultsBar.Clear()
	c.state = &database.TableState{}
	c.stateMap = database.NewStateMap()
	c.columns = nil
}

func (c *Content) Render() {
	c.Flex.Clear()
	c.tableFlex.Clear()

	var focusPrimitive tview.Primitive
	focusPrimitive = c

	if c.filterBar.IsEnabled() {
		c.Flex.AddItem(c.filterBar, 3, 0, false)
		focusPrimitive = c.filterBar
	}
	if c.sortBar.IsEnabled() {
		c.Flex.AddItem(c.sortBar, 3, 0, false)
		focusPrimitive = c.sortBar
	}
	if c.queryBar.IsEnabled() {
		c.Flex.AddItem(c.queryBar, 3, 0, false)
		focusPrimitive = c.queryBar
	}

	c.tableFlex.AddItem(c.resultsBar, 2, 0, false)
	c.tableFlex.AddItem(c.table, 0, 1, true)

	c.Flex.AddItem(c.tableFlex, 0, 1, true)

	c.App.SetFocus(focusPrimitive)
}

func (c *Content) listRows(ctx context.Context) ([]database.Row, error) {
	start := time.Now()
	c.countPending = c.state.Count == 0

	countCallback := func(count int64) {
		c.state.Count = count
		c.countPending = false
		c.App.QueueUpdateDraw(func() {
			c.resultsBar.Render(c.state, c.lastExecTime, c.countPending)
		})
	}

	var (
		query string
		rows  []database.Row
		err   error
	)

	if c.state.RawSQL != "" {
		var cols []database.ColumnInfo
		query, rows, cols, err = c.Driver.ListQueryRows(ctx, c.state.RawSQL, c.state.Limit, c.state.Offset, countCallback)
		if err != nil {
			return nil, err
		}
		c.columns = cols
	} else {
		query, rows, err = c.Driver.ListRows(ctx, c.state, c.state.Where, c.state.OrderBy, nil, countCallback)
		if err != nil {
			return nil, err
		}
	}

	c.lastExecTime = time.Since(start)
	c.state.LastQuery = query

	if len(rows) == 0 {
		return nil, nil
	}

	c.state.PopulateRows(rows)
	if c.state.RawSQL == "" {
		c.loadAutocompleteKeys(ctx)
	}

	return rows, nil
}

func (c *Content) loadAutocompleteKeys(ctx context.Context) {
	cols, err := c.Driver.GetTableColumnNames(ctx, c.state.Schema, c.state.Table)
	if err != nil {
		return
	}
	c.filterBar.LoadAutocompleteKeys(cols)
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
	c.resultsBar.Render(c.state, c.lastExecTime, c.countPending)
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

	allCols := c.orderedColumnNames(rows[0])

	// Filter hidden columns
	hiddenCols := c.stateMap.GetHiddenColumns(c.state.Schema, c.state.Table)
	var visibleCols []string
	for _, col := range allCols {
		if !slices.Contains(hiddenCols, col) {
			visibleCols = append(visibleCols, col)
		}
	}

	// Build column type map for header display and bool detection
	typeMap := make(map[string]string)
	boolCols := make(map[string]bool)
	for _, col := range c.columns {
		typeMap[col.Name] = database.AbbreviateTypeName(col.DataType)
		if col.DataType == "boolean" {
			boolCols[col.Name] = true
		}
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
			if boolCols[colName] {
				switch cellText {
				case "t":
					cellText = "true"
				case "f":
					cellText = "false"
				}
			}
			if len(cellText) > 35 {
				cellText = cellText[:35] + "..."
			}

			cell := tview.NewTableCell(cellText).
				SetAlign(tview.AlignLeft).
				SetMaxWidth(30)

			c.table.SetCell(row+1, col, cell)
		}
	}
	c.table.Select(1, 0)
}

func (c *Content) filterBarHandler(ctx context.Context) {
	acceptFunc := func(text string) {
		c.state.SetWhere(text)
		err := c.updateContent(ctx, false)
		if err != nil {
			c.state.SetWhere("")
			modal.ShowError(c.App.Pages, "Error applying WHERE filter", err)
		} else {
			c.filterBar.Disable()
			c.Flex.RemoveItem(c.filterBar)
			c.App.SetFocus(c.table)
		}
	}
	rejectFunc := func() {
		c.Flex.RemoveItem(c.filterBar)
		c.App.SetFocus(c.table)
	}
	c.filterBar.DoneFuncHandler(acceptFunc, rejectFunc)
}

func (c *Content) sortBarHandler(ctx context.Context) {
	acceptFunc := func(text string) {
		c.state.SetOrderBy(text)
		err := c.updateContent(ctx, false)
		if err != nil {
			c.state.SetOrderBy("")
			modal.ShowError(c.App.Pages, "Error applying ORDER BY", err)
		} else {
			c.sortBar.Disable()
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

	// Collect primary keys: prefer multi-selected rows, fall back to cursor row.
	selectedRows := c.table.GetSelectedRows()
	var pks []database.PrimaryKey
	if len(selectedRows) > 0 {
		for _, r := range selectedRows {
			if pk := c.rowPrimaryKey(r); pk != nil {
				pks = append(pks, *pk)
			}
		}
	} else {
		pk := c.rowPrimaryKey(row)
		if pk == nil {
			return nil
		}
		pks = []database.PrimaryKey{*pk}
	}

	if len(pks) == 0 {
		return nil
	}

	confirmText := "Are you sure you want to delete this row?"
	if len(pks) > 1 {
		confirmText = fmt.Sprintf("Are you sure you want to delete %d rows?", len(pks))
	}

	c.confirmModal.SetConfirmButtonLabel("Delete")
	c.confirmModal.SetText(confirmText)
	c.confirmModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		defer c.App.Pages.RemovePage(c.confirmModal.GetIdentifier())
		if buttonLabel == "Delete" {
			err := c.Driver.DeleteRows(ctx, c.state.Schema, c.state.Table, pks)
			if err != nil {
				modal.ShowError(c.App.Pages, "Error deleting row", err)
				return
			}
			for _, pk := range pks {
				c.state.DeleteRow(pk)
			}
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

	rows := c.state.GetAllRows()
	dataRow := row - 1 // account for header
	if dataRow < 0 || dataRow >= len(rows) {
		return nil
	}

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
	allCols := c.orderedColumnNames(rows[0])
	hiddenCols := c.stateMap.GetHiddenColumns(c.state.Schema, c.state.Table)
	var visible []string
	for _, col := range allCols {
		if !slices.Contains(hiddenCols, col) {
			visible = append(visible, col)
		}
	}
	return visible
}

// orderedColumnNames returns column names in their ordinal_position order
// using c.columns metadata. Falls back to alphabetical if metadata is absent.
func (c *Content) orderedColumnNames(row database.Row) []string {
	if len(c.columns) > 0 {
		names := make([]string, 0, len(c.columns))
		for _, col := range c.columns {
			if _, ok := row[col.Name]; ok {
				names = append(names, col.Name)
			}
		}
		return names
	}
	return database.GetSortedColumnNames(row)
}

func (c *Content) handleCopyCell(row, col int) *tcell.EventKey {
	if row < 1 {
		return nil
	}
	headerCell := c.table.GetCell(0, col)
	if headerCell == nil {
		return nil
	}
	colName, ok := headerCell.GetReference().(string)
	if !ok {
		return nil
	}
	rows := c.state.GetAllRows()
	dataRow := row - 1
	if dataRow < 0 || dataRow >= len(rows) {
		return nil
	}
	clipboard.WriteAll(database.StringifyValue(rows[dataRow][colName]))
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

func (c *Content) handleToggleFilter() *tcell.EventKey {
	if c.state.Where != "" {
		c.filterBar.Toggle(c.state.Where)
	} else {
		c.filterBar.Toggle("")
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

func (c *Content) handleToggleQueryBar() *tcell.EventKey {
	text := c.state.LastQuery
	if c.state.RawSQL != "" {
		text = c.state.RawSQL
	}
	c.queryBar.Toggle(text)
	c.Render()
	return nil
}

func (c *Content) handleOpenEditor(ctx context.Context) {
	sql, err := c.sqlEditor.Open("")
	if err != nil {
		modal.ShowError(c.App.Pages, "Editor error", err)
		return
	}
	if sql == "" {
		return
	}

	if isSelectQuery(sql) {
		sqlState := database.NewTableState("", "")
		sqlState.RawSQL = sql
		sqlState.Limit = c.state.Limit

		start := time.Now()
		query, rows, cols, err := c.Driver.ListQueryRows(ctx, sql, sqlState.Limit, 0, func(count int64) {
			sqlState.Count = count
			c.App.QueueUpdateDraw(func() {
				c.resultsBar.Render(c.state, c.lastExecTime, false)
			})
		})
		if err != nil {
			modal.ShowError(c.App.Pages, "Query error", err)
			return
		}
		execTime := time.Since(start)
		sqlState.LastQuery = query
		sqlState.PopulateRows(rows)

		c.App.QueueUpdateDraw(func() {
			c.state = sqlState
			c.columns = cols
			c.lastExecTime = execTime
			c.countPending = sqlState.Count == 0

			c.table.Clear()
			c.resultsBar.Render(c.state, c.lastExecTime, c.countPending)

			if len(rows) == 0 {
				c.table.SetFixed(0, 0)
				c.table.SetSelectable(false, false)
				c.table.SetCell(0, 0, tview.NewTableCell("No rows returned"))
				return
			}
			c.renderTableView(rows)
		})
	} else {
		start := time.Now()
		affected, err := c.Driver.ExecuteStatement(ctx, sql)
		if err != nil {
			modal.ShowError(c.App.Pages, "Statement error", err)
			return
		}
		execTime := time.Since(start)
		c.App.QueueUpdateDraw(func() {
			c.showStatementResult(affected, execTime)
		})
	}
}

// queryBarHandler wires the QueryBar's accept/reject callbacks.
// On Enter it detects whether the SQL is a SELECT-like query or a
// DML/DDL statement and dispatches accordingly.
func (c *Content) queryBarHandler(ctx context.Context) {
	acceptFunc := func(text string) {
		text = strings.TrimSpace(text)
		if text == "" {
			c.queryBar.Disable()
			c.Flex.RemoveItem(c.queryBar)
			c.App.SetFocus(c.table)
			return
		}

		if isSelectQuery(text) {
			sqlState := database.NewTableState("", "")
			sqlState.RawSQL = text
			sqlState.Limit = c.state.Limit
			c.state = sqlState
			if err := c.updateContent(ctx, false); err != nil {
				// Keep the bar open so the user can fix the query.
				modal.ShowError(c.App.Pages, "Query error", err)
				return
			}
		} else {
			start := time.Now()
			affected, err := c.Driver.ExecuteStatement(ctx, text)
			if err != nil {
				// Keep the bar open so the user can fix the statement.
				modal.ShowError(c.App.Pages, "Statement error", err)
				return
			}
			c.showStatementResult(affected, time.Since(start))
		}

		c.queryBar.Disable()
		c.Flex.RemoveItem(c.queryBar)
		c.App.SetFocus(c.table)

		if err := c.queryBar.SaveToHistory(text); err != nil {
			modal.ShowError(c.App.Pages, "Failed to save history", err)
		}
	}
	rejectFunc := func() {
		c.Flex.RemoveItem(c.queryBar)
		c.App.SetFocus(c.table)
	}
	c.queryBar.DoneFuncHandler(acceptFunc, rejectFunc)
}

// showStatementResult updates the table area after a non-SELECT statement.
func (c *Content) showStatementResult(affected int64, execTime time.Duration) {
	c.table.Clear()
	c.table.SetFixed(0, 0)
	c.table.SetSelectable(false, false)
	c.resultsBar.RenderStatementResult(affected, execTime)
	c.table.SetCell(0, 0, tview.NewTableCell(
		fmt.Sprintf("%d rows affected", affected)))
}

// handleInlineEdit opens the InlineEditModal pre-filled with the current cell value.
// On confirm the cell's column is updated via Driver.UpdateRow.
func (c *Content) handleInlineEdit(ctx context.Context, row, col int) *tcell.EventKey {
	if row < 1 {
		return nil
	}

	pk := c.rowPrimaryKey(row)
	if pk == nil {
		return nil
	}

	headerCell := c.table.GetCell(0, col)
	if headerCell == nil {
		return nil
	}
	colName, _ := headerCell.GetReference().(string)
	if colName == "" {
		return nil
	}

	rows := c.state.GetAllRows()
	dataRow := row - 1
	if dataRow < 0 || dataRow >= len(rows) {
		return nil
	}
	originalRow := rows[dataRow]
	currentValue := c.App.GetFormatter().EditableString(originalRow[colName])

	c.inlineEdit.SetApplyCallback(func(fieldName, newValue string) error {
		updatedRow := make(database.Row)
		for k, v := range originalRow {
			updatedRow[k] = v
		}
		// If the displayed string is unchanged keep the original typed value
		// so the DB driver doesn't need to reparse it (avoids type coercion issues).
		if newValue != c.App.GetFormatter().EditableString(originalRow[fieldName]) {
			updatedRow[fieldName] = newValue
		}

		if err := c.Driver.UpdateRow(ctx, c.state.Schema, c.state.Table, *pk, originalRow, updatedRow); err != nil {
			return err
		}
		c.state.UpdateRow(*pk, updatedRow)
		c.inlineEdit.Hide()
		c.App.SetFocus(c.table)
		c.updateContent(ctx, true)
		c.table.Select(row, col)
		return nil
	})

	c.inlineEdit.SetCancelCallback(func() {
		c.inlineEdit.Hide()
		c.App.SetFocus(c.table)
		c.table.Select(row, col)
	})

	c.inlineEdit.Render(colName, currentValue)
	return nil
}

// handleAddRow opens the external editor pre-filled with an INSERT SQL template.
// The user fills in values and saves; the statement is executed immediately.
// On execution error the user can choose Fix (reopen editor with their SQL) or Cancel.
func (c *Content) handleAddRow(ctx context.Context) {
	if len(c.columns) == 0 {
		return
	}
	var openEditor func(sql string)
	openEditor = func(sql string) {
		edited, err := c.sqlEditor.Open(sql)
		if err != nil {
			modal.ShowError(c.App.Pages, "Editor error", err)
			return
		}
		if edited == "" {
			return
		}
		_, err = c.Driver.ExecuteStatement(ctx, edited)
		if err != nil {
			modal.ShowErrorWithRetry(c.App.Pages, "Insert error", err, func() {
				openEditor(edited)
			})
			return
		}
		c.updateContent(ctx, false)
	}
	openEditor(c.buildInsertSQL())
}

func (c *Content) buildInsertSQL() string {
	return database.BuildInsertSQL(c.state.Schema, c.state.Table, c.columns)
}

func (c *Content) buildDuplicateInsertSQL(row database.Row) string {
	colMeta := make(map[string]database.ColumnInfo, len(c.columns))
	for _, col := range c.columns {
		colMeta[col.Name] = col
	}
	return database.BuildDuplicateInsertSQL(
		c.state.Schema, c.state.Table,
		c.orderedColumnNames(row), colMeta,
		c.App.GetFormatter().SQLLiteral, row,
	)
}

// handleDuplicateRow opens the external editor pre-filled with an INSERT
// statement containing all values from the selected row (auto-generated
// columns omitted). On save the statement is executed and the table refreshes.
// On execution error the user can choose Fix (reopen editor with their SQL) or Cancel.
func (c *Content) handleDuplicateRow(ctx context.Context, row int) {
	if row < 1 {
		return
	}
	rows := c.state.GetAllRows()
	dataRow := row - 1
	if dataRow < 0 || dataRow >= len(rows) {
		return
	}

	var openEditor func(sql string)
	openEditor = func(sql string) {
		edited, err := c.sqlEditor.Open(sql)
		if err != nil {
			modal.ShowError(c.App.Pages, "Editor error", err)
			return
		}
		if edited == "" {
			return
		}
		_, err = c.Driver.ExecuteStatement(ctx, edited)
		if err != nil {
			modal.ShowErrorWithRetry(c.App.Pages, "Insert error", err, func() {
				openEditor(edited)
			})
			return
		}
		c.updateContent(ctx, false)
	}
	openEditor(c.buildDuplicateInsertSQL(rows[dataRow]))
}

// handleEditRow opens the external editor pre-filled with an UPDATE SQL template
// for the current row. The edited statement is executed on save.
// On execution error the user can choose Fix (reopen editor with their SQL) or Cancel.
func (c *Content) handleEditRow(ctx context.Context, row int) *tcell.EventKey {
	if row < 1 {
		return nil
	}

	pk := c.rowPrimaryKey(row)
	if pk == nil {
		return nil
	}

	rows := c.state.GetAllRows()
	dataRow := row - 1
	if dataRow < 0 || dataRow >= len(rows) {
		return nil
	}

	var openEditor func(sql string)
	openEditor = func(sql string) {
		edited, err := c.sqlEditor.Open(sql)
		if err != nil {
			modal.ShowError(c.App.Pages, "Editor error", err)
			return
		}
		if edited == "" {
			return
		}
		_, err = c.Driver.ExecuteStatement(ctx, edited)
		if err != nil {
			modal.ShowErrorWithRetry(c.App.Pages, "Update error", err, func() {
				openEditor(edited)
			})
			return
		}
		c.updateContent(ctx, false)
	}
	openEditor(c.buildUpdateSQL(rows[dataRow], pk))
	return nil
}

func (c *Content) buildUpdateSQL(row database.Row, pk *database.PrimaryKey) string {
	return database.BuildUpdateSQL(
		c.state.Schema, c.state.Table,
		c.orderedColumnNames(row), c.state.GetPrimaryKey(),
		c.App.GetFormatter().SQLLiteral, row, *pk,
	)
}

// isSelectQuery returns true when sql is a statement that returns rows.
func isSelectQuery(sql string) bool {
	upper := strings.ToUpper(sql)
	return strings.HasPrefix(upper, "SELECT") ||
		strings.HasPrefix(upper, "WITH") ||
		strings.HasPrefix(upper, "EXPLAIN") ||
		strings.HasPrefix(upper, "TABLE")
}
