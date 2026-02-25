package util

import (
	"encoding/json"
	"strings"
	"unicode"
)

// IsJsonEmpty checks if a JSON string is empty or only contains whitespace
func IsJsonEmpty(s string) bool {
	s = strings.ReplaceAll(s, " ", "")
	return s == "" || s == "{}"
}

// CleanJsonWhitespaces removes new lines and redundant spaces from a JSON string
// and also removes comma from the end of the string
func CleanJsonWhitespaces(s string) string {
	s = strings.TrimSuffix(s, ",")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\t", "")

	var result strings.Builder
	inQuotes := false
	prevChar := ' '

	for _, char := range s {
		if char == '"' && prevChar != '\\' {
			inQuotes = !inQuotes
		}

		if inQuotes {
			result.WriteRune(char)
		} else if !unicode.IsSpace(char) {
			result.WriteRune(char)
		} else if unicode.IsSpace(char) && prevChar != ' ' {
			result.WriteRune(char)
		}

		prevChar = char
	}

	return result.String()
}

// CleanAllWhitespaces removes all whitespaces from a string
func CleanAllWhitespaces(s string) string {
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\t", "")
	return s
}

// IndentJson indents a JSON string for display.
func IndentJson(data []byte) (string, error) {
	var out json.RawMessage
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// DeepCopy creates a deep copy of a map[string]any via JSON round-trip.
func DeepCopy(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		// fallback to shallow copy
		cp := make(map[string]any, len(m))
		for k, v := range m {
			cp[k] = v
		}
		return cp
	}
	var cp map[string]any
	if err := json.Unmarshal(b, &cp); err != nil {
		cp = make(map[string]any, len(m))
		for k, v := range m {
			cp[k] = v
		}
	}
	return cp
}
