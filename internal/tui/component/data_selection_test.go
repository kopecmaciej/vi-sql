package component

import (
	"errors"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/testutil"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func selectionTestTab(t *testing.T, vim bool) (*Data, *testutil.MockDriver, tcell.SimulationScreen) {
	t.Helper()
	app, screen := testutil.NewTestApp(t)
	keys, err := config.LoadKeybindings(vim)
	require.NoError(t, err)
	app.GetKeys().ReloadKeybidings(keys)
	driver := &testutil.MockDriver{}
	app.SetDriver(driver)
	app.SetFormatter(database.DefaultFormatter{})
	tab := NewTableTab()
	require.NoError(t, tab.Init(app))
	tab.state = database.NewTableState("main", "users")
	tab.state.SetPrimaryKey([]string{"id"})
	tab.state.PopulateRows([]database.Row{
		{"id": 1, "name": "Alice", "note": "keep"},
		{"id": 2, "name": "Bob", "note": "keep"},
		{"id": 3, "name": "Carol", "note": "keep"},
	})
	tab.columns = []database.ColumnInfo{{Name: "id", IsPK: true}, {Name: "name"}, {Name: "note"}}
	tab.reRenderState()
	tab.resultGrid.SetRect(0, 0, 100, 20)
	tab.resultGrid.Select(2, 2)
	app.SetFocus(tab.resultGrid)
	return tab, driver, screen
}

func gridKeys(tab *Data, keys string) {
	for _, r := range keys {
		tab.resultGrid.InputHandler()(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone), func(tview.Primitive) {})
	}
}

func TestCellSelectionNavigation(t *testing.T) {
	for _, vim := range []bool{false, true} {
		t.Run(map[bool]string{true: "vim", false: "normal"}[vim], func(t *testing.T) {
			tab, driver, screen := selectionTestTab(t, vim)
			gridKeys(tab, "vjl")
			require.Len(t, tab.resultGrid.SelectedCells(), 4)
			gridKeys(tab, "gg")
			row, col := tab.resultGrid.GetSelection()
			assert.Equal(t, 1, row)
			assert.Equal(t, 3, col)
			require.Len(t, tab.resultGrid.SelectedCells(), 4)
			gridKeys(tab, "G")
			row, _ = tab.resultGrid.GetSelection()
			assert.Equal(t, 3, row)
			gridKeys(tab, "hk")
			require.Equal(t, []SearchMatch{{Row: 2, Col: 2}}, tab.resultGrid.SelectedCells())
			gridKeys(tab, "ggkkhhhh")
			row, col = tab.resultGrid.GetSelection()
			assert.Equal(t, 1, row)
			assert.Equal(t, 1, col)
			tab.resultGrid.Draw(screen)
			x, y, _ := tab.resultGrid.GetCell(2, 2).GetLastPosition()
			_, _, style, _ := screen.GetContent(x, y)
			_, bg, _ := style.Decompose()
			assert.Equal(t, tab.App.GetStyles().Data.MultiSelectedRowColor.Color(), bg)
			gridKeys(tab, "g") // Escape also cancels an unfinished gg sequence.
			tab.resultGrid.InputHandler()(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone), func(tview.Primitive) {})
			assert.False(t, tab.resultGrid.cellSelection)
			assert.Empty(t, tab.resultGrid.SelectedCells())
			assert.Equal(t, " Table ", tab.tableFlex.GetTitle())
			driver.AssertNotCalled(t, "UpdateRows", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestBatchEditSaveAndCancel(t *testing.T) {
	for _, save := range []bool{false, true} {
		t.Run(map[bool]string{true: "save", false: "cancel"}[save], func(t *testing.T) {
			tab, driver, screen := selectionTestTab(t, true)
			original := tab.state.GetAllRows()
			gridKeys(tab, "vjlc")
			require.True(t, tab.App.Pages.HasPage(modal.InlineEditModalId))
			require.Equal(t, original, tab.state.GetAllRows())
			input := tab.inlineEdit.Form.GetFormItem(0).(*tview.InputField)
			input.SetRect(0, 0, 100, 1)
			input.Draw(screen)
			input.SetText("updated ' value")
			if save {
				driver.On("UpdateRows", mock.Anything, "main", "users", mock.MatchedBy(func(updates []database.RowUpdate) bool {
					return len(updates) == 2 && updates[0].PrimaryKey.Columns["id"] == 2 && updates[1].PrimaryKey.Columns["id"] == 3 && updates[0].Updated["name"] == "updated ' value" && updates[1].Updated["note"] == "updated ' value"
				})).Return(nil).Once()
				tab.inlineEdit.Form.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModCtrl), func(tview.Primitive) {})
				rows := tab.state.GetAllRows()
				assert.Equal(t, original[0], rows[0])
				assert.Equal(t, "updated ' value", rows[1]["name"])
				assert.Equal(t, "updated ' value", rows[2]["note"])
				driver.AssertExpectations(t)
			} else {
				tab.inlineEdit.Form.InputHandler()(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone), func(tview.Primitive) {})
				assert.Equal(t, original, tab.state.GetAllRows())
				driver.AssertNotCalled(t, "UpdateRows", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			}
			assert.False(t, tab.resultGrid.cellSelection)
			assert.False(t, tab.App.Pages.HasPage(modal.InlineEditModalId))
		})
	}
}

func TestBatchEditFailurePreservesStateAndEditor(t *testing.T) {
	tab, driver, screen := selectionTestTab(t, true)
	original := tab.state.GetAllRows()
	gridKeys(tab, "vjc")
	input := tab.inlineEdit.Form.GetFormItem(0).(*tview.InputField)
	input.SetRect(0, 0, 100, 1)
	input.Draw(screen)
	input.SetText("bad value")
	driver.On("UpdateRows", mock.Anything, "main", "users", mock.Anything).Return(errors.New("constraint failed")).Once()
	tab.inlineEdit.Form.InputHandler()(tcell.NewEventKey(tcell.KeyCtrlS, 0, tcell.ModCtrl), func(tview.Primitive) {})
	assert.Equal(t, original, tab.state.GetAllRows())
	assert.True(t, tab.resultGrid.cellSelection)
	assert.True(t, tab.App.Pages.HasPage(modal.InlineEditModalId))
	driver.AssertExpectations(t)
}

func TestSelectedRowUpdatesUseVisibleRowsAndColumns(t *testing.T) {
	tab, _, _ := selectionTestTab(t, true)
	tab.resultGrid.HideColumn(1)
	tab.search.text = "Carol"
	tab.reRenderState()
	tab.resultGrid.Select(1, 1)
	gridKeys(tab, "v")
	updates, columns, err := tab.selectedRowUpdates()
	require.NoError(t, err)
	require.Len(t, updates, 1)
	assert.Equal(t, 3, updates[0].PrimaryKey.Columns["id"])
	assert.Equal(t, []string{"name"}, columns)
	tab.state.SetPrimaryKey(nil)
	_, _, err = tab.selectedRowUpdates()
	require.Error(t, err)
}

func TestBatchEditRejectsPrimaryKeyAndReadOnlyTabs(t *testing.T) {
	tab, driver, _ := selectionTestTab(t, true)
	tab.resultGrid.Select(1, 1)
	gridKeys(tab, "v")
	_, _, err := tab.selectedRowUpdates()
	require.Error(t, err)
	tab.resultGrid.ClearSelection()
	for _, mode := range []TabMode{QueryMode, ViewMode} {
		tab.mode = mode
		gridKeys(tab, "vc")
		assert.False(t, tab.resultGrid.cellSelection)
		assert.False(t, tab.App.Pages.HasPage(modal.InlineEditModalId))
	}
	driver.AssertNotCalled(t, "UpdateRows", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestCellSelectionSurvivesAppendAndClearsOnRerender(t *testing.T) {
	tab, _, _ := selectionTestTab(t, true)
	gridKeys(tab, "vj")
	selected := tab.resultGrid.SelectedCells()
	tab.state.AppendRows([]database.Row{{"id": 4, "name": "Dave", "note": "keep"}})
	tab.renderAfterAppend()
	assert.Equal(t, selected, tab.resultGrid.SelectedCells())
	tab.reRenderState()
	assert.False(t, tab.resultGrid.cellSelection)
	assert.Equal(t, " Table ", tab.tableFlex.GetTitle())
}
