package component

import (
	"time"

	"github.com/kopecmaciej/vi-sql/internal/database"
)

// scrollFetcher triggers background row fetches as the cursor approaches the
// end of the loaded buffer. All methods must run on the tview main goroutine.
type scrollFetcher struct {
	runner  *QueryRunner
	state   *database.TableState
	pending bool // true while a prefetch is in flight
	noMore  bool // true once a fetch returned 0 rows (end of result reached)
}

func newScrollFetcher(runner *QueryRunner, state *database.TableState) *scrollFetcher {
	return &scrollFetcher{runner: runner, state: state}
}

// updateState is called when triggerRefresh replaces c.state with a new object.
func (s *scrollFetcher) updateState(state *database.TableState) {
	s.state = state
}

// cancel aborts any in-flight prefetch and clears stale flags. Call before any
// fresh fetch (filter/sort/refresh/tab-close) so tryPrefetch starts clean.
func (s *scrollFetcher) cancel() {
	if s.pending {
		s.runner.Cancel()
	}
	s.pending = false
	s.noMore = false
}

// tryPrefetch fires a background fetch when the cursor is within one
// visible-window of the buffer end. Safe to call on every cursor move.
func (s *scrollFetcher) tryPrefetch(cursorRow, viewHeight int, onAppend func()) {
	if s.pending || s.noMore || s.runner.IsRunning() {
		return
	}
	bufLen := s.state.RowCount()
	if bufLen == 0 {
		return
	}
	if s.state.Count > 0 && int64(bufLen) >= s.state.Count {
		return
	}
	if cursorRow < bufLen-viewHeight {
		return
	}

	s.pending = true
	s.runner.Refresh(s.state, RunCallbacks{
		OnSelect: func(rows []database.Row, _ []database.ColumnInfo, _ string, _ time.Duration) {
			s.pending = false
			if len(rows) == 0 {
				s.noMore = true
				return
			}
			s.state.AppendRows(rows)
			onAppend()
		},
		OnError:  func(_ error) { s.pending = false },
		OnCancel: func() { s.pending = false },
	})
}
