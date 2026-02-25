package database

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
)

// TableState stores the current view state for a table.
type TableState struct {
	Schema     string
	Table      string
	Offset     int64
	Limit      int64
	Count      int64
	Where      string
	OrderBy    string
	Columns    string
	primaryKey []string
	rows       []Row
}

func NewTableState(schema, table string) *TableState {
	return &TableState{
		Schema: schema,
		Table:  table,
		Offset: 0,
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

func (t *TableState) GetRowByPK(pk PrimaryKey) Row {
	for _, row := range t.rows {
		if matchesPK(row, pk) {
			return deepCopyRow(row)
		}
	}
	return nil
}

func (t *TableState) GetJsonRowByPK(pk PrimaryKey) (string, error) {
	row := t.GetRowByPK(pk)
	if row == nil {
		return "", fmt.Errorf("row not found")
	}
	b, err := json.MarshalIndent(row, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (t *TableState) GetValueByPKAndColumn(pk PrimaryKey, column string) string {
	row := t.GetRowByPK(pk)
	if row == nil {
		return ""
	}
	val, ok := row[column]
	if !ok {
		return ""
	}
	return StringifyValue(val)
}

func (t *TableState) SetOffset(offset int64) {
	if offset < 0 {
		t.Offset = 0
	} else {
		t.Offset = offset
	}
}

func (t *TableState) GetCurrentPage() int64 {
	if t.Limit == 0 {
		return 1
	}
	return (t.Offset / t.Limit) + 1
}

func (t *TableState) GetTotalPages() int64 {
	if t.Limit == 0 {
		return 1
	}
	total := t.Count / t.Limit
	if t.Count%t.Limit > 0 {
		total++
	}
	return total
}

func (t *TableState) SetWhere(where string) {
	where = strings.TrimSpace(where)
	t.Where = where
	t.Offset = 0
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
	mu            sync.RWMutex
	states        map[string]*TableState
	hiddenColumns map[string][]string
}

func NewStateMap() *StateMap {
	return &StateMap{
		states:        make(map[string]*TableState),
		hiddenColumns: make(map[string][]string),
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

func (sm *StateMap) AddHiddenColumn(schema, table, column string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	key := sm.Key(schema, table)
	sm.hiddenColumns[key] = append(sm.hiddenColumns[key], column)
}

func (sm *StateMap) GetHiddenColumns(schema, table string) []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	key := sm.Key(schema, table)
	return sm.hiddenColumns[key]
}

func (sm *StateMap) ResetHiddenColumns(schema, table string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	key := sm.Key(schema, table)
	sm.hiddenColumns[key] = nil
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
