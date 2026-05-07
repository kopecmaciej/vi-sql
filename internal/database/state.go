package database

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
)

// TableState stores the current view state for a table.
type TableState struct {
	Schema          string
	Table           string
	BatchSize       int64 // DB round-trip granularity
	UserLimit       int64 // user's explicit LIMIT value from SQL (0 = none)
	Count           int64
	CountIsEstimate bool // true when Count came from pg_class rather than COUNT(*)
	Where           string
	OrderBy         string
	Columns         string
	LastQuery       string
	RawSQL          string // non-empty when displaying an ad-hoc SQL query result
	primaryKey      []string
	rows            []Row
}

func NewTableState(schema, table string) *TableState {
	return &TableState{
		Schema: schema,
		Table:  table,
	}
}

func (t *TableState) SetPrimaryKey(cols []string) {
	t.primaryKey = cols
}

func (t *TableState) GetPrimaryKey() []string {
	return t.primaryKey
}

func (t *TableState) GetAllRows() []Row {
	copies := make([]Row, len(t.rows))
	for i, row := range t.rows {
		copies[i] = deepCopyRow(row)
	}
	return copies
}

// RowCount returns the number of rows currently in the buffer.
func (t *TableState) RowCount() int {
	return len(t.rows)
}

// AppendRows appends fetched rows to the buffer (used by scroll prefetch).
func (t *TableState) AppendRows(rows []Row) {
	for _, row := range rows {
		t.rows = append(t.rows, deepCopyRow(row))
	}
}

// ClearBuffer resets the row buffer and count. Called before a fresh fetch
// (filter change, sort change, explicit refresh).
func (t *TableState) ClearBuffer() {
	t.rows = nil
	t.Count = 0
	t.CountIsEstimate = false
}

func (t *TableState) SetWhere(where string) {
	t.Where = strings.TrimSpace(where)
}

func (t *TableState) SetOrderBy(orderBy string) {
	t.OrderBy = strings.TrimSpace(orderBy)
}

func (t *TableState) SetColumns(columns string) {
	t.Columns = strings.TrimSpace(columns)
}

func (t *TableState) PopulateRows(rows []Row) {
	t.rows = make([]Row, len(rows))
	for i, row := range rows {
		t.rows[i] = deepCopyRow(row)
	}
}

func (t *TableState) UpdateRow(pk PrimaryKey, updated Row) {
	for i, row := range t.rows {
		if matchesPK(row, pk) {
			t.rows[i] = deepCopyRow(updated)
			return
		}
	}
	t.rows = append(t.rows, deepCopyRow(updated))
}

func (t *TableState) AppendRow(row Row) {
	t.rows = append(t.rows, deepCopyRow(row))
	t.Count++
}

func (t *TableState) DeleteRow(pk PrimaryKey) {
	for i, row := range t.rows {
		if matchesPK(row, pk) {
			t.rows = append(t.rows[:i], t.rows[i+1:]...)
			t.Count--
			return
		}
	}
}

// StateMap preserves table states when switching between tables.
type StateMap struct {
	mu     sync.RWMutex
	states map[string]*TableState
}

func NewStateMap() *StateMap {
	return &StateMap{
		states: make(map[string]*TableState),
	}
}

func (sm *StateMap) Key(schema, table string) string {
	return schema + "." + table
}

func (sm *StateMap) Get(key string) (*TableState, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	state, ok := sm.states[key]
	return state, ok
}

func (sm *StateMap) Set(key string, state *TableState) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.states[key] = state
}

// Helper functions

func matchesPK(row Row, pk PrimaryKey) bool {
	for col, val := range pk.Columns {
		rowVal, ok := row[col]
		if !ok {
			return false
		}
		if !reflect.DeepEqual(rowVal, val) {
			return false
		}
	}
	return true
}

func deepCopyRow(row Row) Row {
	if row == nil {
		return nil
	}
	copy := make(Row, len(row))
	for k, v := range row {
		copy[k] = v
	}
	return copy
}

func StringifyValue(v any) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func GetSortedColumnNames(row Row) []string {
	keys := make([]string, 0, len(row))
	for k := range row {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
