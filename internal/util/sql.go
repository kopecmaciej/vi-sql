package util

import (
	"fmt"
	"net/url"
	"strings"
)

// ParsedDSN holds parts of a PostgreSQL connection string.
type ParsedDSN struct {
	Host     string
	Port     string
	Database string
	Username string
	Password string
	SSLMode  string
}

// ParseDSN parses a PostgreSQL DSN (URL form) into its components.
func ParseDSN(dsn string) (*ParsedDSN, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DSN: %w", err)
	}

	result := &ParsedDSN{
		Host:     u.Hostname(),
		Port:     u.Port(),
		Database: strings.TrimPrefix(u.Path, "/"),
	}

	if u.User != nil {
		result.Username = u.User.Username()
		result.Password, _ = u.User.Password()
	}

	if sslMode := u.Query().Get("sslmode"); sslMode != "" {
		result.SSLMode = sslMode
	}

	if result.Port == "" {
		result.Port = "5432"
	}

	return result, nil
}

// BuildDSN constructs a PostgreSQL DSN from individual components.
func BuildDSN(host string, port int, database, username, password, sslMode string) string {
	var userInfo string
	if username != "" {
		if password != "" {
			userInfo = fmt.Sprintf("%s:%s@", url.PathEscape(username), url.PathEscape(password))
		} else {
			userInfo = fmt.Sprintf("%s@", url.PathEscape(username))
		}
	}

	if sslMode == "" {
		sslMode = "disable"
	}

	return fmt.Sprintf("postgres://%s%s:%d/%s?sslmode=%s",
		userInfo, host, port, database, sslMode)
}

// HidePasswordInDSN replaces the password in a DSN with asterisks.
func HidePasswordInDSN(dsn string) string {
	return HidePasswordInUri(dsn)
}
