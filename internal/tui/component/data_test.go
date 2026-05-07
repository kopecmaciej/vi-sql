package component

import (
	"context"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/testutil"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newDataMock returns a MockDriver configured for Data/TableTab tests.
// It stubs all the calls made during HandleTableSelection → runner.Refresh →
// loadAutocompleteKeys so the component can initialise without errors.
func newDataMock(rows []database.Row, cols []database.ColumnInfo) *testutil.MockDriver {
	m := &testutil.MockDriver{}
	m.On("GetTableColumns", mock.Anything, mock.Anything, mock.Anything).Return(cols, nil)
	m.On("GetTableForeignKeys", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	m.On("GetEstimatedRowCount", mock.Anything, mock.Anything, mock.Anything).Return(int64(0), false, nil)
	m.On("FetchTableRows", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("SELECT *", rows, nil)
	m.On("GetTableColumnNames", mock.Anything, mock.Anything, mock.Anything).Return([]string{"id", "name"}, nil)
	m.On("ListSchemas", mock.Anything, mock.Anything).
		Return([]database.Schema{{Schema: "public", Tables: []string{"users"}}}, nil)
	return m
}

// awaitIdle waits for the tab's query runner to go idle and then forces a draw
// to flush any pending QueueUpdateDraw callbacks. Fails the test on timeout.
func awaitIdle(t *testing.T, app *core.App, tab *Data) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for tab.IsQueryRunning() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for query runner to go idle")
		}
		time.Sleep(time.Millisecond)
	}
	app.ForceDraw()
}

// TestData_HandleTableSelection_LimitDefault verifies that the batch size defaults
// to 100 when no Options.Limit is configured for the connection.
func TestData_HandleTableSelection_LimitDefault(t *testing.T) {
	app, _ := testutil.NewTestApp(t)

	var capturedLimit int64
	m := &testutil.MockDriver{}
	m.On("GetTableColumns", mock.Anything, mock.Anything, mock.Anything).
		Return([]database.ColumnInfo{}, nil)
	m.On("GetTableForeignKeys", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	m.On("GetEstimatedRowCount", mock.Anything, mock.Anything, mock.Anything).
		Return(int64(0), false, nil)
	m.On("FetchTableRows", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			state, ok := args.Get(1).(*database.TableState)
			if ok {
				capturedLimit = state.BatchSize
			}
		}).
		Return("SELECT *", []database.Row{}, nil)
	m.On("GetTableColumnNames", mock.Anything, mock.Anything, mock.Anything).
		Return([]string{}, nil)
	m.On("ListSchemas", mock.Anything, mock.Anything).
		Return([]database.Schema{}, nil)

	app.SetDriver(m)

	tab := NewTableTab()
	require.NoError(t, tab.Init(app))

	err := tab.HandleTableSelection(context.Background(), "public", "users")
	require.NoError(t, err)
	awaitIdle(t, app, tab)

	assert.Equal(t, int64(100), capturedLimit, "default batch size should be 100")
}

// TestData_HandleTableSelection_LimitFromConfig verifies that when a connection
// has an explicit row limit configured, that value takes priority over the
// screen-derived height.
func TestData_HandleTableSelection_LimitFromConfig(t *testing.T) {
	app, _ := testutil.NewTestApp(t)

	configuredLimit := int64(200)
	cfg := app.GetConfig()
	conn := cfg.GetCurrentConnection()
	if conn != nil {
		conn.Options.Limit = &configuredLimit
	}

	var capturedLimit int64
	m := newDataMock([]database.Row{}, []database.ColumnInfo{})
	// Override FetchTableRows to capture the limit.
	m.ExpectedCalls = nil
	m.On("GetTableColumns", mock.Anything, mock.Anything, mock.Anything).
		Return([]database.ColumnInfo{}, nil)
	m.On("GetTableForeignKeys", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	m.On("GetEstimatedRowCount", mock.Anything, mock.Anything, mock.Anything).
		Return(int64(0), false, nil)
	m.On("FetchTableRows", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			state, ok := args.Get(1).(*database.TableState)
			if ok {
				capturedLimit = state.BatchSize
			}
		}).
		Return("SELECT *", []database.Row{}, nil)
	m.On("GetTableColumnNames", mock.Anything, mock.Anything, mock.Anything).
		Return([]string{}, nil)
	m.On("ListSchemas", mock.Anything, mock.Anything).
		Return([]database.Schema{}, nil)

	app.SetDriver(m)

	tab := NewTableTab()
	require.NoError(t, tab.Init(app))

	err := tab.HandleTableSelection(context.Background(), "public", "users")
	require.NoError(t, err)
	awaitIdle(t, app, tab)

	if conn != nil {
		assert.Equal(t, configuredLimit, capturedLimit,
			"configured connection limit should override screen-derived height")
	} else {
		t.Log("no current connection in test config — skipping config-limit assertion")
	}
}

// TestData_HandleTableSelection_ReusesStateOnRevisit ensures that when a table
// has already been visited, the saved state (including its limit) is reused
// without recalculating from the screen.
func TestData_HandleTableSelection_ReusesStateOnRevisit(t *testing.T) {
	app, _ := testutil.NewTestApp(t)

	var callCount int
	var limitsObserved []int64

	m := &testutil.MockDriver{}
	m.On("GetTableColumns", mock.Anything, mock.Anything, mock.Anything).
		Return([]database.ColumnInfo{}, nil)
	m.On("GetTableForeignKeys", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	m.On("GetEstimatedRowCount", mock.Anything, mock.Anything, mock.Anything).
		Return(int64(0), false, nil)
	m.On("FetchTableRows", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			callCount++
			state, ok := args.Get(1).(*database.TableState)
			if ok {
				limitsObserved = append(limitsObserved, state.BatchSize)
			}
		}).
		Return("SELECT *", []database.Row{}, nil)
	m.On("GetTableColumnNames", mock.Anything, mock.Anything, mock.Anything).
		Return([]string{}, nil)
	m.On("ListSchemas", mock.Anything, mock.Anything).
		Return([]database.Schema{}, nil)

	app.SetDriver(m)

	tab := NewTableTab()
	require.NoError(t, tab.Init(app))

	// First visit: state is created fresh.
	err := tab.HandleTableSelection(context.Background(), "public", "users")
	require.NoError(t, err)
	awaitIdle(t, app, tab) // wait for runner + flush OnSelect (which saves to stateMap)
	require.Equal(t, 1, callCount, "FetchTableRows should be called on first visit")

	// Manually override the limit in the saved state to a known value.
	// awaitIdle already flushed OnSelect which saved tab.state to stateMap,
	// so overriding now is safe and won't be clobbered.
	tab.state.BatchSize = int64(123)
	tab.stateMap.Set(tab.stateMap.Key("public", "users"), tab.state)

	// Second visit: the state map entry should be found and reused.
	err = tab.HandleTableSelection(context.Background(), "public", "users")
	require.NoError(t, err)
	awaitIdle(t, app, tab)
	require.Equal(t, 2, callCount, "FetchTableRows should be called on second visit too")

	assert.Equal(t, int64(123), limitsObserved[1],
		"second visit should reuse the saved state limit, not recalculate from screen")
}
