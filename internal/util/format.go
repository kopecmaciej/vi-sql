package util

// NormalizeBooleanValue converts PostgreSQL's text boolean representation to
// the values used by the TUI.
func NormalizeBooleanValue(value string, isBoolean bool) string {
	if !isBoolean {
		return value
	}
	switch value {
	case "t":
		return "true"
	case "f":
		return "false"
	default:
		return value
	}
}
