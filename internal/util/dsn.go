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

// ParseSQLServerDSN parses a sqlserver:// URL into its components.
// The database name is read from the ?database= query parameter.
func ParseSQLServerDSN(dsn string) (*ParsedDSN, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DSN: %w", err)
	}
	result := &ParsedDSN{
		Host:     u.Hostname(),
		Port:     u.Port(),
		Database: u.Query().Get("database"),
		SSLMode:  u.Query().Get("encrypt"),
	}
	if u.User != nil {
		result.Username = u.User.Username()
		result.Password, _ = u.User.Password()
	}
	if result.Port == "" {
		result.Port = "1433"
	}
	return result, nil
}

// BuildSQLServerDSN constructs a sqlserver:// connection URL.
func BuildSQLServerDSN(host string, port int, database, username, password, encrypt string) string {
	var userInfo string
	if username != "" {
		if password != "" {
			userInfo = url.PathEscape(username) + ":" + url.PathEscape(password) + "@"
		} else {
			userInfo = url.PathEscape(username) + "@"
		}
	}
	params := url.Values{}
	if database != "" {
		params.Set("database", database)
	}
	if encrypt != "" {
		params.Set("encrypt", encrypt)
	}
	dsn := fmt.Sprintf("sqlserver://%s%s:%d", userInfo, host, port)
	if len(params) > 0 {
		dsn += "?" + params.Encode()
	}
	return dsn
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

// DatabaseNameFromDSN extracts a human-friendly default connection name from a
// DSN. For URL-form DSNs the database name (last path segment, no extension) is
// returned. Returns "" when nothing useful can be extracted.

// DatabaseNameFromDSN returns
func DatabaseNameFromDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil || u.Path == "" {
		return ""
	}
	db := strings.TrimLeft(u.Path, "/")
	if db == "" {
		return ""
	}
	// For file-path DSNs (SQLite) strip directory prefix and extension.
	if idx := strings.LastIndexByte(db, '/'); idx >= 0 {
		db = db[idx+1:]
	}
	if idx := strings.LastIndexByte(db, '.'); idx > 0 {
		db = db[:idx]
	}
	return db
}

// DetectDriverFromDSN returns the driver name inferred from the DSN scheme.
func DetectDriverFromDSN(dsn string) (string, error) {
	lower := strings.ToLower(dsn)
	switch {
	case strings.HasPrefix(lower, "cockroachdb://"):
		return "cockroachdb", nil
	case strings.HasPrefix(lower, "postgres://"), strings.HasPrefix(lower, "postgresql://"):
		return "postgres", nil
	case strings.HasPrefix(lower, "mysql://"):
		return "mysql", nil
	case strings.HasPrefix(lower, "mariadb://"):
		return "mariadb", nil
	case strings.HasPrefix(lower, "sqlserver://"):
		return "sqlserver", nil
	case strings.HasPrefix(lower, "oracle://"):
		return "oracle", nil
	case strings.HasPrefix(lower, "sqlite://"),
		strings.HasPrefix(lower, ":memory:"),
		strings.HasPrefix(lower, "file:"):
		return "sqlite", nil
	default:
		return "", fmt.Errorf("unrecognised DSN scheme in %q", dsn)
	}
}

// ParseOracleDSN parses an oracle:// URL into its components.
// The service name is read from the URL path (e.g. oracle://user:pass@host:1521/ORCL).
func ParseOracleDSN(dsn string) (*ParsedDSN, error) {
	return parseURLDSN(dsn, "1521", "")
}

// BuildOracleDSN constructs an oracle:// connection URL.
func BuildOracleDSN(host string, port int, service, username, password string) string {
	return BuildDSN("oracle", host, port, service, username, password, nil)
}

// BuildPostgresDSN constructs a PostgreSQL connection URL from individual components.
func BuildPostgresDSN(host string, port int, database, username, password, sslMode string) string {
	if sslMode == "" {
		sslMode = "disable"
	}
	return BuildDSN("postgres", host, port, database, username, password, map[string]string{"sslmode": sslMode})
}

// IsMultiHostDSN reports whether dsn contains multiple hosts in the authority
func IsMultiHostDSN(dsn string) bool {
	_, rest, ok := strings.Cut(dsn, "://")
	if !ok {
		return false
	}
	authority := rest
	if i := strings.IndexAny(authority, "/?"); i >= 0 {
		authority = authority[:i]
	}
	if at := strings.LastIndex(authority, "@"); at >= 0 {
		authority = authority[at+1:]
	}
	return strings.Contains(authority, ",")
}

// SplitDSNPassword extracts the password from a DSN's userinfo. It returns the
// DSN with the password replaced by "****" and the url-decoded password. If no
// password is present both return values reflect that: (dsn, "").
func SplitDSNPassword(dsn string) (masked, password string) {
	parts := strings.SplitN(dsn, "://", 2)
	if len(parts) != 2 {
		return dsn, ""
	}
	rest := parts[1]
	atIdx := strings.LastIndex(rest, "@")
	if atIdx < 0 {
		return dsn, ""
	}
	username, rawPassword, ok := strings.Cut(rest[:atIdx], ":")
	if !ok {
		return dsn, ""
	}
	pw, _ := url.PathUnescape(rawPassword)
	return parts[0] + "://" + username + ":****@" + rest[atIdx+1:], pw
}

// InjectDSNPassword inserts a password into the userinfo of a DSN, replacing
// any existing password or "****" placeholder. The password is url-escaped.
// If password is empty the DSN is returned unchanged.
func InjectDSNPassword(dsn, password string) string {
	if password == "" {
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
	username, _, _ := strings.Cut(rest[:atIdx], ":")
	return parts[0] + "://" + username + ":" + url.PathEscape(password) + "@" + rest[atIdx+1:]
}
