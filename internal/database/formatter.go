package database

import "strings"

// ValueFormatter provides driver-specific value formatting for the TUI layer.
type ValueFormatter interface {
	// SQLLiteral formats val as a SQL literal for UPDATE templates.
	SQLLiteral(val any) string
	// EditableString formats val for display in the inline edit input field.
	EditableString(val any) string
}

// DefaultFormatter implements ValueFormatter with plain ANSI SQL quoting.
// All current backends scan values to strings or use Postgres text wire format,
// so there is nothing dialect-specific yet; drivers can drop in their own
// implementation later if needed (e.g. Postgres array literals, MySQL hex BLOBs).
type DefaultFormatter struct{}

func (DefaultFormatter) SQLLiteral(val any) string {
	if val == nil {
		return "NULL"
	}
	s := StringifyValue(val)
	if s == "NULL" {
		return "NULL"
	}
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func (DefaultFormatter) EditableString(val any) string {
	if val == nil {
		return ""
	}
	return StringifyValue(val)
}
