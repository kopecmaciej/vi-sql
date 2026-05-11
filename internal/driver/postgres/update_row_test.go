package postgres

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// changedColumns replicates the diff logic from UpdateRow so we can test it
// without a real database connection.
func changedColumns(original, updated map[string]any) []string {
	var changed []string
	for col, newVal := range updated {
		if col == "_pk" {
			continue
		}
		oldVal, exists := original[col]
		if !exists || !reflect.DeepEqual(oldVal, newVal) {
			changed = append(changed, col)
		}
	}
	return changed
}

// ---------------------------------------------------------------------------
// Comparable scalar types
// ---------------------------------------------------------------------------

func TestChangedColumns_NoChange_String(t *testing.T) {
	orig := map[string]any{"name": "Alice"}
	updated := map[string]any{"name": "Alice"}
	assert.Empty(t, changedColumns(orig, updated))
}

func TestChangedColumns_Changed_String(t *testing.T) {
	orig := map[string]any{"name": "Alice"}
	updated := map[string]any{"name": "Bob"}
	assert.ElementsMatch(t, []string{"name"}, changedColumns(orig, updated))
}

func TestChangedColumns_NoChange_Int(t *testing.T) {
	orig := map[string]any{"age": "30"}
	updated := map[string]any{"age": "30"}
	assert.Empty(t, changedColumns(orig, updated))
}

func TestChangedColumns_Changed_Int(t *testing.T) {
	orig := map[string]any{"age": "30"}
	updated := map[string]any{"age": "31"}
	assert.ElementsMatch(t, []string{"age"}, changedColumns(orig, updated))
}

func TestChangedColumns_NoChange_Bool(t *testing.T) {
	orig := map[string]any{"active": "t"}
	updated := map[string]any{"active": "t"}
	assert.Empty(t, changedColumns(orig, updated))
}

// ---------------------------------------------------------------------------
// Uncomparable types — these would panic with != but must not with DeepEqual
// ---------------------------------------------------------------------------

func TestChangedColumns_NoChange_ByteSlice(t *testing.T) {
	b := []byte{1, 2, 3}
	orig := map[string]any{"data": b}
	updated := map[string]any{"data": []byte{1, 2, 3}}
	// Must not panic and must detect no change
	assert.Empty(t, changedColumns(orig, updated))
}

func TestChangedColumns_Changed_ByteSlice(t *testing.T) {
	orig := map[string]any{"data": []byte{1, 2, 3}}
	updated := map[string]any{"data": []byte{4, 5, 6}}
	assert.ElementsMatch(t, []string{"data"}, changedColumns(orig, updated))
}

func TestChangedColumns_NoChange_JSONArray(t *testing.T) {
	arr := []interface{}{"a", "b", "c"}
	orig := map[string]any{"tags": arr}
	updated := map[string]any{"tags": []interface{}{"a", "b", "c"}}
	// Must not panic
	assert.Empty(t, changedColumns(orig, updated))
}

func TestChangedColumns_Changed_JSONArray(t *testing.T) {
	orig := map[string]any{"tags": []interface{}{"a", "b"}}
	updated := map[string]any{"tags": []interface{}{"a", "b", "c"}}
	assert.ElementsMatch(t, []string{"tags"}, changedColumns(orig, updated))
}

func TestChangedColumns_NoChange_JSONObject(t *testing.T) {
	orig := map[string]any{"meta": map[string]interface{}{"k": "v"}}
	updated := map[string]any{"meta": map[string]interface{}{"k": "v"}}
	assert.Empty(t, changedColumns(orig, updated))
}

func TestChangedColumns_Changed_JSONObject(t *testing.T) {
	orig := map[string]any{"meta": map[string]interface{}{"k": "v1"}}
	updated := map[string]any{"meta": map[string]interface{}{"k": "v2"}}
	assert.ElementsMatch(t, []string{"meta"}, changedColumns(orig, updated))
}

// ---------------------------------------------------------------------------
// timestamp strings — values arrive from PostgreSQL as text
// ---------------------------------------------------------------------------

func TestChangedColumns_NoChange_Timestamp(t *testing.T) {
	orig := map[string]any{"created_at": "2026-03-06T15:16:45Z"}
	updated := map[string]any{"created_at": "2026-03-06T15:16:45Z"}
	assert.Empty(t, changedColumns(orig, updated))
}

func TestChangedColumns_Changed_Timestamp(t *testing.T) {
	orig := map[string]any{"created_at": "2026-03-06T15:16:45Z"}
	updated := map[string]any{"created_at": "2026-03-06T16:16:45Z"}
	assert.ElementsMatch(t, []string{"created_at"}, changedColumns(orig, updated))
}

// ---------------------------------------------------------------------------
// _pk column is always skipped
// ---------------------------------------------------------------------------

func TestChangedColumns_SkipsPKColumn(t *testing.T) {
	orig := map[string]any{"_pk": "ignored", "name": "Alice"}
	updated := map[string]any{"_pk": "different", "name": "Alice"}
	assert.Empty(t, changedColumns(orig, updated))
}

// ---------------------------------------------------------------------------
// New column in updated (not present in original) is treated as changed
// ---------------------------------------------------------------------------

func TestChangedColumns_NewColumn(t *testing.T) {
	orig := map[string]any{"name": "Alice"}
	updated := map[string]any{"name": "Alice", "email": "alice@example.com"}
	assert.ElementsMatch(t, []string{"email"}, changedColumns(orig, updated))
}
