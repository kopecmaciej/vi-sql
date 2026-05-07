package component

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
)

const maxBroadcastRows = 100

// RunCallbacks are the handlers QueryRunner calls after each execution phase.
// All callbacks are invoked via the scheduler, so they run on the tview main
// goroutine and may update UI directly without QueueUpdateDraw.
type RunCallbacks struct {
	OnRunning   func()
	OnEstimate  func(count int64) // fast row-count estimate shown while real count loads
	OnCount     func(count int64) // real count from driver
	OnSelect    func(rows []database.Row, cols []database.ColumnInfo, query string, execTime time.Duration)
	OnStatement func(affected int64, execTime time.Duration)
	OnExplain   func(result, bareSql string, analyze bool)
	OnError     func(error)
	OnCancel    func()
}

// QueryRunner manages a single in-flight database query.
// Starting a new run cancels any previous one automatically.
type QueryRunner struct {
	driver      database.Driver
	schedule    func(func()) // app.QueueUpdateDraw
	onHistory   func(sql string)
	onBroadcast func(manager.QueryResult)

	mu      sync.Mutex
	cancel  context.CancelFunc
	running bool
}

func NewQueryRunner(
	driver database.Driver,
	schedule func(func()),
	onHistory func(sql string),
	onBroadcast func(manager.QueryResult),
) *QueryRunner {
	return &QueryRunner{
		driver:      driver,
		schedule:    schedule,
		onHistory:   onHistory,
		onBroadcast: onBroadcast,
	}
}

// Cancel cancels any in-flight run.
func (r *QueryRunner) Cancel() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		r.cancel()
	}
}

// IsRunning reports whether a query is currently in flight.
func (r *QueryRunner) IsRunning() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

// Execute classifies sql (explain / select / statement) and runs it.
// Any in-flight query is cancelled first. Returns immediately.
func (r *QueryRunner) Execute(sql string, batchSize int64, cbs RunCallbacks) {
	ctx := r.start()
	go func() {
		defer r.done()
		r.fire(cbs.OnRunning)
		switch {
		case isExplainQuery(sql):
			r.runExplain(ctx, sql, cbs)
		case isSelectQuery(sql):
			r.runSelect(ctx, sql, batchSize, 0, cbs)
		default:
			r.runStatement(ctx, sql, cbs)
		}
	}()
}

// Refresh loads rows for the given table state.
// Picks ListQueryRows when state has RawSQL set, ListRows otherwise.
// Any in-flight query is cancelled first. Returns immediately.
func (r *QueryRunner) Refresh(state *database.TableState, cbs RunCallbacks) {
	ctx := r.start()
	go func() {
		defer r.done()
		r.fire(cbs.OnRunning)
		r.runRefresh(ctx, state, cbs)
	}()
}

func (r *QueryRunner) start() context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		r.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.running = true
	return ctx
}

func (r *QueryRunner) done() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.running = false
	r.cancel = nil
}

func (r *QueryRunner) fire(fn func()) {
	if fn == nil {
		return
	}
	r.schedule(fn)
}

func (r *QueryRunner) runExplain(ctx context.Context, sql string, cbs RunCallbacks) {
	bare, analyze := parseExplainPrefix(sql)
	var (
		result string
		err    error
	)
	if analyze {
		result, err = r.driver.ExplainAnalyze(ctx, bare)
	} else {
		result, err = r.driver.ExplainPlan(ctx, bare)
	}
	if err != nil {
		r.dispatchError(ctx, err, cbs)
		return
	}
	r.onHistory(sql)
	r.schedule(func() {
		if cbs.OnExplain != nil {
			cbs.OnExplain(result, bare, analyze)
		}
	})
}

func (r *QueryRunner) runSelect(ctx context.Context, sql string, batchSize, offset int64, cbs RunCallbacks) {
	start := time.Now()
	countCb := func(count int64) {
		r.schedule(func() {
			if cbs.OnCount != nil {
				cbs.OnCount(count)
			}
		})
	}
	query, rows, cols, err := r.driver.ListQueryRows(ctx, sql, batchSize, offset, countCb)
	if err != nil {
		r.dispatchError(ctx, err, cbs)
		return
	}
	execTime := time.Since(start)
	r.onHistory(sql)

	colNames := make([]string, len(cols))
	for i, col := range cols {
		colNames[i] = col.Name
	}
	capped := rows
	if len(capped) > maxBroadcastRows {
		capped = capped[:maxBroadcastRows]
	}
	r.onBroadcast(manager.QueryResult{
		Query:    sql,
		Columns:  colNames,
		Rows:     capped,
		RowCount: len(rows),
	})

	r.schedule(func() {
		if cbs.OnSelect != nil {
			cbs.OnSelect(rows, cols, query, execTime)
		}
	})
}

func (r *QueryRunner) runStatement(ctx context.Context, sql string, cbs RunCallbacks) {
	start := time.Now()
	affected, err := r.driver.ExecuteStatement(ctx, sql)
	if err != nil {
		r.dispatchError(ctx, err, cbs)
		return
	}
	execTime := time.Since(start)
	r.onHistory(sql)
	r.onBroadcast(manager.QueryResult{
		Query:    sql,
		Affected: affected,
	})
	r.schedule(func() {
		if cbs.OnStatement != nil {
			cbs.OnStatement(affected, execTime)
		}
	})
}

func (r *QueryRunner) runRefresh(ctx context.Context, state *database.TableState, cbs RunCallbacks) {
	// Pre-fill with a fast estimate for first table visit so the results bar
	// shows a number immediately while the real COUNT(*) runs in the background.
	// Only fire for Postgres (isEstimate=true); SQLite returns an exact count via
	// the countCb path below, so we skip the pre-fill there.
	if state.RawSQL == "" && state.Count == 0 {
		if est, isEstimate, err := r.driver.GetEstimatedRowCount(ctx, state.Schema, state.Table); err == nil && isEstimate && est > 0 {
			r.schedule(func() {
				if cbs.OnEstimate != nil {
					cbs.OnEstimate(est)
				}
			})
		}
	}

	start := time.Now()
	// Only create countCb when the caller actually wants count updates; passing
	// nil suppresses the background COUNT(*) query (used by scroll prefetch).
	var countCb func(int64)
	if cbs.OnCount != nil {
		countCb = func(count int64) {
			r.schedule(func() { cbs.OnCount(count) })
		}
	}

	var (
		query string
		rows  []database.Row
		cols  []database.ColumnInfo
		err   error
	)

	if state.RawSQL != "" {
		offset := int64(state.RowCount())
		query, rows, cols, err = r.driver.ListQueryRows(ctx, state.RawSQL, state.BatchSize, offset, countCb)
	} else {
		query, rows, err = r.driver.ListRows(ctx, state, state.Where, state.OrderBy, nil, countCb)
	}
	if err != nil {
		r.dispatchError(ctx, err, cbs)
		return
	}
	execTime := time.Since(start)
	r.schedule(func() {
		if cbs.OnSelect != nil {
			cbs.OnSelect(rows, cols, query, execTime)
		}
	})
}

func (r *QueryRunner) dispatchError(_ context.Context, err error, cbs RunCallbacks) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		r.fire(cbs.OnCancel)
		return
	}
	r.schedule(func() {
		if cbs.OnError != nil {
			cbs.OnError(err)
		}
	})
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

func isSelectQuery(sql string) bool {
	upper := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(upper, "SELECT") ||
		strings.HasPrefix(upper, "WITH") ||
		strings.HasPrefix(upper, "EXPLAIN") ||
		strings.HasPrefix(upper, "TABLE") ||
		database.IsReturningDML(sql)
}
