package database_test

import (
	"testing"

	"github.com/kopecmaciej/vi-sql/internal/database"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/cockroachdb"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/gaussdb"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/mariadb"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/mysql"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/postgres"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildConfigFromDSN_DefaultsNameToDatabase(t *testing.T) {
	cfg, err := database.BuildConfigFromDSN("", "postgresql://user:pass@localhost:5432/mydb")
	require.NoError(t, err)
	assert.Equal(t, "mydb", cfg.Name)
}

func TestBuildConfigFromDSN_DefaultsNameToDriverWhenNoDatabase(t *testing.T) {
	cfg, err := database.BuildConfigFromDSN("", "postgresql://user:pass@localhost:5432/")
	require.NoError(t, err)
	assert.Equal(t, "postgres", cfg.Name)
}

func TestBuildConfigFromDSN_ExplicitNamePreserved(t *testing.T) {
	cfg, err := database.BuildConfigFromDSN("my-pg", "postgresql://user:pass@localhost:5432/mydb")
	require.NoError(t, err)
	assert.Equal(t, "my-pg", cfg.Name)
}

func TestBuildConfigFromDSN_PostgresTimeoutNonZero(t *testing.T) {
	cfg, err := database.BuildConfigFromDSN("my-conn", "postgresql://user:pass@localhost:5432/mydb")
	require.NoError(t, err)
	assert.Greater(t, cfg.Timeout, 0)
}

func TestBuildConfigFromDSN_PostgresParsesDSNFields(t *testing.T) {
	cfg, err := database.BuildConfigFromDSN("my-conn", "postgresql://alice:secret@db.example.com:5433/mydb")
	require.NoError(t, err)
	assert.Equal(t, "postgres", cfg.Driver)
	assert.Equal(t, "db.example.com", cfg.Host)
	assert.Equal(t, 5433, cfg.Port)
	assert.Equal(t, "alice", cfg.Username)
	assert.Equal(t, "mydb", cfg.Database)
	assert.Equal(t, "my-conn", cfg.Name)
}

func TestBuildConfigFromDSN_UnknownScheme(t *testing.T) {
	_, err := database.BuildConfigFromDSN("", "ftp://example.com/db")
	assert.Error(t, err)
}

func TestBuildConfigFromDSN_InvalidPostgresDSN(t *testing.T) {
	_, err := database.BuildConfigFromDSN("", "postgresql://")
	assert.Error(t, err)
}

func TestBuildConfigFromDSN_SQLite(t *testing.T) {
	cases := []struct {
		name    string
		dsn     string
		wantDSN string
	}{
		{"file URI", "file:/home/user/mydb.sqlite", "file:/home/user/mydb.sqlite"},
		{"memory", ":memory:", ":memory:"},
		{"explicit name", "file:/tmp/db.sqlite", "file:/tmp/db.sqlite"},
		{"sqlite scheme", "sqlite:///home/user/mydb.sqlite", "sqlite:///home/user/mydb.sqlite"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := database.BuildConfigFromDSN("", tc.dsn)
			require.NoError(t, err)
			assert.Equal(t, "sqlite", cfg.Driver)
			assert.Equal(t, tc.wantDSN, cfg.DSN)
		})
	}
}
