package database

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseValueByType converts a string value to the appropriate Go type
// based on the SQL column data type.
func ParseValueByType(value string, dataType string) (any, error) {
	if strings.EqualFold(value, "NULL") || strings.EqualFold(value, "null") {
		return nil, nil
	}

	dt := strings.ToLower(dataType)

	switch {
	case strings.Contains(dt, "int"):
		return strconv.ParseInt(value, 10, 64)
	case strings.Contains(dt, "numeric") || strings.Contains(dt, "decimal") ||
		strings.Contains(dt, "real") || strings.Contains(dt, "double") || dt == "float":
		return strconv.ParseFloat(value, 64)
	case strings.Contains(dt, "bool"):
		return strconv.ParseBool(value)
	case strings.Contains(dt, "timestamp") || strings.Contains(dt, "date"):
		return parseTimeValue(value)
	default:
		return value, nil
	}
}

func parseTimeValue(value string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, value); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time value: %s", value)
}

// SanitizeWhereClause performs basic sanity checks on user-provided WHERE input.
// It rejects obvious DDL/DML injection attempts in a filter context.
func SanitizeWhereClause(where string) error {
	if where == "" {
		return nil
	}

	upper := strings.ToUpper(strings.TrimSpace(where))
	forbidden := []string{
		"DROP ", "DELETE ", "INSERT ", "UPDATE ", "ALTER ",
		"CREATE ", "TRUNCATE ", "GRANT ", "REVOKE ",
		"EXEC ", "EXECUTE ",
	}
	for _, f := range forbidden {
		if strings.Contains(upper, f) {
			return fmt.Errorf("WHERE clause must not contain %s statements", strings.TrimSpace(f))
		}
	}
	return nil
}
