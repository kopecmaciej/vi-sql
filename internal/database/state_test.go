package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepCopyRow_Independence(t *testing.T) {
	original := Row{"a": "hello", "b": int64(42), "c": nil}
	copied := deepCopyRow(original)

	//Mutation of copy must not affect original
	copied["a"] = "world"
	assert.Equal(t, "hello", original["a"], "original must not be mutated through copy")
}

func TestDeepCopyRow_Nil(t *testing.T) {
	assert.Nil(t, deepCopyRow(nil))
}

func TestStringifyValue_Nil(t *testing.T) {
	assert.Equal(t, "NULL", StringifyValue(nil))
}

func TestStringifyValue_String(t *testing.T) {
	assert.Equal(t, "hello", StringifyValue("hello"))
}

func TestStringifyValue_Int(t *testing.T) {
	assert.Equal(t, "42", StringifyValue(int64(42)))
}

func TestStringifyValue_Bool(t *testing.T) {
	assert.Equal(t, "true", StringifyValue(true))
	assert.Equal(t, "false", StringifyValue(false))
}

func TestStringifyValue_Bytes(t *testing.T) {
	assert.Equal(t, "hello", StringifyValue([]byte("hello")))
}

func TestStringifyValue_UUID(t *testing.T) {
	// UUIDs now arrive as text strings from PostgreSQL's text wire format.
	uuid := "01234567-89ab-cdef-0123-456789abcdef"
	assert.Equal(t, uuid, StringifyValue(uuid))
}

func TestStringifyValue_Map(t *testing.T) {
	m := map[string]interface{}{"key": "value"}
	result := StringifyValue(m)
	assert.Contains(t, result, "key")
	assert.Contains(t, result, "value")
}

func TestStringifyValue_Slice(t *testing.T) {
	s := []interface{}{1, "two", true}
	result := StringifyValue(s)
	assert.Contains(t, result, "two")
}

func TestStringifyValue_TimestampString(t *testing.T) {
	// Timestamps now arrive as text strings from PostgreSQL's text wire format.
	ts := "2026-03-06 15:16:45.802794+02"
	assert.Equal(t, ts, StringifyValue(ts))
}

func TestTableState_SetAndGetPrimaryKey(t *testing.T) {
	ts := NewTableState("public", "users")
	assert.Empty(t, ts.GetPrimaryKey())

	ts.SetPrimaryKey([]string{"id"})
	assert.Equal(t, []string{"id"}, ts.GetPrimaryKey())
}

func TestTableState_GetAllRows_ReturnsDeepCopies(t *testing.T) {
	ts := NewTableState("public", "users")
	ts.PopulateRows([]Row{{"id": int64(1), "name": "Alice"}})

	rows := ts.GetAllRows()
	require.Len(t, rows, 1)

	// Mutating the returned copy must not touch the internal state
	rows[0]["name"] = "Mutated"
	fresh := ts.GetAllRows()
	assert.Equal(t, "Alice", fresh[0]["name"], "internal state must not be mutated via returned rows")
}

func TestTableState_UpdateRow_ChangesCorrectRow(t *testing.T) {
	ts := NewTableState("public", "users")
	ts.SetPrimaryKey([]string{"id"})
	ts.PopulateRows([]Row{
		{"id": int64(1), "name": "Alice"},
		{"id": int64(2), "name": "Bob"},
	})

	pk := PrimaryKey{Columns: map[string]any{"id": int64(1)}}
	ts.UpdateRow(pk, Row{"id": int64(1), "name": "Alice Updated"})

	rows := ts.GetAllRows()
	require.Len(t, rows, 2)

	var alice, bob Row
	for _, r := range rows {
		if r["id"] == int64(1) {
			alice = r
		} else {
			bob = r
		}
	}
	assert.Equal(t, "Alice Updated", alice["name"], "updated row must have new value")
	assert.Equal(t, "Bob", bob["name"], "other rows must be unchanged")
}

// ---------------------------------------------------------------------------
// TableState — DeleteRow
// ---------------------------------------------------------------------------

func TestTableState_DeleteRow(t *testing.T) {
	state := NewTableState("public", "users")
	state.SetPrimaryKey([]string{"id"})
	state.PopulateRows([]Row{
		{"id": int64(1), "name": "Alice"},
		{"id": int64(2), "name": "Bob"},
	})
	state.Count = 2

	pk := PrimaryKey{Columns: map[string]any{"id": int64(1)}}
	state.DeleteRow(pk)

	rows := state.GetAllRows()
	assert.Len(t, rows, 1)
	assert.Equal(t, int64(1), state.Count)
	assert.Equal(t, "Bob", rows[0]["name"])
}

func TestTableState_AppendRows(t *testing.T) {
	state := NewTableState("public", "t")
	state.PopulateRows([]Row{{"id": int64(1)}})

	state.AppendRows([]Row{{"id": int64(2)}, {"id": int64(3)}})

	assert.Equal(t, 3, state.RowCount())
	rows := state.GetAllRows()
	assert.Equal(t, int64(1), rows[0]["id"])
	assert.Equal(t, int64(2), rows[1]["id"])
	assert.Equal(t, int64(3), rows[2]["id"])
}

func TestTableState_ClearBuffer(t *testing.T) {
	state := NewTableState("public", "t")
	state.PopulateRows([]Row{{"id": int64(1)}})
	state.Count = 42
	state.CountIsEstimate = true

	state.ClearBuffer()

	assert.Equal(t, 0, state.RowCount())
	assert.Equal(t, int64(0), state.Count)
	assert.False(t, state.CountIsEstimate)
}

func TestStateMap_SetAndGet(t *testing.T) {
	sm := NewStateMap()
	state := NewTableState("public", "users")
	sm.Set(sm.Key("public", "users"), state)

	got, ok := sm.Get(sm.Key("public", "users"))
	assert.True(t, ok)
	assert.Same(t, state, got)
}
