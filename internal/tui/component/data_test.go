package component

import (
	"context"
	"testing"

	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newDataMock returns a MockDriver configured for Data/TableTab tests.
// It stubs all the calls made during HandleTableSelection → listRows →
// loadAutocompleteKeys so the component can initialise without errors.
func newDataMock(rows []database.Row, cols []database.ColumnInfo) *testutil.MockDriver {
	m := &testutil.MockDriver{}
	m.On("GetTableColumns", mock.Anything, mock.Anything, mock.Anything).Return(cols, nil)
	m.On("GetEstimatedRowCount", mock.Anything, mock.Anything, mock.Anything).Return(int64(0), nil)
	m.On("ListRows", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("SELECT *", rows, nil)
	m.On("GetTableColumnNames", mock.Anything, mock.Anything, mock.Anything).Return([]string{"id", "name"}, nil)
	m.On("ListSchemas", mock.Anything, mock.Anything).
		Return([]database.Schema{{Schema: "public", Tables: []string{"users"}}}, nil)
	return m
}

// TestData_HandleTableSelection_LimitReflectsRenderedHeight verifies that once the
// component has been placed on screen and drawn, the limit is derived from the
// actual rendered table height rather than a hard-coded default.
//
// The simulation screen is 120×40. After rendering, the table inner height is
// screen height minus the borders and results-bar rows, so the limit must be
// significantly larger than the 9 produced by the unrendered bug path and
// must NOT equal the 50 fallback (because the rect is now valid).
func TestData_HandleTableSelection_LimitReflectsRenderedHeight(t *testing.T) {
	app, sim := testutil.NewTestApp(t)

	var capturedLimit int64
	m := &testutil.MockDriver{}
	m.On("GetTableColumns", mock.Anything, mock.Anything, mock.Anything).
		Return([]database.ColumnInfo{}, nil)
	m.On("GetEstimatedRowCount", mock.Anything, mock.Anything, mock.Anything).
		Return(int64(0), nil)
	m.On("ListRows", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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

	// Set the tab as root and draw so the layout engine assigns real coordinates.
	app.SetRoot(tab, true)
	tab.Render()
	testutil.DrawAndSync(app, sim)

	err := tab.HandleTableSelection(context.Background(), "public", "users")
	require.NoError(t, err)

	// After a proper draw on a 120×40 screen the inner table height must be
	// meaningfully larger than the 9-row bug value and must not be the 50 fallback
	// (which only fires when the rect returns ≤ 0).
	assert.Greater(t, capturedLimit, int64(9),
		"limit should exceed the 9-row bug value after a proper render; got %d", capturedLimit)
	assert.NotEqual(t, int64(50), capturedLimit,
		"limit should NOT be the unrendered fallback (50) when the table has real dimensions; got %d", capturedLimit)
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
	// Override ListRows to capture the limit.
	m.ExpectedCalls = nil
	m.On("GetTableColumns", mock.Anything, mock.Anything, mock.Anything).
		Return([]database.ColumnInfo{}, nil)
	m.On("GetEstimatedRowCount", mock.Anything, mock.Anything, mock.Anything).
		Return(int64(0), nil)
	m.On("ListRows", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
	m.On("GetEstimatedRowCount", mock.Anything, mock.Anything, mock.Anything).
		Return(int64(0), nil)
	m.On("ListRows", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
	require.Equal(t, 1, callCount, "ListRows should be called on first visit")

	// Manually override the limit in the saved state to a known value.
	tab.state.BatchSize = int64(123)
	tab.stateMap.Set(tab.stateMap.Key("public", "users"), tab.state)

	// Second visit: the state map entry should be found and reused.
	err = tab.HandleTableSelection(context.Background(), "public", "users")
	require.NoError(t, err)
	require.Equal(t, 2, callCount, "ListRows should be called on second visit too")

	assert.Equal(t, int64(123), limitsObserved[1],
		"second visit should reuse the saved state limit, not recalculate from screen")
}
