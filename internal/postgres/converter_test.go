package postgres

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestSQLLiteral_Integer(t *testing.T) {
	assert.Equal(t, "'42'", f.SQLLiteral(int64(42)))
}

func TestSQLLiteral_Bool(t *testing.T) {
	assert.Equal(t, "'true'", f.SQLLiteral(true))
}

func TestSQLLiteral_Time_UsesRFC3339Nano(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Warsaw")
	require.NoError(t, err)
	ts := time.Date(2026, 3, 6, 15, 16, 45, 802794000, loc)

	result := f.SQLLiteral(ts)

	// Must produce a quoted RFC3339Nano string, NOT Go's default format
	expected := "'" + ts.Format(time.RFC3339Nano) + "'"
	assert.Equal(t, expected, result)

	// Must NOT contain the "CET" abbreviation that PostgreSQL rejects
	assert.NotContains(t, result, "CET", "timezone abbreviation must not appear in SQL literal")
	// Must use numeric offset (+02:00 style)
	assert.Contains(t, result, "+", "numeric offset must be present")
}

func TestSQLLiteral_Time_UTC(t *testing.T) {
	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	result := f.SQLLiteral(ts)
	assert.Equal(t, "'"+ts.Format(time.RFC3339Nano)+"'", result)
}

func TestSQLLiteral_NullString(t *testing.T) {
	assert.Equal(t, "NULL", f.SQLLiteral(nil))
}

// ---------------------------------------------------------------------------
// EditableString — pre-fills the inline-edit input field
// ---------------------------------------------------------------------------

func TestEditableString_Time_UsesRFC3339Nano(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Warsaw")
	require.NoError(t, err)
	ts := time.Date(2026, 3, 6, 15, 16, 45, 802794000, loc)

	result := f.EditableString(ts)

	assert.Equal(t, ts.Format(time.RFC3339Nano), result)
	assert.NotContains(t, result, "CET", "CET abbreviation must not appear — PostgreSQL rejects it")
}

func TestEditableString_NonTime_FallsBackToStringify(t *testing.T) {
	assert.Equal(t, "NULL", f.EditableString(nil))
	assert.Equal(t, "hello", f.EditableString("hello"))
	assert.Equal(t, "42", f.EditableString(int64(42)))
}

// ---------------------------------------------------------------------------
// Round-trip invariant
// ---------------------------------------------------------------------------

func TestRoundTrip_TimeValue_NoTimezoneAbbreviation(t *testing.T) {
	locs := []string{"Europe/Warsaw", "America/New_York", "Asia/Tokyo", "UTC"}
	for _, locName := range locs {
		t.Run(locName, func(t *testing.T) {
			loc, err := time.LoadLocation(locName)
			require.NoError(t, err)
			ts := time.Date(2026, 6, 15, 12, 30, 0, 123456000, loc)

			editable := f.EditableString(ts)
			literal := f.SQLLiteral(ts)

			// Both must NOT contain named timezone abbreviations
			for _, abbr := range []string{"CET", "CEST", "EST", "EDT", "JST"} {
				assert.NotContains(t, editable, abbr, "editable string must not contain %s", abbr)
				assert.NotContains(t, literal, abbr, "SQL literal must not contain %s", abbr)
			}

			// editable string must be parseable back as RFC3339Nano
			parsed, err := time.Parse(time.RFC3339Nano, editable)
			require.NoError(t, err, "EditableString output must be parseable as RFC3339Nano")
			assert.True(t, ts.Equal(parsed), "parsed time must equal original")
		})
	}
}
