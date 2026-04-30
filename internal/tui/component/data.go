package component

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
	"github.com/kopecmaciej/vi-sql/internal/tui/widget"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

const (
	DataId            = "Data"
	FilterBarId       = "FilterBar"
	SortBarId         = "SortBar"
	DataDeleteModalId = "DataDeleteModal"
	DataEditModalId   = "DataEditModal"

	maxQueryResultRows = 100
)

// SQL editor size states toggled by the Fullscreen key.
const (
	editorSizeNormal     = 0 // 30/70 proportional split
	editorSizeFullscreen = 1 // editor fills the tab, results hidden
)

// QueryTabMode controls which keybindings and features are active.
type QueryTabMode int

const (
	// TableMode: pre-filled SELECT, full CRUD keybindings. Used when opening a
	// table directly from the schema tree.
	TableMode QueryTabMode = iota
	// QueryMode: blank editor, read-only results. Used for ad-hoc query tabs.
	QueryMode
)

// dataTabCounter generates unique identifiers for each Data/QueryTab instance
// so that multiple tabs can subscribe to the event system without colliding.
var dataTabCounter int32

func nextDataID() string {
	n := atomic.AddInt32(&dataTabCounter, 1)
	return fmt.Sprintf("QueryTab-%d", n)
}

// Data displays table rows in a grid with pagination, filtering,
// sorting, column hide/show, and row CRUD.
type Data struct {
	*core.BaseElement
	*core.Flex

	mode           QueryTabMode
	tableFlex      *core.Flex
	resultsBar     *widget.ResultsBar
	table          *core.Table
	style          *config.DataStyle
	filterBar      *InputBar
	sortBar        *InputBar
	termEditor     *TermEditor
	sqlQueryEditor *SQLQueryEditor
	editorSize     int
	inlineEdit     *modal.InlineEditModal
	confirmModal   *modal.Confirm
	exportModal    *modal.ExportModal
	sqlEditModal   *SQLEditModal
	peeker         *Peeker
	explainViewer  *ExplainViewer
	columns        []database.ColumnInfo
	foreignKeys    []database.ForeignKeyInfo
	state          *database.TableState
	stateMap       *database.StateMap
	lastExecTime   time.Duration
	countPending   bool
	// TODO: refactor - move some of the logic elsewhere to not have 5k file next week
	cancelQuery  context.CancelFunc
	queryRunning bool
}

func newData(mode QueryTabMode) *Data {
	id := tview.Identifier(nextDataID())
	c := &Data{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),

		mode:           mode,
		tableFlex:      core.NewFlex(),
		resultsBar:     widget.NewResultsBar(),
		table:          core.NewTable(),
		filterBar:      NewInputBar(id+"-filter", "WHERE"),
		sortBar:        NewInputBar(id+"-sort", "ORDER BY"),
		termEditor:     NewTermEditor(),
		sqlQueryEditor: NewSQLQueryEditor(),
		editorSize:     editorSizeNormal,
		inlineEdit:     modal.NewInlineEditModal(),
		confirmModal:   modal.NewConfirm(id + "-delete"),
		exportModal:    modal.NewExportModal(),
		sqlEditModal:   NewSQLEditModal(),
		peeker:         NewPeeker(),
		explainViewer:  NewExplainViewer(),
		state:          &database.TableState{},
		stateMap:       database.NewStateMap(),
	}

	c.SetIdentifier(id)
	if mode == QueryMode {
		c.table.SetIdentifier(id + "-results")
	} else {
		c.table.SetIdentifier(id)
	}
	c.SetAfterInitFunc(c.init)

	return c
}

// NewData creates a blank query-mode tab (no CRUD, empty editor).
func NewData() *Data {
	return newData(QueryMode)
}

// NewTableTab creates a table-mode tab with full CRUD keybindings.
// Callers must follow up with HandleTableSelection to load data.
func NewTableTab() *Data {
	return newData(TableMode)
}

func (c *Data) init() error {
	ctx := context.Background()

	c.setLayout()
	c.setStyle()
	c.setKeybindings(ctx)

	if err := c.termEditor.Init(c.App); err != nil {
		return err
	}
	if err := c.sqlQueryEditor.Init(c.App); err != nil {
		return err
	}
	c.sqlQueryEditor.SetColumnFetcher(func(schema, table string) ([]string, error) {
		return c.Driver.GetTableColumnNames(context.Background(), schema, table)
	})
	c.sqlQueryEditor.SetOnFullscreen(func() {
		c.toggleFullscreen()
	})
	c.sqlQueryEditor.SetOnFocusDown(func() {
		c.App.SetFocusOnly(c.table)
	})
	c.sqlQueryEditor.SetOnOpenInEditor(func() {
		if c.App.GetConfig().Editor.Enabled {
			c.handleTermEditorForQuery()
		}
	})
	c.sqlQueryEditor.SetOnCancel(func() {
		if c.queryRunning && c.cancelQuery != nil {
			c.cancelQuery()
		}
	})
	c.sqlQueryEditor.SetOnExecute(func(sql string) {
		if c.editorSize == editorSizeFullscreen {
			c.toggleFullscreen()
		}
		go func() {
			if c.cancelQuery != nil {
				c.cancelQuery()
			}
			queryCtx, cancel := context.WithCancel(context.Background())
			c.cancelQuery = cancel
			c.queryRunning = true
			defer func() {
				c.queryRunning = false
				c.cancelQuery = nil
			}()

			c.App.QueueUpdateDraw(func() {
				c.resultsBar.RenderRunning()
			})

			if isExplainQuery(sql) {
				c.runExplain(queryCtx, sql)
				return
			}
			if isSelectQuery(sql) {
				sqlState := database.NewTableState("", "")
				sqlState.RawSQL = sql
				sqlState.BatchSize = c.state.BatchSize
				if sqlState.BatchSize <= 0 {
					_, _, _, height := c.table.GetInnerRect()
					sqlState.BatchSize = int64(height - 1)
					if sqlState.BatchSize <= 0 {
						sqlState.BatchSize = 50
					}
				}

				start := time.Now()
				query, rows, cols, err := c.Driver.ListQueryRows(queryCtx, sql, sqlState.BatchSize, 0, func(count int64) {
					sqlState.Count = count
					c.App.QueueUpdateDraw(func() {
						c.resultsBar.Render(sqlState, c.lastExecTime, false)
					})
				})
				if err != nil {
					if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
						c.App.QueueUpdateDraw(func() {
							c.resultsBar.RenderCancelled()
						})
						return
					}
					c.App.QueueUpdateDraw(func() {
						modal.ShowErrorWithDone(c.App.Pages, "Query error", err, func() {
							c.resultsBar.RestorePrevious()
						})
					})
					return
				}
				execTime := time.Since(start)
				c.sqlQueryEditor.SaveQueryToHistory(sql)
				sqlState.LastQuery = query
				if val, ok := database.ExtractLimitValue(sql); ok {
					sqlState.UserLimit = val
				}
				sqlState.PopulateRows(rows)

				colNames := make([]string, len(cols))
				for i, col := range cols {
					colNames[i] = col.Name
				}
				capped := rows
				if len(capped) > maxQueryResultRows {
					capped = capped[:maxQueryResultRows]
				}
				c.App.GetManager().Broadcast(manager.NewQueryExecutedMsg(manager.QueryResult{
					Query:    sql,
					Columns:  colNames,
					Rows:     capped,
					RowCount: len(rows),
				}))

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
				if !c.confirmIfDestructive(queryCtx, sql) {
					c.executeStatement(queryCtx, sql)
				}
			}
		}()
	})
	if err := c.sqlEditModal.Init(c.App); err != nil {
		return err
	}
	if err := c.inlineEdit.Init(c.App); err != nil {
		return err
	}
	if err := c.confirmModal.Init(c.App); err != nil {
		return err
	}
	if err := c.exportModal.Init(c.App); err != nil {
		return err
	}
	if err := c.peeker.Init(c.App); err != nil {
		return err
	}
	if err := c.explainViewer.Init(c.App); err != nil {
		return err
	}
	if err := c.filterBar.Init(c.App); err != nil {
		return err
	}
	if err := c.sortBar.Init(c.App); err != nil {
		return err
	}

	c.filterBar.EnableColumnAutocomplete(database.OperatorKeywords)
	c.sortBar.EnableColumnAutocomplete(database.OrderKeywords)

	c.filterBarHandler(ctx)
	c.sortBarHandler(ctx)

	c.handleEvents(ctx)

	return nil
}

func (c *Data) handleEvents(ctx context.Context) {
	go c.HandleEvents(c.GetIdentifier(), func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			c.setStyle()
			_ = c.updateData(ctx, true)
		}
	})
}

func (c *Data) setStyle() {
	c.style = &c.App.GetStyles().Data
	styles := c.App.GetStyles()
	sqlEditorStyle := &styles.SQLEditor
	c.filterBar.EnableHighlighting(sqlEditorStyle)
	c.sortBar.EnableHighlighting(sqlEditorStyle)

	c.tableFlex.SetStyle(styles)
	c.resultsBar.SetStyle(styles)
	c.Flex.SetStyle(styles)
	c.table.SetStyle(styles)

	c.table.SetBordersColor(styles.Others.SeparatorColor.Color())
	c.table.SetSeparator(styles.Icons.Separator.Rune())

	multiSelectedStyle := tcell.StyleDefault.
		Background(c.style.MultiSelectedRowColor.Color()).
		Foreground(tcell.ColorWhite)
	c.table.SetMultiSelectedStyle(multiSelectedStyle)
}

func (c *Data) setLayout() {
	c.tableFlex.SetBorder(true)
	c.tableFlex.SetDirection(tview.FlexRow)
	c.tableFlex.SetTitle(" Data ")
	c.tableFlex.SetTitleAlign(tview.AlignCenter)
	c.tableFlex.SetBorderPadding(0, 0, 1, 1)

	c.Flex.SetDirection(tview.FlexRow)
}

func (c *Data) setKeybindings(ctx context.Context) {
	k := c.App.GetKeys()

	c.table.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, col := c.table.GetSelection()
		switch {
		case k.Match(k.Navigation.GoTop, event):
			if c.table.GetRowCount() > 1 {
				c.table.Select(1, col)
			}
			return nil
		case k.Match(k.Navigation.GoBottom, event):
			if rc := c.table.GetRowCount(); rc > 1 {
				c.table.Select(rc-1, col)
			}
			return nil
		case k.Match(k.Data.PeekRow, event):
			return c.handlePeekRow(ctx, row, false)
		case k.Match(k.Data.FullPagePeek, event):
			return c.handlePeekRow(ctx, row, true)
		case k.Match(k.Common.Copy, event):
			return c.handleCopyCell(row, col)
		case k.Match(k.Data.CopyRow, event):
			return c.handleCopyRow(row)
		case k.Match(k.Common.Refresh, event):
			return c.handleRefresh(ctx)
		case k.Match(k.Navigation.FocusUp, event):
			if c.mode == QueryMode {
				c.App.SetFocusOnly(c.sqlQueryEditor)
				return nil
			}
		case k.Match(k.Data.HideColumn, event):
			return c.handleHideColumn(ctx, col)
		case k.Match(k.Data.ResetHiddenColumns, event):
			return c.handleResetHiddenColumns(ctx)
		case k.Match(k.Data.NextPage, event):
			return c.handleNextPage(ctx)
		case k.Match(k.Data.PreviousPage, event):
			return c.handlePreviousPage(ctx)
		case k.Match(k.Data.MultipleSelect, event):
			return c.handleMultipleSelect(row)
		case k.Match(k.Data.ClearSelection, event):
			if c.queryRunning && c.cancelQuery != nil {
				c.cancelQuery()
				return nil
			}
			return c.handleClearSelection()
		case k.Match(k.Data.ExplainQuery, event):
			if c.state.LastQuery != "" {
				go c.runExplain(ctx, c.state.LastQuery)
			}
			return nil
		case k.Match(k.Data.ExportData, event):
			return c.handleExportData(ctx)
		case k.Match(k.Data.FollowForeignKey, event):
			return c.handleFollowForeignKey(ctx, row, col)
		}

		// SortByColumn works in both modes.
		if k.Match(k.Data.SortByColumn, event) {
			return c.handleSortByColumn(ctx, col)
		}

		// CRUD keybindings — only available in TableMode.
		if c.mode == TableMode {
			switch {
			case k.Match(k.Common.Edit, event):
				return c.handleInlineEdit(ctx, row, col)
			case k.Match(k.Data.EditRow, event):
				return c.handleEditRow(ctx, row)
			case k.Match(k.Common.Add, event):
				c.handleAddRow(ctx)
				return nil
			case k.Match(k.Data.DuplicateRow, event):
				c.handleDuplicateRow(ctx, row)
				return nil
			case k.Match(k.Common.Delete, event):
				return c.handleDeleteRow(ctx, row, col)
			case k.Match(k.Common.Filter, event):
				return c.handleToggleFilter()
			case k.Match(k.Data.ToggleSortBar, event):
				return c.handleToggleSort()
			}
		}

		return event
	}))
}

// TabOptions carries optional initial state for a new table tab.
type TabOptions struct {
	Where string
}

func (c *Data) HandleTableSelection(ctx context.Context, schema, table string, opts ...TabOptions) error {
	c.filterBar.SetText("")
	c.sortBar.SetText("")

	state, ok := c.stateMap.Get(c.stateMap.Key(schema, table))
	if ok {
		c.state = state
	} else {
		c.state = database.NewTableState(schema, table)

		conn := c.App.GetConfig().GetCurrentConnection()
		if conn != nil && conn.Options.Limit != nil {
			c.state.BatchSize = *conn.Options.Limit
		} else {
			_, _, _, height := c.table.GetInnerRect()
			c.state.BatchSize = int64(height - 1)
			if c.state.BatchSize <= 0 {
				c.state.BatchSize = 50
			}
		}

		if len(opts) > 0 && opts[0].Where != "" {
			c.state.Where = opts[0].Where
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

	c.foreignKeys, _ = c.Driver.GetTableForeignKeys(ctx, schema, table)

	err = c.updateData(ctx, false)
	if err != nil {
		return err
	}

	c.App.SetFocus(c)
	return nil
}

func (c *Data) Reset() {
	c.table.Clear()
	c.resultsBar.Clear()
	c.state = &database.TableState{}
	c.stateMap = database.NewStateMap()
	c.columns = nil
}

func (c *Data) SetSchemasForAutocomplete(schemas []database.Schema) {
	c.filterBar.SetSchemas(schemas)
	c.sortBar.SetSchemas(schemas)
	c.sqlQueryEditor.SetSchemas(schemas)
	c.sqlEditModal.SetSchemas(schemas)
}

// IsQueryTab reports whether this tab is in query mode (not a table tab).
func (c *Data) IsQueryTab() bool {
	return c.mode == QueryMode
}

// IsCleanQueryTab reports whether this tab has never loaded any table data,
// making it safe to replace without losing user work.
func (c *Data) IsCleanQueryTab() bool {
	return c.mode == QueryMode && c.state.Table == "" && c.state.RawSQL == "" &&
		strings.TrimSpace(c.sqlQueryEditor.GetText()) == ""
}

// HasResults reports whether the tab currently has query results loaded.
func (c *Data) HasResults() bool {
	return c.state != nil && (c.state.Table != "" || c.state.RawSQL != "")
}

// SelectedTable returns the schema and table currently loaded in this tab.
// Returns empty strings for query-mode tabs or tabs with no table loaded.
func (c *Data) SelectedTable() (schema, table string) {
	if c.state == nil || c.mode == QueryMode {
		return "", ""
	}
	return c.state.Schema, c.state.Table
}

func (c *Data) SetEditorText(text string) {
	c.sqlQueryEditor.SetText(text, true)
}

func (c *Data) SetEditorTextAndExecute(text string) {
	c.sqlQueryEditor.SetText(text, true)
	c.sqlQueryEditor.Execute()
}

func (c *Data) GetEditorText() string { return c.sqlQueryEditor.GetText() }

// GetFocusPrimitive returns the inner primitive that should receive focus
// when this tab is activated from outside (e.g. tab switching).
func (c *Data) GetFocusPrimitive() tview.Primitive {
	if c.mode == QueryMode {
		return c.sqlQueryEditor
	}
	return c.table
}

func (c *Data) Render() {
	c.Flex.Clear()
	c.tableFlex.Clear()

	c.tableFlex.AddItem(c.resultsBar, 2, 0, false)
	c.tableFlex.AddItem(c.table, 0, 1, true)

	var focusPrimitive tview.Primitive

	if c.mode == QueryMode {
		focusPrimitive = c.sqlQueryEditor
		switch c.editorSize {
		case editorSizeFullscreen:
			c.Flex.AddItem(c.sqlQueryEditor, 0, 1, true)
		default: // editorSizeNormal
			c.Flex.AddItem(c.sqlQueryEditor, 0, 3, true)
			c.Flex.AddItem(c.tableFlex, 0, 7, true)
		}
	} else {
		// TableMode: filter/sort bars sit above the table
		focusPrimitive = c.table
		if c.filterBar.IsEnabled() {
			c.Flex.AddItem(c.filterBar, 3, 0, false)
			focusPrimitive = c.filterBar
		}
		if c.sortBar.IsEnabled() {
			c.Flex.AddItem(c.sortBar, 3, 0, false)
			focusPrimitive = c.sortBar
		}
		c.Flex.AddItem(c.tableFlex, 0, 1, true)
	}

	c.App.SetFocus(focusPrimitive)
}

func (c *Data) listRows(_ context.Context) ([]database.Row, error) {
	// Cancel any in-flight query (also stops its internal count goroutine).
	if c.cancelQuery != nil {
		c.cancelQuery()
	}
	queryCtx, cancel := context.WithCancel(context.Background())
	c.cancelQuery = cancel
	c.queryRunning = true
	defer func() {
		c.queryRunning = false
		c.cancelQuery = nil
	}()

	// TODO: HandleTableSelection should call listRows in a goroutine (like SetOnExecute)
	// so Esc can cancel table fetches too. Requires updateData UI calls wrapped in
	// QueueUpdateDraw and error handling moved into the goroutine.

	// For table mode on first visit, pre-fill with a fast estimate so the
	// results bar shows a number immediately while the exact count runs.
	c.countPending = c.state.Count == 0
	if c.state.RawSQL == "" && c.state.Count == 0 {
		if est, err := c.Driver.GetEstimatedRowCount(queryCtx, c.state.Schema, c.state.Table); err == nil && est > 0 {
			c.state.Count = est
		}
	}

	start := time.Now()
	thisState := c.state
	countCallback := func(count int64) {
		thisState.Count = count
		if c.state == thisState {
			c.countPending = false
		}
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
		query, rows, cols, err = c.Driver.ListQueryRows(queryCtx, c.state.RawSQL, c.state.BatchSize, c.state.Offset, countCallback)
		if err != nil {
			return nil, err
		}
		if val, ok := database.ExtractLimitValue(c.state.RawSQL); ok {
			c.state.UserLimit = val
		}
		c.columns = cols
	} else {
		query, rows, err = c.Driver.ListRows(queryCtx, c.state, c.state.Where, c.state.OrderBy, nil, countCallback)
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
		c.loadAutocompleteKeys(queryCtx)
	}

	return rows, nil
}

func (c *Data) loadAutocompleteKeys(ctx context.Context) {
	cols, err := c.Driver.GetTableColumnNames(ctx, c.state.Schema, c.state.Table)
	if err != nil {
		return
	}
	c.filterBar.LoadAutocompleteKeys(cols)
	c.sortBar.LoadAutocompleteKeys(cols)
	c.sqlQueryEditor.SetColumnsForTable(c.state.Schema, c.state.Table, cols)

	msg := manager.NewUpdateAutocompleteKeysMsg(cols)
	msg.Sender = c.GetIdentifier()
	c.App.GetManager().Broadcast(msg)

	schemas, err := c.Driver.ListSchemas(ctx, "")
	if err != nil {
		return
	}
	c.filterBar.SetSchemas(schemas)
	c.sortBar.SetSchemas(schemas)
	c.sqlQueryEditor.SetSchemas(schemas)
}

func (c *Data) updateData(ctx context.Context, useState bool) error {
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

func (c *Data) renderTableView(rows []database.Row) {
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

	// Build column metadata maps for header display
	typeMap := make(map[string]string)
	boolCols := make(map[string]bool)
	pkCols := make(map[string]bool)
	icons := c.App.GetStyles().Icons
	for _, col := range c.columns {
		typeMap[col.Name] = icons.TypeSymbol(col.DataType)
		if col.DataType == "boolean" {
			boolCols[col.Name] = true
		}
		if col.IsPK {
			pkCols[col.Name] = true
		}
	}

	// Header row: [key] name type
	for col, name := range visibleCols {
		headerText := name
		if t, ok := typeMap[name]; ok {
			pkPrefix := ""
			if pkCols[name] {
				if c.App.GetConfig().Styles.NerdFont {
					pkPrefix = fmt.Sprintf("[%s]\uF084 ", c.App.GetStyles().Global.SecondaryTextColor.String())
				} else {
					pkPrefix = fmt.Sprintf("[%s]* ", c.App.GetStyles().Global.SecondaryTextColor.String())
				}
			}
			headerText = fmt.Sprintf("%s[%s]%s [%s]%s ",
				pkPrefix,
				c.App.GetStyles().Global.SecondaryTextColor.String(), name,
				c.App.GetStyles().Global.MoreContrastBackgroundColor.String(), t)
		}
		c.table.SetCell(0, col, tview.NewTableCell(headerText).
			SetReference(name).
			SetSelectable(false).
			SetBackgroundColor(c.App.GetStyles().Global.ContrastBackgroundColor.Color()).
			SetAlign(tview.AlignCenter))
	}

	// Data rows
	for row, rowData := range rows {
		for col, colName := range visibleCols {
			isNull := rowData[colName] == nil
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
			if isNull {
				cellText = fmt.Sprintf("[%s]NULL[-:-:-]", c.App.GetStyles().Global.DimColor)
			}

			cell := tview.NewTableCell(cellText).
				SetAlign(tview.AlignLeft).
				SetMaxWidth(30)

			c.table.SetCell(row+1, col, cell)
		}
	}
	c.table.Select(1, 0)
}

func (c *Data) filterBarHandler(ctx context.Context) {
	acceptFunc := func(text string) {
		if c.state.RawSQL != "" {
			c.state.RawSQL = database.RebuildSelectSQL(c.state.RawSQL, text, c.state.OrderBy)
		}
		c.state.SetWhere(text)
		err := c.updateData(ctx, false)
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

func (c *Data) sortBarHandler(ctx context.Context) {
	acceptFunc := func(text string) {
		if c.state.RawSQL != "" {
			c.state.RawSQL = database.RebuildSelectSQL(c.state.RawSQL, c.state.Where, text)
		}
		c.state.SetOrderBy(text)
		err := c.updateData(ctx, false)
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

func (c *Data) handlePeekRow(_ context.Context, row int, fullScreen bool) *tcell.EventKey {
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

func (c *Data) handleDeleteRow(ctx context.Context, row, col int) *tcell.EventKey {
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
	c.confirmModal.SetOnConfirm(func() {
		defer c.App.Pages.RemovePage(c.confirmModal.GetIdentifier())
		err := c.Driver.DeleteRows(ctx, c.state.Schema, c.state.Table, pks)
		if err != nil {
			modal.ShowError(c.App.Pages, "Error deleting row", err)
			return
		}
		for _, pk := range pks {
			c.state.DeleteRow(pk)
		}
		c.table.ClearSelection()
		if err := c.updateData(ctx, true); err != nil {
			modal.ShowError(c.App.Pages, "Error refreshing rows", err)
			return
		}
		if row >= c.table.GetRowCount() {
			c.table.Select(row-1, col)
		} else {
			c.table.Select(row, col)
		}
	})
	c.App.Pages.AddPage(c.confirmModal.GetIdentifier(), c.confirmModal, true, true)
	return nil
}

func (c *Data) rowPrimaryKey(row int) *database.PrimaryKey {
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

// executeStatement runs sql as a non-SELECT statement and updates the results bar.
func (c *Data) executeStatement(ctx context.Context, sql string) {
	start := time.Now()
	affected, err := c.Driver.ExecuteStatement(ctx, sql)
	if err != nil {
		c.App.QueueUpdateDraw(func() {
			modal.ShowErrorWithDone(c.App.Pages, "Statement error", err, func() {
				c.resultsBar.RestorePrevious()
			})
		})
		return
	}
	execTime := time.Since(start)
	c.sqlQueryEditor.SaveQueryToHistory(sql)
	c.App.GetManager().Broadcast(manager.NewQueryExecutedMsg(manager.QueryResult{
		Query:    sql,
		Affected: affected,
	}))
	c.App.QueueUpdateDraw(func() {
		c.state.RawSQL = sql
		c.showStatementResult(affected, execTime)
	})
}

func (c *Data) confirmIfDestructive(ctx context.Context, sql string) bool {
	conn := c.App.GetConfig().GetCurrentConnection()
	if conn != nil {
		opts := conn.GetOptions()
		if opts.AlwaysConfirmActions != nil && !*opts.AlwaysConfirmActions {
			return false
		}
	}

	info := database.HasDestructiveStatement(sql)
	if info == nil {
		return false
	}

	c.App.QueueUpdateDraw(func() {
		var text strings.Builder
		if info.Table != "" {
			text.WriteString(info.Operation + " on [::b]" + info.Table + "[::-]")
		} else {
			text.WriteString(info.Operation + " statement")
		}
		if (info.Operation == "DELETE" || info.Operation == "UPDATE") && !info.HasWhere {
			text.WriteString("\n\n[red]No WHERE clause — all rows will be affected.[white]")
		}
		text.WriteString("\n\nExecute this statement?")

		c.confirmModal.SetConfirmButtonLabel("Execute")
		c.confirmModal.SetText(text.String())
		c.confirmModal.SetOnConfirm(func() {
			c.App.Pages.RemovePage(c.confirmModal.GetIdentifier())
			go c.executeStatement(ctx, sql)
		})
		c.App.Pages.AddPage(c.confirmModal.GetIdentifier(), c.confirmModal, true, true)
		c.App.SetFocusOnly(c.confirmModal)
	})
	return true
}

func (c *Data) getVisibleColumns() []string {
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
func (c *Data) orderedColumnNames(row database.Row) []string {
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

func (c *Data) handleCopyCell(row, col int) *tcell.EventKey {
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
	util.Copy(database.StringifyValue(rows[dataRow][colName]))
	return nil
}

func (c *Data) handleCopyRow(row int) *tcell.EventKey {
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
	util.Copy(strings.Join(parts, ", "))
	return nil
}

func (c *Data) handleRefresh(ctx context.Context) *tcell.EventKey {
	err := c.updateData(ctx, false)
	if err != nil {
		modal.ShowError(c.App.Pages, "Error refreshing rows", err)
	}
	return nil
}

func (c *Data) handleToggleFilter() *tcell.EventKey {
	if c.state.Where != "" {
		c.filterBar.Toggle(c.state.Where)
	} else {
		c.filterBar.Toggle("")
	}
	c.Render()
	return nil
}

func (c *Data) handleToggleSort() *tcell.EventKey {
	if c.state.OrderBy != "" {
		c.sortBar.Toggle(c.state.OrderBy)
	} else {
		c.sortBar.Toggle("")
	}
	c.Render()
	return nil
}

func (c *Data) handleSortByColumn(ctx context.Context, col int) *tcell.EventKey {
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

	if c.mode == QueryMode && c.state.RawSQL != "" {
		c.state.RawSQL = database.RebuildSelectSQL(c.state.RawSQL, c.state.Where, newSort)
	}
	c.state.SetOrderBy(newSort)
	if err := c.updateData(ctx, false); err != nil {
		modal.ShowError(c.App.Pages, "Error sorting rows", err)
	}
	c.table.Select(1, col)
	return nil
}

func (c *Data) handleHideColumn(ctx context.Context, col int) *tcell.EventKey {
	headerCell := c.table.GetCell(0, col)
	if headerCell == nil {
		return nil
	}
	columnName, _ := headerCell.GetReference().(string)
	if columnName == "" {
		columnName = headerCell.Text
	}
	row, _ := c.table.GetSelection()
	c.stateMap.AddHiddenColumn(c.state.Schema, c.state.Table, columnName)
	if err := c.updateData(ctx, true); err != nil {
		modal.ShowError(c.App.Pages, "Error refreshing rows", err)
		return nil
	}
	newCol := col
	if newColCount := c.table.GetColumnCount(); newCol >= newColCount {
		newCol = newColCount - 1
	}
	if newCol < 0 {
		newCol = 0
	}
	c.table.Select(row, newCol)
	return nil
}

func (c *Data) handleResetHiddenColumns(ctx context.Context) *tcell.EventKey {
	c.stateMap.ResetHiddenColumns(c.state.Schema, c.state.Table)
	if err := c.updateData(ctx, true); err != nil {
		modal.ShowError(c.App.Pages, "Error refreshing rows", err)
	}
	return nil
}

func (c *Data) handleNextPage(ctx context.Context) *tcell.EventKey {
	if c.state.Offset+c.state.BatchSize >= c.state.Count {
		return nil
	}
	c.state.SetOffset(c.state.Offset + c.state.BatchSize)
	c.stateMap.Set(c.stateMap.Key(c.state.Schema, c.state.Table), c.state)
	if err := c.updateData(ctx, false); err != nil {
		modal.ShowError(c.App.Pages, "Error loading page", err)
	}
	return nil
}

func (c *Data) handlePreviousPage(ctx context.Context) *tcell.EventKey {
	if c.state.Offset == 0 {
		return nil
	}
	c.state.SetOffset(c.state.Offset - c.state.BatchSize)
	c.stateMap.Set(c.stateMap.Key(c.state.Schema, c.state.Table), c.state)
	if err := c.updateData(ctx, false); err != nil {
		modal.ShowError(c.App.Pages, "Error loading page", err)
	}
	return nil
}

func (c *Data) handleMultipleSelect(row int) *tcell.EventKey {
	c.table.ToggleRowSelection(row)
	return nil
}

func (c *Data) handleClearSelection() *tcell.EventKey {
	c.table.ClearSelection()
	return nil
}

// handleFollowForeignKey opens a new table tab for the referenced table, pre-filtered
// to the row that the current cell's FK value points to.
func (c *Data) handleFollowForeignKey(_ context.Context, row, col int) *tcell.EventKey {
	if row < 1 || len(c.foreignKeys) == 0 {
		return nil
	}

	headerCell := c.table.GetCell(0, col)
	if headerCell == nil {
		return nil
	}
	colName, ok := headerCell.GetReference().(string)
	if !ok || colName == "" {
		return nil
	}

	var fk *database.ForeignKeyInfo
	for i := range c.foreignKeys {
		for _, fkCol := range c.foreignKeys[i].Columns {
			if fkCol == colName {
				fk = &c.foreignKeys[i]
				break
			}
		}
		if fk != nil {
			break
		}
	}
	if fk == nil {
		return nil
	}

	rows := c.state.GetAllRows()
	dataRow := row - 1
	if dataRow < 0 || dataRow >= len(rows) {
		return nil
	}
	rowData := rows[dataRow]

	lit := c.App.GetFormatter().SQLLiteral
	var whereParts []string
	for i, fkColName := range fk.Columns {
		val := rowData[fkColName]
		if val == nil {
			return nil
		}
		whereParts = append(whereParts, fmt.Sprintf(`"%s" = %s`, fk.ReferencedCols[i], lit(val)))
	}

	c.App.GetManager().Broadcast(manager.NewOpenTableTabMsg(manager.TableTabRequest{
		Schema: fk.ReferencedSchema,
		Table:  fk.ReferencedTable,
		Where:  strings.Join(whereParts, " AND "),
	}))

	return nil
}

// handleTermEditorForQuery opens $EDITOR pre-filled with the current query editor text.
func (c *Data) handleTermEditorForQuery() {
	currentText := c.sqlQueryEditor.GetText()
	edited, err := c.termEditor.Open(currentText)
	if err != nil {
		modal.ShowError(c.App.Pages, "Editor error", err)
		return
	}
	if edited == "" {
		return
	}
	c.sqlQueryEditor.SetText(edited, true)
}

// runEditorStatement opens the configured editor (built-in modal or external $EDITOR)
// pre-filled with initialSQL, executes the result as a SQL statement, and retries on
// error. The table is refreshed on success. modalTitle is shown in the modal header
// (e.g. "EDIT", "ADD", "DUPLICATE").
func (c *Data) runEditorStatement(ctx context.Context, modalTitle, initialSQL, errorTitle string) {
	if !c.App.GetConfig().Editor.Enabled {
		var openModal func(sql string)
		openModal = func(sql string) {
			c.sqlEditModal.Open(modalTitle, sql, func(editedSQL string) {
				go func() {
					_, err := c.Driver.ExecuteStatement(ctx, editedSQL)
					if err != nil {
						c.App.QueueUpdateDraw(func() {
							modal.ShowErrorWithRetry(c.App.Pages, errorTitle, err, func() {
								openModal(editedSQL)
							})
						})
						return
					}
					if err := c.updateData(ctx, false); err != nil {
						c.App.QueueUpdateDraw(func() {
							modal.ShowError(c.App.Pages, "Error refreshing rows", err)
						})
					}
				}()
			})
		}
		openModal(initialSQL)
		return
	}

	var openEditor func(sql string)
	openEditor = func(sql string) {
		edited, err := c.termEditor.Open(sql)
		if err != nil {
			modal.ShowError(c.App.Pages, "Editor error", err)
			return
		}
		if edited == "" {
			return
		}
		_, err = c.Driver.ExecuteStatement(ctx, edited)
		if err != nil {
			modal.ShowErrorWithRetry(c.App.Pages, errorTitle, err, func() {
				openEditor(edited)
			})
			return
		}
		if err := c.updateData(ctx, false); err != nil {
			modal.ShowError(c.App.Pages, "Error refreshing rows", err)
		}
	}
	openEditor(initialSQL)
}

func (c *Data) toggleFullscreen() {
	if c.editorSize == editorSizeFullscreen {
		c.editorSize = editorSizeNormal
	} else {
		c.editorSize = editorSizeFullscreen
	}
	c.Render()
}

func (c *Data) showStatementResult(affected int64, execTime time.Duration) {
	c.table.Clear()
	c.table.SetFixed(0, 0)
	c.table.SetSelectable(false, false)
	c.resultsBar.RenderStatementResult(affected, execTime)
	c.table.SetCell(0, 0, tview.NewTableCell(
		fmt.Sprintf("%d rows affected", affected)))
}

func (c *Data) runExplain(ctx context.Context, sql string) {
	bare, userWantsAnalyze := parseExplainPrefix(sql)
	var result string
	var err error
	if userWantsAnalyze {
		result, err = c.Driver.ExplainAnalyze(ctx, bare)
	} else {
		result, err = c.Driver.ExplainPlan(ctx, bare)
	}
	if err != nil {
		c.App.QueueUpdateDraw(func() {
			modal.ShowError(c.App.Pages, "Explain error", err)
		})
		return
	}
	c.sqlQueryEditor.SaveQueryToHistory(sql)
	c.App.QueueUpdateDraw(func() {
		c.showExplainViewer(ctx, bare, result, userWantsAnalyze)
	})
}

func (c *Data) showExplainViewer(ctx context.Context, sql, result string, analyze bool) {
	c.explainViewer.analyzeMode = analyze
	c.explainViewer.SetExplainFunc(func(analyze bool) (string, error) {
		if analyze {
			return c.Driver.ExplainAnalyze(ctx, sql)
		}
		return c.Driver.ExplainPlan(ctx, sql)
	})
	c.explainViewer.Render(result)
	c.explainViewer.SetDoneFunc(func() {
		c.App.Pages.RemovePage(ExplainViewerId)
	})
	c.App.Pages.AddPage(ExplainViewerId, c.explainViewer, true, true)
	c.App.SetFocusOnly(c.explainViewer.tree.TreeView)
}

func isExplainQuery(sql string) bool {
	return strings.HasPrefix(strings.ToUpper(strings.TrimSpace(sql)), "EXPLAIN")
}

// parseExplainPrefix strips any leading EXPLAIN / EXPLAIN ANALYZE / EXPLAIN (...)
// prefix and reports whether the user explicitly requested ANALYZE.
func parseExplainPrefix(sql string) (bare string, analyze bool) {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	if !strings.HasPrefix(upper, "EXPLAIN") {
		return sql, false
	}
	rest := strings.TrimSpace(sql[7:])
	restUpper := strings.ToUpper(rest)
	// Parenthesised options: EXPLAIN (ANALYZE, FORMAT JSON)
	if strings.HasPrefix(restUpper, "(") {
		end := strings.Index(rest, ")")
		if end >= 0 {
			opts := strings.ToUpper(rest[1:end])
			analyze = strings.Contains(opts, "ANALYZE")
			rest = strings.TrimSpace(rest[end+1:])
			restUpper = strings.ToUpper(rest)
		}
	}

	if strings.HasPrefix(restUpper, "ANALYZE") {
		analyze = true
		rest = strings.TrimSpace(rest[7:])
	}
	return rest, analyze
}

func (c *Data) handleInlineEdit(ctx context.Context, row, col int) *tcell.EventKey {
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
		maps.Copy(updatedRow, originalRow)
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
		if err := c.updateData(ctx, true); err != nil {
			modal.ShowError(c.App.Pages, "Error refreshing rows", err)
		}
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

// handleAddRow opens the configured editor pre-filled with an INSERT SQL template.
// The user fills in values and saves; the statement is executed immediately.
// On execution error the user can choose Fix (reopen editor with their SQL) or Cancel.
func (c *Data) handleAddRow(ctx context.Context) {
	if len(c.columns) == 0 {
		return
	}
	c.runEditorStatement(ctx, "ADD", c.buildInsertSQL(), "Insert error")
}

func (c *Data) buildInsertSQL() string {
	return database.BuildInsertSQL(c.state.Schema, c.state.Table, c.columns)
}

func (c *Data) buildDuplicateInsertSQL(row database.Row) string {
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

// handleDuplicateRow opens the configured editor pre-filled with an INSERT
// statement containing all values from the selected row (auto-generated
// columns omitted). On save the statement is executed and the table refreshes.
// On execution error the user can choose Fix (reopen editor with their SQL) or Cancel.
func (c *Data) handleDuplicateRow(ctx context.Context, row int) {
	if row < 1 {
		return
	}
	rows := c.state.GetAllRows()
	dataRow := row - 1
	if dataRow < 0 || dataRow >= len(rows) {
		return
	}
	c.runEditorStatement(ctx, "DUPLICATE", c.buildDuplicateInsertSQL(rows[dataRow]), "Insert error")
}

// handleEditRow opens the configured editor pre-filled with an UPDATE SQL template
// for the current row. The edited statement is executed on save.
// On execution error the user can choose Fix (reopen editor with their SQL) or Cancel.
func (c *Data) handleEditRow(ctx context.Context, row int) *tcell.EventKey {
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

	c.runEditorStatement(ctx, "EDIT", c.buildUpdateSQL(rows[dataRow], pk), "Update error")
	return nil
}

func (c *Data) buildUpdateSQL(row database.Row, pk *database.PrimaryKey) string {
	return database.BuildUpdateSQL(
		c.state.Schema, c.state.Table,
		c.orderedColumnNames(row), c.state.GetPrimaryKey(),
		c.App.GetFormatter().SQLLiteral, row, *pk,
	)
}

func (c *Data) handleExportData(ctx context.Context) *tcell.EventKey {
	if c.state.Table == "" && c.state.RawSQL == "" {
		return nil
	}
	c.exportModal.Render(ctx, c.buildExportQuery(), c.state.Schema, c.state.Table)
	return nil
}

func (c *Data) buildExportQuery() string {
	if c.state.RawSQL != "" {
		return c.state.RawSQL
	}
	q := fmt.Sprintf(`SELECT * FROM "%s"."%s"`, c.state.Schema, c.state.Table)
	if c.state.Where != "" {
		q += " WHERE " + c.state.Where
	}
	if c.state.OrderBy != "" {
		q += " ORDER BY " + c.state.OrderBy
	}
	return q
}

// OpenExport opens the export dialog for the current table or query.
func (c *Data) OpenExport(ctx context.Context) {
	if c.state.Table == "" && c.state.RawSQL == "" {
		return
	}
	c.exportModal.Render(ctx, c.buildExportQuery(), c.state.Schema, c.state.Table)
}

// OpenExplain runs EXPLAIN on the last executed query and shows the viewer.
func (c *Data) OpenExplain(ctx context.Context) {
	if c.state.LastQuery != "" {
		go c.runExplain(ctx, c.state.LastQuery)
	}
}

// OpenHistory opens the SQL history modal.
func (c *Data) OpenHistory() {
	c.sqlQueryEditor.OpenHistory()
}

// OpenHistoryWithCallback opens the SQL history modal and temporarily
// overrides the onAccept callback. The original callback is restored on close.
func (c *Data) OpenHistoryWithCallback(onAccept func(query string)) {
	c.sqlQueryEditor.OpenHistoryWithCallback(onAccept)
}

// isSelectQuery returns true when sql is a statement that returns rows.
func isSelectQuery(sql string) bool {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(upper, "SELECT") ||
		strings.HasPrefix(upper, "WITH") ||
		strings.HasPrefix(upper, "EXPLAIN") ||
		strings.HasPrefix(upper, "TABLE") ||
		database.IsReturningDML(sql)
}
