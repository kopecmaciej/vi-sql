package database

import "strings"

// ValueFormatter provides driver-specific value formatting for the TUI layer.
type ValueFormatter interface {
	SQLLiteral(val any) string
	EditableString(val any) string
}

// DefaultFormatter implements ValueFormatter with plain SQL quoting.
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
