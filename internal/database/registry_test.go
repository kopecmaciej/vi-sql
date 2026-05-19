package database_test

import (
	"testing"

	"github.com/kopecmaciej/vi-sql/internal/database"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/cockroachdb"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/mariadb"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/mysql"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/postgres"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildConfigFromDSN_DefaultsNameToDriver(t *testing.T) {
	cfg, err := database.BuildConfigFromDSN("", "postgresql://user:pass@localhost:5432/mydb")
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
