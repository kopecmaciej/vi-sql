package component

import (
	"context"
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
	sqlpkg "github.com/kopecmaciej/vi-sql/internal/sql"
	"github.com/kopecmaciej/vi-sql/internal/sql/completion"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
	"github.com/kopecmaciej/vi-sql/internal/tui/widget"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog/log"
)

const (
	DataId = "Data"

	FilterBarSuffix   = "-filter"
	OrderBarSuffix    = "-order"
	ResultsSuffix     = "-results"
	DeleteModalSuffix = "-delete"
	EditorSuffix      = "-editor"

	editorSizeNormal     = 0
	editorSizeFullscreen = 1
)

// TabMode controls which keybindings and features are active.
type TabMode int

const (
	TableMode TabMode = iota // pre-filled SELECT, no editor, full CRUD keys
	QueryMode                // blank editor, no CRUD keys
	ViewMode                 // read-only (no PK, no CRUD)
)

// dataTabCounter generates unique identifiers for each Data/QueryTab instance
// so that multiple tabs can subscribe to the event system without colliding.
var dataTabCounter int32

func nextDataID() string {
	n := atomic.AddInt32(&dataTabCounter, 1)
	return fmt.Sprintf("QueryTab-%d", n)
}

// Data displays table rows in a grid with scroll-buffered fetching, filtering,
// ordering, column hide/show, and row CRUD.
type Data struct {
	*core.BaseElement
	*core.Flex

	mode           TabMode
	tableFlex      *core.Flex
	resultsBar     *widget.ResultsBar
	resultGrid     *ResultGrid
	style          *config.DataStyle
	filterBar      *InputBar
	orderBar       *InputBar
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
	runner         *QueryRunner
	scroll         *scrollFetcher
}

func newData(mode TabMode) *Data {
	id := tview.Identifier(nextDataID())
	c := &Data{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),

		mode:           mode,
		tableFlex:      core.NewFlex(),
		resultsBar:     widget.NewResultsBar(),
		resultGrid:     NewResultGrid(),
		filterBar:      NewInputBar(id+FilterBarSuffix, "WHERE"),
		orderBar:       NewInputBar(id+OrderBarSuffix, "ORDER BY"),
		termEditor:     NewTermEditor(),
		sqlQueryEditor: NewSQLQueryEditor(string(id)),
		editorSize:     editorSizeNormal,
		inlineEdit:     modal.NewInlineEditModal(),
		confirmModal:   modal.NewConfirm(id + DeleteModalSuffix),
		exportModal:    modal.NewExportModal(),
		sqlEditModal:   NewSQLEditModal(),
		peeker:         NewPeeker(),
		explainViewer:  NewExplainViewer(),
		state:          &database.TableState{},
		stateMap:       database.NewStateMap(),
	}

	c.SetIdentifier(id)
	if mode == QueryMode {
		c.resultGrid.SetIdentifier(id + ResultsSuffix)
	} else {
		c.resultGrid.SetIdentifier(id)
	}
	c.SetAfterInitFunc(c.init)

	return c
}

func NewQueryMode() *Data {
	return newData(QueryMode)
}

func NewTableTab() *Data {
	return newData(TableMode)
}

func NewViewTab() *Data {
	return newData(ViewMode)
}

func (c *Data) init() error {
	ctx := context.Background()

	c.resultGrid.SetApp(c.App)
	c.setLayout()
	c.setStyle()
	c.setKeybindings(ctx)

	if err := c.termEditor.Init(c.App); err != nil {
		return err
	}
	if err := c.sqlQueryEditor.Init(c.App); err != nil {
		return err
	}
	c.sqlQueryEditor.SetColumnFetcher(func(schema, table string) ([]completion.Column, error) {
		infos, err := c.Driver.GetTableColumns(ctx, schema, table)
		if err != nil {
			return nil, err
		}
		fks, err := c.Driver.GetTableForeignKeys(ctx, schema, table)
		if err != nil {
			log.Error().Err(err).Str("schema", schema).Str("table", table).
				Msg("failed to load foreign keys for autocomplete")
		}
		fkSet := fkColumnSet(fks)
		cols := make([]completion.Column, len(infos))
		for i, info := range infos {
			cols[i] = completion.Column{Name: info.Name, TypeHint: info.DataType, IsPK: info.IsPK, IsFK: fkSet[info.Name]}
		}
		return cols, nil
	})
	c.sqlQueryEditor.SetOnFullscreen(func() {
		c.toggleFullscreen()
	})
	c.sqlQueryEditor.SetOnFocusDown(func() {
		c.App.SetFocusOnly(c.resultGrid)
	})
	c.sqlQueryEditor.SetOnOpenInEditor(func() {
		if c.App.GetConfig().Editor.Enabled {
			c.handleTermEditorForQuery()
		}
	})
	c.runner = NewQueryRunner(
		c.Driver,
		func(f func()) { go c.App.QueueUpdateDraw(f) },
		c.sqlQueryEditor.SaveQueryToHistory,
		func(result manager.QueryResult) {
			c.App.GetManager().Broadcast(manager.NewQueryExecutedMsg(result))
		},
	)
	c.scroll = newScrollFetcher(c.runner, c.state)
	c.sqlQueryEditor.SetOnCancel(func() {
		c.runner.CancelQuery()
	})
	c.sqlQueryEditor.SetOnExecute(func(sql string) {
		if c.editorSize == editorSizeFullscreen {
			c.toggleFullscreen()
		}
		// For statements, show a confirm modal first; if shown the modal's
		// OnConfirm calls runner.Execute itself so we return early here.
		if !isSelectQuery(sql) && !isExplainQuery(sql) {
			if c.confirmIfDestructive(sql, func() {
				c.runner.Execute(sql, 0, c.statementCallbacks(sql))
			}) {
				return
			}
		}
		// Pre-create state so OnCount can reference it while the query runs.
		sqlState := database.NewTableState("", "")
		sqlState.RawSQL = sql
		sqlState.BatchSize = c.batchSize()
		c.scroll.cancel()
		c.runner.Execute(sql, sqlState.BatchSize, RunCallbacks{
			OnRunning: func() { c.resultsBar.RenderRunning() },
			OnSelect: func(rows []database.Row, cols []database.ColumnInfo, query string, execTime time.Duration) {
				sqlState.LastQuery = query
				if val, ok := database.ExtractLimitValue(sql); ok {
					sqlState.UserLimit = val
				}
				sqlState.PopulateRows(rows)
				c.state = sqlState
				c.scroll.updateState(c.state)
				c.columns = cols
				c.lastExecTime = execTime
				c.resultGrid.Clear()
				c.resultsBar.Render(c.state, c.lastExecTime)
				if len(rows) == 0 {
					c.resultGrid.SetFixed(0, 0)
					c.resultGrid.SetSelectable(false, false)
					c.resultGrid.SetCell(0, 0, tview.NewTableCell("No rows returned"))
					return
				}
				c.resultGrid.Render(rows, c.columns, c.App.GetStyles())
			},
			OnStatement: c.statementCallbacks(sql).OnStatement,
			OnExplain: func(result, bareSql string, analyze bool) {
				c.showExplainViewer(ctx, bareSql, result, analyze)
			},
			OnError: func(err error) {
				modal.ShowErrorWithDone(c.App.Pages, "Query error", err, func() {
					c.resultsBar.RestorePrevious()
				})
			},
			OnCancel: func() { c.resultsBar.RenderCancelled() },
		})
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
	if err := c.orderBar.Init(c.App); err != nil {
		return err
	}

	c.filterBar.EnableColumnAutocomplete(sqlpkg.OperatorKeywords)
	c.orderBar.EnableColumnAutocomplete(sqlpkg.OrderKeywords)

	c.filterBarHandler()
	c.orderBarHandler()

	c.handleEvents()

	return nil
}

func (c *Data) handleEvents() {
	go c.HandleEvents(c.GetIdentifier(), func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			go c.App.QueueUpdateDraw(func() {
				c.setStyle()
				c.reRenderState()
			})
		}
	})
}

func (c *Data) setStyle() {
	c.style = &c.App.GetStyles().Data
	styles := c.App.GetStyles()
	sqlEditorStyle := &styles.SQLEditor
	c.filterBar.EnableHighlighting(sqlEditorStyle)
	c.orderBar.EnableHighlighting(sqlEditorStyle)

	c.tableFlex.SetStyle(styles)
	c.resultsBar.SetStyle(styles)
	c.Flex.SetStyle(styles)
	c.resultGrid.SetStyle(styles, c.style)
}

func (c *Data) setLayout() {
	c.tableFlex.SetBorder(true)
	c.tableFlex.SetDirection(tview.FlexRow)
	title := " Table "
	switch c.mode {
	case QueryMode:
		title = " Query "
	case ViewMode:
		title = " View "
	}
	c.tableFlex.SetTitle(title)
	c.tableFlex.SetTitleAlign(tview.AlignCenter)
	c.tableFlex.SetBorderPadding(0, 0, 1, 1)

	c.Flex.SetDirection(tview.FlexRow)
}

func (c *Data) setKeybindings(ctx context.Context) {
	k := c.App.GetKeys()

	c.resultGrid.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, col := c.resultGrid.GetSelection()
		switch {
		case k.Match(k.Navigation.GoTop, event):
			if c.resultGrid.GetRowCount() > 1 {
				c.resultGrid.Select(1, col)
			}
			return nil
		case k.Match(k.Navigation.GoBottom, event):
			if rc := c.resultGrid.GetRowCount(); rc > 1 {
				c.resultGrid.Select(rc-1, col)
				_, _, _, viewHeight := c.resultGrid.GetInnerRect()
				c.scroll.tryPrefetch(rc-1, viewHeight, c.renderAfterAppend)
			}
			return nil
		case k.Match(k.Data.PeekRow, event):
			return c.handlePeekRow(row, false)
		case k.Match(k.Data.FullPagePeek, event):
			return c.handlePeekRow(row, true)
		case k.Match(k.Data.CopyCell, event):
			c.resultGrid.CopyCell(row, col, c.state.GetAllRows())
			return nil
		case k.Match(k.Data.CopyRow, event):
			c.resultGrid.CopyRow(row, c.state.GetAllRows(), c.columns)
			return nil
		case k.Match(k.Data.CopyRowJSON, event):
			c.resultGrid.CopyRowAs(util.ExportJSON, row, c.state.GetAllRows(), c.columns)
			return nil
		case k.Match(k.Data.CopyRowCSV, event):
			c.resultGrid.CopyRowAs(util.ExportCSV, row, c.state.GetAllRows(), c.columns)
			return nil
		case k.Match(k.Common.Refresh, event):
			c.triggerRefresh(nil, func(err error) {
				modal.ShowError(c.App.Pages, "Error refreshing rows", err)
			})
			return nil
		case k.Match(k.Navigation.FocusUp, event):
			if c.mode == QueryMode {
				c.App.SetFocusOnly(c.sqlQueryEditor)
				return nil
			}
		case k.Match(k.Data.HideColumn, event):
			return c.handleHideColumn(col)
		case k.Match(k.Data.ResetHiddenColumns, event):
			c.resultGrid.ResetHiddenColumns()
			c.reRenderState()
			return nil
		case k.Match(k.Data.MultipleSelect, event):
			c.resultGrid.ToggleVisualMode()
			return nil
		case k.Match(k.Data.ClearSelection, event):
			if c.runner.IsQueryRunning() {
				c.runner.CancelQuery()
				return nil
			}
			c.resultGrid.ClearSelection()
			return nil
		case k.Match(k.Data.ExplainQuery, event):
			if c.state.LastQuery != "" {
				c.runner.Execute("EXPLAIN "+c.state.LastQuery, 0, RunCallbacks{
					OnExplain: func(result, bareSql string, analyze bool) {
						c.showExplainViewer(ctx, bareSql, result, analyze)
					},
					OnError:  func(err error) { modal.ShowError(c.App.Pages, "Explain error", err) },
					OnCancel: func() { c.resultsBar.RenderCancelled() },
				})
			}
			return nil
		case k.Match(k.Data.ExportData, event):
			return c.handleExportData(ctx)
		case k.Match(k.Data.FollowForeignKey, event):
			return c.handleFollowForeignKey(row, col)
		case k.Match(k.Data.FindReferences, event):
			return c.handleFindReferences(ctx, row, col)
		case k.Match(k.Data.OrderByColumn, event):
			return c.handleOrderByColumn(col)
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
				c.filterBar.Toggle(c.state.Where)
				c.Render()
				return nil
			case k.Match(k.Data.ToggleOrderBar, event):
				c.orderBar.Toggle(c.state.OrderBy)
				c.Render()
				return nil
			}
		}

		_, _, _, viewHeight := c.resultGrid.GetInnerRect()
		c.scroll.tryPrefetch(row, viewHeight, c.renderAfterAppend)

		return event
	}))
}

// TabOptions carries optional initial state for a new table tab.
type TabOptions struct {
	Where       string
	FocusColumn string // column name to select after loading (empty = default col 0)
}

func (c *Data) HandleTableSelection(ctx context.Context, schema, table string, opts ...TabOptions) error {
	c.filterBar.SetText("")
	c.orderBar.SetText("")
	c.resultGrid.ResetHiddenColumns()

	state, ok := c.stateMap.Get(c.stateMap.Key(schema, table))
	if ok {
		c.state = state
	} else {
		c.state = database.NewTableState(schema, table)
		c.state.BatchSize = c.batchSize()

		if len(opts) > 0 && opts[0].Where != "" {
			c.state.Where = opts[0].Where
		}
	}
	c.scroll.updateState(c.state)

	columns, err := c.Driver.GetTableColumns(ctx, schema, table)
	if err != nil {
		log.Error().Err(err).Str("schema", schema).Str("table", table).
			Msg("failed to load table columns")
	} else {
		c.columns = columns
		var pkCols []string
		for _, col := range columns {
			if col.IsPK {
				pkCols = append(pkCols, col.Name)
			}
		}
		c.state.SetPrimaryKey(pkCols)
	}

	var fkErr error
	c.foreignKeys, fkErr = c.Driver.GetTableForeignKeys(ctx, schema, table)
	if fkErr != nil {
		log.Error().Err(fkErr).Str("schema", schema).Str("table", table).
			Msg("failed to load foreign keys")
	}
	fkSet := fkColumnSet(c.foreignKeys)
	for i := range c.columns {
		c.columns[i].IsFK = fkSet[c.columns[i].Name]
	}

	focusCol := ""
	if len(opts) > 0 {
		focusCol = opts[0].FocusColumn
	}
	c.triggerRefresh(
		func() {
			if focusCol != "" {
				c.selectColumn(focusCol)
			}
			c.App.SetFocus(c.resultGrid)
		},
		func(err error) {
			modal.ShowError(c.App.Pages, "Error loading table", err)
		},
	)
	return nil
}

// selectColumn moves the table cursor to the first data row of the named column.
func (c *Data) selectColumn(colName string) {
	for col := 0; col < c.resultGrid.GetColumnCount(); col++ {
		cell := c.resultGrid.GetCell(0, col)
		if cell == nil {
			continue
		}
		if ref, ok := cell.GetReference().(string); ok && ref == colName {
			c.resultGrid.Select(1, col)
			return
		}
	}
}

func (c *Data) Reset() {
	c.resultGrid.Clear()
	c.resultsBar.Clear()
	c.state = &database.TableState{}
	c.stateMap = database.NewStateMap()
	c.columns = nil
}

func (c *Data) SetSchemasForAutocomplete(schemas []database.Schema) {
	c.filterBar.SetSchemas(schemas)
	c.orderBar.SetSchemas(schemas)
	c.sqlQueryEditor.SetSchemas(schemas)
	c.sqlEditModal.SetSchemas(schemas)
}

// IsQueryRunning reports whether a query is currently in flight.
func (c *Data) IsQueryRunning() bool { return c.runner != nil && c.runner.IsQueryRunning() }

// Refresh discards the current buffer and re-fetches from offset 0.
func (c *Data) Refresh() {
	c.triggerRefresh(nil, func(err error) {
		modal.ShowError(c.App.Pages, "Error refreshing rows", err)
	})
}

// IsQueryTab reports whether this tab is in query mode (not a table tab).
func (c *Data) IsQueryTab() bool {
	return c.mode == QueryMode
}

// IsViewTab reports whether this tab is browsing a database view (read-only).
func (c *Data) IsViewTab() bool {
	return c.mode == ViewMode
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

// CopyRowAs copies the current row (or multi-selection) to the clipboard in
// the given export format. Delegates to ResultGrid with the current selection.
func (c *Data) CopyRowAs(format util.ExportFormat) {
	row, _ := c.resultGrid.GetSelection()
	c.resultGrid.CopyRowAs(format, row, c.state.GetAllRows(), c.columns)
}

// SelectedTable returns the schema and table/view currently loaded in this tab.
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

func (c *Data) EnterNormalMode() { c.sqlQueryEditor.EnterNormalMode() }

// GetFocusPrimitive returns the inner primitive that should receive focus
// when this tab is activated from outside (e.g. tab switching).
func (c *Data) GetFocusPrimitive() tview.Primitive {
	if c.mode == QueryMode {
		return c.sqlQueryEditor
	}
	return c.resultGrid
}

func (c *Data) Render() {
	c.Flex.Clear()
	c.tableFlex.Clear()

	c.tableFlex.AddItem(c.resultsBar, 2, 0, false)
	c.tableFlex.AddItem(c.resultGrid, 0, 1, true)

	var focusPrimitive tview.Primitive

	if c.mode == QueryMode {
		focusPrimitive = c.sqlQueryEditor
		switch c.editorSize {
		case editorSizeFullscreen:
			c.Flex.AddItem(c.sqlQueryEditor, 0, 1, true)
		default:
			c.Flex.AddItem(c.sqlQueryEditor, 0, 3, true)
			c.Flex.AddItem(c.tableFlex, 0, 7, true)
		}
	} else {
		// TableMode: filter/order bars sit above the table
		focusPrimitive = c.resultGrid
		if c.filterBar.IsEnabled() {
			c.Flex.AddItem(c.filterBar, 3, 0, false)
			focusPrimitive = c.filterBar
		}
		if c.orderBar.IsEnabled() {
			c.Flex.AddItem(c.orderBar, 3, 0, false)
			focusPrimitive = c.orderBar
		}
		c.Flex.AddItem(c.tableFlex, 0, 1, true)
	}

	c.App.SetFocus(focusPrimitive)
}

// fkColumnSet returns a set of column names that participate in any FK in fks.
func fkColumnSet(fks []database.ForeignKeyInfo) map[string]bool {
	set := make(map[string]bool, len(fks))
	for _, fk := range fks {
		for _, col := range fk.Columns {
			set[col] = true
		}
	}
	return set
}

func (c *Data) loadAutocompleteKeys(ctx context.Context) {
	colNames := make([]string, len(c.columns))
	completionCols := make([]completion.Column, len(c.columns))
	for i, col := range c.columns {
		colNames[i] = col.Name
		completionCols[i] = completion.Column{Name: col.Name, TypeHint: col.DataType, IsPK: col.IsPK, IsFK: col.IsFK}
	}
	c.filterBar.LoadAutocompleteColumns(completionCols)
	c.orderBar.LoadAutocompleteColumns(completionCols)
	c.sqlQueryEditor.SetColumnsForTable(c.state.Schema, c.state.Table, completionCols)

	msg := manager.NewUpdateAutocompleteKeysMsg(colNames)
	msg.Sender = c.GetIdentifier()
	c.App.GetManager().Broadcast(msg)

	schemas, err := c.Driver.ListSchemas(ctx, "")
	if err != nil {
		return
	}
	c.filterBar.SetSchemas(schemas)
	c.orderBar.SetSchemas(schemas)
	c.sqlQueryEditor.SetSchemas(schemas)
}

func (c *Data) filterBarHandler() {
	acceptFunc := func(text string) {
		if c.state.RawSQL != "" {
			c.state.RawSQL = database.RebuildSelectSQL(c.state.RawSQL, text, c.state.OrderBy)
		}
		c.state.SetWhere(text)
		c.triggerRefresh(
			func() {
				c.filterBar.Disable()
				c.Flex.RemoveItem(c.filterBar)
				c.App.SetFocus(c.resultGrid)
			},
			func(err error) {
				c.state.SetWhere("")
				modal.ShowError(c.App.Pages, "Error applying WHERE filter", err)
			},
		)
	}
	rejectFunc := func() {
		c.Flex.RemoveItem(c.filterBar)
		c.App.SetFocus(c.resultGrid)
	}
	c.filterBar.DoneFuncHandler(acceptFunc, rejectFunc)
}

func (c *Data) orderBarHandler() {
	acceptFunc := func(text string) {
		if c.state.RawSQL != "" {
			c.state.RawSQL = database.RebuildSelectSQL(c.state.RawSQL, c.state.Where, text)
		}
		c.state.SetOrderBy(text)
		c.triggerRefresh(
			func() {
				c.orderBar.Disable()
				c.Flex.RemoveItem(c.orderBar)
				c.App.SetFocus(c.resultGrid)
			},
			func(err error) {
				c.state.SetOrderBy("")
				modal.ShowError(c.App.Pages, "Error applying ORDER BY", err)
			},
		)
	}
	rejectFunc := func() {
		c.Flex.RemoveItem(c.orderBar)
		c.App.SetFocus(c.resultGrid)
	}
	c.orderBar.DoneFuncHandler(acceptFunc, rejectFunc)
}

func (c *Data) handlePeekRow(row int, fullScreen bool) *tcell.EventKey {
	if row < 1 {
		return nil
	}
	rowData := c.resultGrid.RowData(row, c.state.GetAllRows())
	if rowData == nil {
		return nil
	}
	c.peeker.ViewModal.SetFullScreen(fullScreen)
	c.peeker.Render(rowData, c.columns)
	return nil
}

func (c *Data) handleDeleteRow(ctx context.Context, row, col int) *tcell.EventKey {
	if row < 1 {
		return nil
	}

	// Collect primary keys: prefer multi-selected rows, fall back to cursor row.
	selectedRows := c.resultGrid.GetSelectedRows()
	allRows := c.state.GetAllRows()
	pkCols := c.state.GetPrimaryKey()
	var pks []database.PrimaryKey
	if len(selectedRows) > 0 {
		for _, r := range selectedRows {
			if pk := c.resultGrid.RowPrimaryKey(r, allRows, pkCols); pk != nil {
				pks = append(pks, *pk)
			}
		}
	} else {
		pk := c.resultGrid.RowPrimaryKey(row, allRows, pkCols)
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
		c.reRenderState()
		if row >= c.resultGrid.GetRowCount() {
			c.resultGrid.Select(row-1, col)
		} else {
			c.resultGrid.Select(row, col)
		}
	})
	c.App.Pages.AddPage(c.confirmModal.GetIdentifier(), c.confirmModal, true, true)
	return nil
}

func (c *Data) rowPrimaryKey(row int) *database.PrimaryKey {
	return c.resultGrid.RowPrimaryKey(row, c.state.GetAllRows(), c.state.GetPrimaryKey())
}

// statementCallbacks returns RunCallbacks for a non-SELECT statement result.
// Used for both the direct execute path and the confirm-then-execute path.
func (c *Data) statementCallbacks(sql string) RunCallbacks {
	return RunCallbacks{
		OnRunning: func() { c.resultsBar.RenderRunning() },
		OnStatement: func(affected int64, execTime time.Duration) {
			c.state.RawSQL = sql
			c.showStatementResult(affected, execTime)
		},
		OnError: func(err error) {
			modal.ShowErrorWithDone(c.App.Pages, "Statement error", err, func() {
				c.resultsBar.RestorePrevious()
			})
		},
		OnCancel: func() { c.resultsBar.RenderCancelled() },
	}
}

// triggerRefresh discards the current buffer and fetches from offset 0.
// onDone is called on the tview goroutine after the table re-renders.
// onErr is called on the tview goroutine if the query fails.
func (c *Data) triggerRefresh(onDone func(), onErr func(error)) {
	c.resultGrid.ClearSelection()
	c.state.ClearBuffer()
	c.scroll.cancel()
	c.runner.Refresh(c.state, RunCallbacks{
		OnRunning: func() { c.resultsBar.RenderRunning() },
		OnCountEstimate: func(count int64) {
			c.state.Count = count
			c.state.CountIsEstimate = true
			c.resultsBar.Render(c.state, c.lastExecTime)
		},
		OnCount: func(count int64) {
			c.state.Count = count
			c.state.CountIsEstimate = false
			c.resultsBar.Render(c.state, c.lastExecTime)
		},
		OnSelect: func(rows []database.Row, cols []database.ColumnInfo, query string, execTime time.Duration) {
			c.state.LastQuery = query
			c.lastExecTime = execTime
			if cols != nil {
				c.columns = cols
			}
			if c.state.RawSQL != "" {
				if val, ok := database.ExtractLimitValue(c.state.RawSQL); ok {
					c.state.UserLimit = val
				}
			}
			c.state.PopulateRows(rows)
			if c.state.RawSQL == "" {
				go c.loadAutocompleteKeys(context.Background())
			}
			c.resultGrid.Clear()
			c.resultsBar.Render(c.state, c.lastExecTime)
			c.stateMap.Set(c.stateMap.Key(c.state.Schema, c.state.Table), c.state)
			if len(rows) == 0 {
				c.resultGrid.SetCell(0, 0, tview.NewTableCell("No rows found"))
			} else {
				c.resultGrid.Render(rows, c.columns, c.App.GetStyles())
			}
			if onDone != nil {
				onDone()
			}
		},
		OnError: func(err error) {
			if onErr != nil {
				onErr(err)
			}
		},
		OnCancel: func() { c.resultsBar.RenderCancelled() },
	})
}

// reRenderState re-renders the table from the current in-memory state without
// hitting the database. Used after local mutations (delete, hide column, etc.).
func (c *Data) reRenderState() {
	c.resultGrid.ClearSelection()
	rows := c.state.GetAllRows()
	c.resultGrid.Clear()
	c.resultsBar.Render(c.state, c.lastExecTime)
	c.stateMap.Set(c.stateMap.Key(c.state.Schema, c.state.Table), c.state)
	if len(rows) == 0 {
		c.resultGrid.SetCell(0, 0, tview.NewTableCell("No rows found"))
		return
	}
	c.resultGrid.Render(rows, c.columns, c.App.GetStyles())
}

// RunCount fires an exact COUNT(*) for the current table/query and updates the results bar.
func (c *Data) RunCount() {
	sql := c.buildCountSQL()
	if sql == "" {
		return
	}
	c.runner.RunCount(sql, RunCallbacks{
		OnRunning: func() { c.resultsBar.RenderRunning() },
		OnCount: func(count int64) {
			c.state.Count = count
			c.state.CountIsEstimate = false
			c.resultsBar.Render(c.state, c.lastExecTime)
		},
		OnError:  func(err error) { modal.ShowError(c.App.Pages, "Count error", err) },
		OnCancel: func() { c.resultsBar.RenderCancelled() },
	})
}

func (c *Data) buildCountSQL() string {
	if c.state.RawSQL != "" {
		return fmt.Sprintf("SELECT count(*) FROM (%s) AS _q", c.state.RawSQL)
	}
	if c.state.Table == "" {
		return ""
	}
	q := fmt.Sprintf(`SELECT count(*) FROM "%s"."%s"`, c.state.Schema, c.state.Table)
	if c.state.Where != "" {
		q += " WHERE " + c.state.Where
	}
	return q
}

// batchSize returns the DB fetch granularity: Options.FetchLimit when set, otherwise 100.
func (c *Data) batchSize() int64 {
	conn := c.App.GetConfig().GetCurrentConnection()
	if conn != nil && conn.Options.FetchLimit != nil {
		return *conn.Options.FetchLimit
	}
	return 100
}

// renderAfterAppend re-renders the full buffer after a scroll prefetch appends rows.
// Preserves the current cursor position.
func (c *Data) renderAfterAppend() {
	row, col := c.resultGrid.GetSelection()
	rows := c.state.GetAllRows()
	c.resultGrid.Clear()
	c.resultsBar.Render(c.state, c.lastExecTime)
	c.stateMap.Set(c.stateMap.Key(c.state.Schema, c.state.Table), c.state)
	c.resultGrid.Render(rows, c.columns, c.App.GetStyles())
	if row > 0 && row < c.resultGrid.GetRowCount() {
		c.resultGrid.Select(row, col)
	}
}

// confirmIfDestructive shows a confirm modal for destructive SQL statements.
// If the statement is destructive, it shows the modal and wires proceed as the
// confirm action, then returns true. Returns false if no confirmation is needed.
func (c *Data) confirmIfDestructive(sql string, proceed func()) bool {
	conn := c.App.GetConfig().GetCurrentConnection()
	if conn != nil {
		opts := conn.GetOptions()
		if opts.AlwaysConfirmActions != nil && !*opts.AlwaysConfirmActions {
			return false
		}
	}

	info := sqlpkg.HasDestructiveStatement(sql)
	if info == nil {
		return false
	}

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
		proceed()
	})
	c.App.Pages.AddPage(c.confirmModal.GetIdentifier(), c.confirmModal, true, true)
	c.App.SetFocusOnly(c.confirmModal)
	return true
}

func (c *Data) handleOrderByColumn(col int) *tcell.EventKey {
	columnName := c.resultGrid.ColumnName(col)
	if columnName == "" {
		return nil
	}
	quotedName := c.App.GetQuoter().Ident(columnName)
	currentOrder := c.state.OrderBy

	var newOrder string
	if currentOrder == quotedName+" ASC" {
		newOrder = quotedName + " DESC"
	} else {
		newOrder = quotedName + " ASC"
	}

	if c.mode == QueryMode && c.state.RawSQL != "" {
		c.state.RawSQL = database.RebuildSelectSQL(c.state.RawSQL, c.state.Where, newOrder)
	}
	c.state.SetOrderBy(newOrder)
	c.triggerRefresh(func() {
		c.resultGrid.Select(1, col)
	}, func(err error) {
		modal.ShowError(c.App.Pages, "Error ordering rows", err)
	})
	return nil
}

func (c *Data) handleHideColumn(col int) *tcell.EventKey {
	if c.resultGrid.HideColumn(col) == "" {
		return nil
	}
	row, _ := c.resultGrid.GetSelection()
	c.reRenderState()
	c.resultGrid.Select(row, c.resultGrid.ClampCol(col))
	return nil
}

// handleFollowForeignKey opens a new table tab for the referenced table, pre-filtered
// to the row that the current cell's FK value points to.
func (c *Data) handleFollowForeignKey(row, col int) *tcell.EventKey {
	if row < 1 || len(c.foreignKeys) == 0 {
		return nil
	}

	colName := c.resultGrid.ColumnName(col)
	if colName == "" {
		return nil
	}

	var fk *database.ForeignKeyInfo
	for i := range c.foreignKeys {
		if slices.Contains(c.foreignKeys[i].Columns, colName) {
			fk = &c.foreignKeys[i]
			break
		}
	}
	if fk == nil {
		return nil
	}

	rowData := c.resultGrid.RowData(row, c.state.GetAllRows())
	if rowData == nil {
		return nil
	}

	driverName := c.App.GetConfig().GetCurrentConnection().GetDriver()
	def, ok := database.GetConnector(driverName)
	if !ok {
		log.Warn().Str("driver", driverName).Msg("FollowForeignKey: connector not registered")
		return nil
	}
	lit := c.App.GetFormatter().SQLLiteral
	// Composite FKs reference multiple columns, so build one clause per column.
	var whereParts []string
	for i, fkColName := range fk.Columns {
		val := rowData[fkColName]
		if val == nil {
			return nil
		}
		whereParts = append(whereParts, fmt.Sprintf(`%s = %s`, def.Quoter.Ident(fk.ReferencedCols[i]), lit(val)))
	}

	c.App.GetManager().Broadcast(manager.NewOpenTableTabMsg(manager.TableTabRequest{
		Schema: fk.ReferencedSchema,
		Table:  fk.ReferencedTable,
		Where:  strings.Join(whereParts, " AND "),
	}))

	return nil
}

// handleFindReferences finds all tables that have foreign keys pointing to the current
// cell's column and opens a tab pre-filtered to matching rows. If multiple tables
// reference the column a list is shown so the user can choose which one to open.
func (c *Data) handleFindReferences(ctx context.Context, row, col int) *tcell.EventKey {
	if c.mode != TableMode || row < 1 {
		return nil
	}

	colName := c.resultGrid.ColumnName(col)
	if colName == "" {
		return nil
	}

	rowData := c.resultGrid.RowData(row, c.state.GetAllRows())
	if rowData == nil {
		return nil
	}
	cellValue := rowData[colName]
	if cellValue == nil {
		return nil
	}

	incoming, err := c.App.GetDriver().GetIncomingForeignKeys(ctx, c.state.Schema, c.state.Table)
	if err != nil || len(incoming) == 0 {
		modal.ShowInfo(c.App.Pages, "No references found for "+c.state.Table+"."+colName)
		return nil
	}

	// Keep only entries that reference our column.
	var relevant []database.IncomingForeignKeyInfo
	for _, fk := range incoming {
		if slices.Contains(fk.ReferencedCols, colName) {
			relevant = append(relevant, fk)
		}
	}
	if len(relevant) == 0 {
		modal.ShowInfo(c.App.Pages, "No references found for "+c.state.Table+"."+colName)
		return nil
	}

	driverName := c.App.GetConfig().GetCurrentConnection().GetDriver()
	def, ok := database.GetConnector(driverName)
	if !ok {
		log.Warn().Str("driver", driverName).Msg("FindReferences: connector not registered")
		return nil
	}
	lit := c.App.GetFormatter().SQLLiteral

	// A single FK references each column at most once, so finding the match
	// yields exactly one fkCol → one WHERE clause.
	openRef := func(fk database.IncomingForeignKeyInfo) {
		var fkCol string
		for i, refCol := range fk.ReferencedCols {
			if refCol == colName {
				fkCol = fk.Columns[i]
				break
			}
		}
		if fkCol == "" {
			return
		}
		c.App.GetManager().Broadcast(manager.NewOpenTableTabMsg(manager.TableTabRequest{
			Schema:      fk.Schema,
			Table:       fk.Table,
			Where:       fmt.Sprintf(`%s = %s`, def.Quoter.Ident(fkCol), lit(cellValue)),
			FocusColumn: fkCol,
		}))
	}

	if len(relevant) == 1 {
		openRef(relevant[0])
		return nil
	}

	// Multiple referencing tables: show a list so the user can pick one.
	refsPageID := tview.Identifier(string(c.GetIdentifier()) + "-refs")
	list := core.NewList()
	list.SetBorder(true)
	list.SetTitle(" Referenced by ")
	list.ShowSecondaryText(false)

	closeModal := func() {
		c.App.Pages.RemovePage(refsPageID)
	}

	for _, fk := range relevant {
		fkCopy := fk
		label := fk.Schema + "." + fk.Table
		list.AddItem(label, "", 0, func() {
			closeModal()
			openRef(fkCopy)
		})
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			closeModal()
			return nil
		}
		return event
	})

	c.App.Pages.AddPage(refsPageID, core.CenteredFlex(list, 2, 2), true, true)
	c.App.SetFocusOnly(list)

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
					c.App.QueueUpdateDraw(func() {
						c.triggerRefresh(nil, func(err error) {
							modal.ShowError(c.App.Pages, "Error refreshing rows", err)
						})
					})
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
		c.triggerRefresh(nil, func(err error) {
			modal.ShowError(c.App.Pages, "Error refreshing rows", err)
		})
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
	c.resultGrid.Clear()
	c.resultGrid.SetFixed(0, 0)
	c.resultGrid.SetSelectable(false, false)
	c.resultsBar.RenderStatementResult(affected, execTime)
	c.resultGrid.SetCell(0, 0, tview.NewTableCell(
		fmt.Sprintf("%d rows affected", affected)))
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

func (c *Data) handleInlineEdit(ctx context.Context, row, col int) *tcell.EventKey {
	if row < 1 {
		return nil
	}

	pk := c.rowPrimaryKey(row)
	if pk == nil {
		return nil
	}

	colName := c.resultGrid.ColumnName(col)
	if colName == "" {
		return nil
	}

	originalRow := c.resultGrid.RowData(row, c.state.GetAllRows())
	if originalRow == nil {
		return nil
	}
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
		c.App.SetFocus(c.resultGrid)
		c.reRenderState()
		c.resultGrid.Select(row, col)
		return nil
	})

	c.inlineEdit.SetCancelCallback(func() {
		c.inlineEdit.Hide()
		c.App.SetFocus(c.resultGrid)
		c.resultGrid.Select(row, col)
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
	c.runEditorStatement(ctx, "Add Row", c.buildInsertSQL(), "Insert error")
}

func (c *Data) buildInsertSQL() string {
	return sqlpkg.BuildInsertSQL(c.state.Schema, c.state.Table, c.columns, c.App.GetQuoter())
}

func (c *Data) buildDuplicateInsertSQL(row database.Row) string {
	colMeta := make(map[string]database.ColumnInfo, len(c.columns))
	for _, col := range c.columns {
		colMeta[col.Name] = col
	}
	return sqlpkg.BuildDuplicateInsertSQL(
		c.state.Schema, c.state.Table,
		orderedColumnNames(row, c.columns), colMeta,
		c.App.GetFormatter().SQLLiteral, row,
		c.App.GetQuoter(),
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
	c.runEditorStatement(ctx, "Duplicate Row", c.buildDuplicateInsertSQL(rows[dataRow]), "Insert error")
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

	c.runEditorStatement(ctx, "Edit Row", c.buildUpdateSQL(rows[dataRow], pk), "Update error")
	return nil
}

func (c *Data) buildUpdateSQL(row database.Row, pk *database.PrimaryKey) string {
	return sqlpkg.BuildUpdateSQL(
		c.state.Schema, c.state.Table,
		orderedColumnNames(row, c.columns), c.state.GetPrimaryKey(),
		c.App.GetFormatter().SQLLiteral, row, *pk,
		c.App.GetQuoter(),
	)
}

// selectedRowsData returns the currently selected rows and visible columns when
// at least one row is selected or (nil, nil) when nothing is selected.
func (c *Data) selectedRowsData() ([]database.Row, []string) {
	indices := c.resultGrid.GetSelectedRows()
	if len(indices) == 0 {
		return nil, nil
	}
	allRows := c.state.GetAllRows()
	if len(allRows) == 0 {
		return nil, nil
	}
	cols := c.resultGrid.VisibleColumns(allRows[0], c.columns)
	rows := make([]database.Row, 0, len(indices))
	for _, r := range indices {
		if rowData := c.resultGrid.RowData(r, allRows); rowData != nil {
			rows = append(rows, rowData)
		}
	}
	return rows, cols
}

func (c *Data) handleExportData(ctx context.Context) *tcell.EventKey {
	if c.state.Table == "" && c.state.RawSQL == "" {
		return nil
	}
	if rows, cols := c.selectedRowsData(); rows != nil {
		c.exportModal.RenderWithRows(ctx, rows, cols, c.state.Schema, c.state.Table)
		return nil
	}
	c.exportModal.Render(ctx, c.buildExportQuery(), c.state.Schema, c.state.Table)
	return nil
}

func (c *Data) buildExportQuery() string {
	if c.state.RawSQL != "" {
		return c.state.RawSQL
	}
	q := "SELECT * FROM " + c.App.GetQuoter().Table(c.state.Schema, c.state.Table)
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
	if rows, cols := c.selectedRowsData(); rows != nil {
		c.exportModal.RenderWithRows(ctx, rows, cols, c.state.Schema, c.state.Table)
		return
	}
	c.exportModal.Render(ctx, c.buildExportQuery(), c.state.Schema, c.state.Table)
}

// OpenExplain runs EXPLAIN on the last executed query and shows the viewer.
func (c *Data) OpenExplain(ctx context.Context) {
	if c.state.LastQuery != "" {
		c.runner.Execute("EXPLAIN "+c.state.LastQuery, 0, RunCallbacks{
			OnExplain: func(result, bareSql string, analyze bool) {
				c.showExplainViewer(ctx, bareSql, result, analyze)
			},
			OnError: func(err error) { modal.ShowError(c.App.Pages, "Explain error", err) },
		})
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

// Prettify reformats the SQL editor's buffer.
func (c *Data) Prettify() {
	c.sqlQueryEditor.Prettify()
}
