package component

import (
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestScrollFetcherFetchNextAppendsRows(t *testing.T) {
	driver := &testutil.MockDriver{}
	driver.On("FetchQueryRows", mock.Anything, "SELECT *", int64(1), int64(1)).
		Return("SELECT *", []database.Row{{"id": "2"}}, []database.ColumnInfo{{Name: "id"}}, nil).Once()

	runner := NewQueryRunner(driver, func(fn func()) { fn() }, func(string, time.Duration) {}, func(manager.QueryResult) {})
	state := database.NewTableState("", "")
	state.RawSQL = "SELECT *"
	state.BatchSize = 1
	state.PopulateRows([]database.Row{{"id": "1"}})
	fetcher := newScrollFetcher(runner, state)

	done := make(chan struct{})
	require.True(t, fetcher.fetchNext(func() { close(done) }))
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for rows")
	}

	assert.Equal(t, 2, state.RowCount())
	assert.False(t, state.AllRowsLoaded, "a full page should leave more rows available")
	driver.AssertExpectations(t)
}

func TestScrollFetcherFetchNextMarksEndOfResults(t *testing.T) {
	driver := &testutil.MockDriver{}
	driver.On("FetchQueryRows", mock.Anything, "SELECT *", int64(2), int64(1)).
		Return("SELECT *", nil, []database.ColumnInfo{{Name: "id"}}, nil).Once()

	runner := NewQueryRunner(driver, func(fn func()) { fn() }, func(string, time.Duration) {}, func(manager.QueryResult) {})
	state := database.NewTableState("", "")
	state.RawSQL = "SELECT *"
	state.BatchSize = 2
	state.PopulateRows([]database.Row{{"id": "1"}})
	fetcher := newScrollFetcher(runner, state)

	done := make(chan struct{})
	require.True(t, fetcher.fetchNext(func() { close(done) }))
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for end-of-results callback")
	}

	assert.True(t, state.AllRowsLoaded)
	assert.False(t, fetcher.fetchNext(nil))
	driver.AssertExpectations(t)
}
