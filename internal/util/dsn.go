package util

import (
	"fmt"
	"net/url"
	"strings"
)

// ParsedDSN holds the components of a database connection URL.
type ParsedDSN struct {
	Host     string
	Port     string
	Database string
	Username string
	Password string
	SSLMode  string
}

func parseURLDSN(dsn, defaultPort, tlsParam string) (*ParsedDSN, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DSN: %w", err)
	}

	result := &ParsedDSN{
		Host:     u.Hostname(),
		Port:     u.Port(),
		Database: strings.TrimPrefix(u.Path, "/"),
		SSLMode:  u.Query().Get(tlsParam),
	}

	if u.User != nil {
		result.Username = u.User.Username()
		result.Password, _ = u.User.Password()
	}

	if result.Port == "" {
		result.Port = defaultPort
	}

	return result, nil
}

// ParsePostgresDSN parses a PostgreSQL DSN (URL form) into its components.
func ParsePostgresDSN(dsn string) (*ParsedDSN, error) {
	return parseURLDSN(dsn, "5432", "sslmode")
}

// ParseMySQLDSN parses a mysql:// URL into its components.
func ParseMySQLDSN(dsn string) (*ParsedDSN, error) {
	return parseURLDSN(dsn, "3306", "tls")
}

// BuildDSN constructs a connection URL from individual components.
func BuildDSN(scheme, host string, port int, database, username, password string, params map[string]string) string {
	var userInfo string
	if username != "" {
		if password != "" {
			userInfo = fmt.Sprintf("%s:%s@", url.PathEscape(username), url.PathEscape(password))
		} else {
			userInfo = fmt.Sprintf("%s@", url.PathEscape(username))
		}
	}

	base := fmt.Sprintf("%s://%s%s:%d/%s", scheme, userInfo, host, port, database)
	if len(params) == 0 {
		return base
	}

	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	return base + "?" + q.Encode()
}

// HidePasswordInDSN replaces the password in a connection URL with asterisks.
func HidePasswordInDSN(dsn string) string {
	if !strings.Contains(dsn, "@") {
		return dsn
	}
	parts := strings.SplitN(dsn, "://", 2)
	if len(parts) != 2 {
		return dsn
	}
	rest := parts[1]
	atIdx := strings.LastIndex(rest, "@")
	if atIdx < 0 {
		return dsn
	}
	credentials := rest[:atIdx]
	colonIdx := strings.Index(credentials, ":")
	if colonIdx < 0 {
		return dsn
	}
	return parts[0] + "://" + credentials[:colonIdx] + ":****" + rest[atIdx:]
}

// BuildPostgresDSN constructs a PostgreSQL connection URL from individual components.
func BuildPostgresDSN(host string, port int, database, username, password, sslMode string) string {
	if sslMode == "" {
		sslMode = "disable"
	}
	return BuildDSN("postgres", host, port, database, username, password, map[string]string{"sslmode": sslMode})
}
