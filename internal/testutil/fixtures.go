package testutil

import (
	"path/filepath"
	"runtime"
)

// RepoRoot returns the absolute path to the module root by walking up from
// the testutil package source file's location.
func RepoRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	// testutil is at internal/testutil/, walk up: testutil → internal → repo root
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func SampleSQLitePath() string {
	return filepath.Join(RepoRoot(), "sample-sql", "sample.sqlite.sql")
}

func SamplePostgresPath() string {
	return filepath.Join(RepoRoot(), "sample-sql", "sample.postgres.sql")
}

func SampleMySQLPath() string {
	return filepath.Join(RepoRoot(), "sample-sql", "sample.mysql.sql")
}
