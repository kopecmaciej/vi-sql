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

func TestHidePasswordInDSN(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "postgres://user:secret@localhost:5432/mydb",
			expected: "postgres://user:****@localhost:5432/mydb",
		},
		{
			input:    "postgres://user@localhost:5432/mydb",
			expected: "postgres://user@localhost:5432/mydb",
		},
		{
			input:    "postgres://localhost:5432/mydb",
			expected: "postgres://localhost:5432/mydb",
		},
		{
			input:    "mysql://admin:p@$$word@host:3306/db",
			expected: "mysql://admin:****@host:3306/db",
		},
		{
			input:    "notaurl",
			expected: "notaurl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, HidePasswordInDSN(tt.input))
		})
	}
}
