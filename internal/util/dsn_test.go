package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePostgresDSN(t *testing.T) {
	tests := []struct {
		dsn              string
		expectedHost     string
		expectedPort     string
		expectedDatabase string
		expectedUsername string
		expectedPassword string
		expectedSSLMode  string
	}{
		{
			dsn:              "postgres://user:pass@localhost:5432/mydb?sslmode=require",
			expectedHost:     "localhost",
			expectedPort:     "5432",
			expectedDatabase: "mydb",
			expectedUsername: "user",
			expectedPassword: "pass",
			expectedSSLMode:  "require",
		},
		{
			dsn:              "postgres://localhost/mydb",
			expectedHost:     "localhost",
			expectedPort:     "5432",
			expectedDatabase: "mydb",
			expectedUsername: "",
			expectedPassword: "",
			expectedSSLMode:  "",
		},
		{
			dsn:              "postgres://user@localhost:5433/mydb",
			expectedHost:     "localhost",
			expectedPort:     "5433",
			expectedDatabase: "mydb",
			expectedUsername: "user",
			expectedPassword: "",
			expectedSSLMode:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.dsn, func(t *testing.T) {
			got, err := ParsePostgresDSN(tt.dsn)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedHost, got.Host)
			assert.Equal(t, tt.expectedPort, got.Port)
			assert.Equal(t, tt.expectedDatabase, got.Database)
			assert.Equal(t, tt.expectedUsername, got.Username)
			assert.Equal(t, tt.expectedPassword, got.Password)
			assert.Equal(t, tt.expectedSSLMode, got.SSLMode)
		})
	}
}

func TestBuildPostgresDSN(t *testing.T) {
	tests := []struct {
		host     string
		port     int
		database string
		username string
		password string
		sslMode  string
		wantDSN  string
	}{
		{
			host:     "localhost",
			port:     5432,
			database: "mydb",
			username: "user",
			password: "pass",
			sslMode:  "disable",
			wantDSN:  "postgres://user:pass@localhost:5432/mydb?sslmode=disable",
		},
		{
			host:     "localhost",
			port:     5432,
			database: "mydb",
			username: "user",
			password: "",
			sslMode:  "disable",
			wantDSN:  "postgres://user@localhost:5432/mydb?sslmode=disable",
		},
		{
			host:     "localhost",
			port:     5432,
			database: "mydb",
			username: "",
			password: "",
			sslMode:  "require",
			wantDSN:  "postgres://localhost:5432/mydb?sslmode=require",
		},
		{
			host:     "localhost",
			port:     5432,
			database: "mydb",
			username: "",
			password: "",
			sslMode:  "",
			wantDSN:  "postgres://localhost:5432/mydb?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.wantDSN, func(t *testing.T) {
			got := BuildPostgresDSN(tt.host, tt.port, tt.database, tt.username, tt.password, tt.sslMode)
			assert.Equal(t, tt.wantDSN, got)
		})
	}
}

func TestBuildGaussDBDSN_TargetSessionAttrs(t *testing.T) {
	t.Run("defaults to primary", func(t *testing.T) {
		got := BuildGaussDBDSN("host", 8000, "mydb", "user", "pass", "disable", "")
		assert.Equal(t, "gaussdb://user:pass@host:8000/mydb?sslmode=disable&target_session_attrs=primary", got)
	})

	t.Run("explicit primary", func(t *testing.T) {
		got := BuildGaussDBDSN("host", 8000, "mydb", "user", "pass", "disable", "primary")
		assert.Equal(t, "gaussdb://user:pass@host:8000/mydb?sslmode=disable&target_session_attrs=primary", got)
	})

	t.Run("standby", func(t *testing.T) {
		got := BuildGaussDBDSN("host", 8000, "mydb", "user", "pass", "disable", "standby")
		assert.Equal(t, "gaussdb://user:pass@host:8000/mydb?sslmode=disable&target_session_attrs=standby", got)
	})
}

func TestBuildGaussDBDSN_MultiHost(t *testing.T) {
	t.Run("comma-separated hosts get :port appended", func(t *testing.T) {
		got := BuildGaussDBDSN("host1,host2", 8000, "mydb", "user", "pass", "disable", "primary")
		assert.Equal(t, "gaussdb://user:pass@host1:8000,host2:8000/mydb?sslmode=disable&target_session_attrs=primary", got)
	})

	t.Run("entries with explicit ports are preserved", func(t *testing.T) {
		got := BuildGaussDBDSN("host1:7000,host2", 8000, "mydb", "user", "pass", "disable", "primary")
		assert.Equal(t, "gaussdb://user:pass@host1:7000,host2:8000/mydb?sslmode=disable&target_session_attrs=primary", got)
	})

	t.Run("spaces around commas are trimmed", func(t *testing.T) {
		got := BuildGaussDBDSN("host1, host2", 8000, "mydb", "user", "pass", "disable", "primary")
		assert.Equal(t, "gaussdb://user:pass@host1:8000,host2:8000/mydb?sslmode=disable&target_session_attrs=primary", got)
	})
}

func TestFormatHostPort(t *testing.T) {
	tests := []struct {
		name string
		host string
		port int
		want string
	}{
		{name: "single host", host: "localhost", port: 8000, want: "localhost:8000"},
		{name: "multi host", host: "h1,h2", port: 8000, want: "h1:8000,h2:8000"},
		{name: "multi host with spaces", host: "h1, h2", port: 8000, want: "h1:8000,h2:8000"},
		{name: "host already has port", host: "h1:7000,h2", port: 8000, want: "h1:7000,h2:8000"},
		{name: "empty entry", host: "h1,,h2", port: 8000, want: "h1:8000,h2:8000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FormatHostPort(tt.host, tt.port))
		})
	}
}

func TestParseGaussDBDSN_TargetSessionAttrs(t *testing.T) {
	parsed, err := ParseGaussDBDSN("gaussdb://user:pass@host:8000/mydb?sslmode=disable&target_session_attrs=any")
	require.NoError(t, err)
	assert.Equal(t, "any", parsed.TargetSessionAttrs)
	assert.Equal(t, "disable", parsed.SSLMode)
}

func TestBuildThenParseDSNRoundtrip(t *testing.T) {
	dsn := BuildPostgresDSN("db.example.com", 5432, "production", "admin", "s3cr3t", "require")
	parsed, err := ParsePostgresDSN(dsn)
	require.NoError(t, err)
	assert.Equal(t, "db.example.com", parsed.Host)
	assert.Equal(t, "5432", parsed.Port)
	assert.Equal(t, "production", parsed.Database)
	assert.Equal(t, "admin", parsed.Username)
	assert.Equal(t, "s3cr3t", parsed.Password)
	assert.Equal(t, "require", parsed.SSLMode)
}

func TestDetectDriverFromDSN(t *testing.T) {
	tests := []struct {
		dsn        string
		wantDriver string
		wantErr    bool
	}{
		{"postgres://user:pass@localhost/db", "postgres", false},
		{"postgresql://user@host:5432/db", "postgres", false},
		{"mysql://user:pass@host/db", "mysql", false},
		{"mariadb://user@host/db", "mariadb", false},
		{"file:/home/user/data.db", "sqlite", false},
		{"file:relative/path.db", "sqlite", false},
		{":memory:", "sqlite", false},
		{"/home/user/data.db", "", true},
		{"relative/path.db", "", true},
		{"redis://localhost", "", true},
		{"mongodb://localhost/db", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.dsn, func(t *testing.T) {
			got, err := DetectDriverFromDSN(tt.dsn)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantDriver, got)
			}
		})
	}
}

func TestIsMultiHostDSN(t *testing.T) {
	tests := []struct {
		dsn  string
		want bool
	}{
		{"postgres://user:pass@h1:5432,h2:5432/db", true},
		{"postgres://h1:5432,h2:5432/db", true},
		{"postgres://user:pass@localhost:5432/db", false},
		{"postgres://localhost/db", false},
		// comma in query string must not trigger
		{"postgres://user@localhost/db?options=-c,search_path%3Dpublic", false},
		// no scheme
		{":memory:", false},
		// comma in password (before last @) must not trigger
		{"postgres://user:p,ass@localhost/db", false},
	}
	for _, tt := range tests {
		t.Run(tt.dsn, func(t *testing.T) {
			assert.Equal(t, tt.want, IsMultiHostDSN(tt.dsn))
		})
	}
}

func TestSplitInjectDSNPasswordRoundtrip(t *testing.T) {
	// plain password: exact string round-trip
	dsn := "postgres://user:secret@h1:5432,h2:5432/db?sslmode=require"
	masked, pw := SplitDSNPassword(dsn)
	assert.Equal(t, "secret", pw)
	assert.Equal(t, "postgres://user:****@h1:5432,h2:5432/db?sslmode=require", masked)
	assert.Equal(t, dsn, InjectDSNPassword(masked, pw))

	// no password: unchanged
	noPw := "postgres://user@h1:5432,h2:5432/db"
	maskedNoPw, pwNoPw := SplitDSNPassword(noPw)
	assert.Equal(t, "", pwNoPw)
	assert.Equal(t, noPw, maskedNoPw)

	// URL-encoded special chars in password: decode on split, re-encode on inject
	specialDSN := "postgres://user:p%40%3Ass@h1,h2/db"
	maskedSpecial, pwSpecial := SplitDSNPassword(specialDSN)
	assert.Equal(t, "p@:ss", pwSpecial)
	assert.Equal(t, "postgres://user:****@h1,h2/db", maskedSpecial)
	// after inject, re-splitting should recover the same decoded password
	_, pwAfter := SplitDSNPassword(InjectDSNPassword(maskedSpecial, pwSpecial))
	assert.Equal(t, "p@:ss", pwAfter)
}

func TestSplitDSNPasswordMasking(t *testing.T) {
	tests := []struct {
		input        string
		wantMasked   string
		wantPassword string
	}{
		{"postgres://user:secret@localhost:5432/mydb", "postgres://user:****@localhost:5432/mydb", "secret"},
		{"postgres://user@localhost:5432/mydb", "postgres://user@localhost:5432/mydb", ""},
		{"postgres://localhost:5432/mydb", "postgres://localhost:5432/mydb", ""},
		{"mysql://admin:p@$$word@host:3306/db", "mysql://admin:****@host:3306/db", "p@$$word"},
		{"notaurl", "notaurl", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			masked, pw := SplitDSNPassword(tt.input)
			assert.Equal(t, tt.wantMasked, masked)
			assert.Equal(t, tt.wantPassword, pw)
		})
	}
}
