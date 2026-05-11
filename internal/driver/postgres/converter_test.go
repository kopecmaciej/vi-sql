package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var f = &Formatter{}

// ---------------------------------------------------------------------------
// SQLLiteral — used to generate the UPDATE template opened in the editor
// ---------------------------------------------------------------------------

func TestSQLLiteral_Nil(t *testing.T) {
	assert.Equal(t, "NULL", f.SQLLiteral(nil))
}

func TestSQLLiteral_String(t *testing.T) {
	assert.Equal(t, "'hello'", f.SQLLiteral("hello"))
}

func TestSQLLiteral_StringWithSingleQuote(t *testing.T) {
	assert.Equal(t, "'it''s'", f.SQLLiteral("it's"), "single quotes must be escaped")
}

func TestSQLLiteral_NumericString(t *testing.T) {
	assert.Equal(t, "'42'", f.SQLLiteral("42"))
}

func TestSQLLiteral_BoolString(t *testing.T) {
	assert.Equal(t, "'t'", f.SQLLiteral("t"))
	assert.Equal(t, "'f'", f.SQLLiteral("f"))
}

func TestSQLLiteral_TimestampString(t *testing.T) {
	assert.Equal(t, "'2026-03-06T15:16:45+02:00'", f.SQLLiteral("2026-03-06T15:16:45+02:00"))
}

// ---------------------------------------------------------------------------
// EditableString — pre-fills the inline-edit input field
// ---------------------------------------------------------------------------

func TestEditableString_Nil(t *testing.T) {
	assert.Equal(t, "", f.EditableString(nil))
}

func TestEditableString_String(t *testing.T) {
	assert.Equal(t, "hello", f.EditableString("hello"))
}

func TestEditableString_NumericString(t *testing.T) {
	assert.Equal(t, "42", f.EditableString("42"))
}

func TestEditableString_TimestampString(t *testing.T) {
	ts := "2026-03-06T15:16:45.802794+02:00"
	assert.Equal(t, ts, f.EditableString(ts))
}
