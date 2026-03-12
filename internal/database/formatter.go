package database

// ValueFormatter provides driver-specific value formatting for the TUI layer.
type ValueFormatter interface {
	// SQLLiteral formats val as a SQL literal for UPDATE templates.
	SQLLiteral(val any) string
	// EditableString formats val for display in the inline edit input field.
	EditableString(val any) string
}
