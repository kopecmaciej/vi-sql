//go:build wezterm

package wezterm_test

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

// via --connect
func TestConnectTwiceSameDSN(t *testing.T) {
	dsn := os.Getenv(harness.EnvDSN)
	if dsn == "" {
		t.Skipf("%s not set — skipping reconnect scenario", harness.EnvDSN)
	}
	jump := harness.DefaultJump()
	_, table, ok := harness.ParseJump(jump)
	if !ok {
		t.Skipf("jump target %q is not in schema.table format", jump)
	}

	configPath, logPath := harness.NewTestConfig(t)

	s1 := harness.SpawnWithConfig(t, configPath, logPath, "--connect", dsn, "--jump", jump, "--debug")
	s1.WaitForPane("⏱", 10*time.Second)
	s1.AssertPaneContains(table)
	s1.KillPane()

	s2 := harness.SpawnWithConfig(t, configPath, logPath, "--connect", dsn, "--jump", jump, "--debug")
	s2.WaitForPane("⏱", 10*time.Second)
	s2.AssertPaneContains(table)
}

func TestConnectSameNameDifferentDSNErrors(t *testing.T) {
	dsn := os.Getenv(harness.EnvDSN)
	if dsn == "" {
		t.Skipf("%s not set — skipping reconnect scenario", harness.EnvDSN)
	}
	jump := harness.DefaultJump()
	_, table, ok := harness.ParseJump(jump)
	if !ok {
		t.Skipf("jump target %q is not in schema.table format", jump)
	}

	u, err := url.Parse(dsn)
	if err != nil || u.Scheme == "" {
		t.Skipf("cannot build conflicting DSN from %q", dsn)
	}
	u.Path = "/vis_sql_conflict_test"
	conflictDSN := u.String()

	configPath, logPath := harness.NewTestConfig(t)

	s1 := harness.SpawnWithConfig(t, configPath, logPath, "--connect", "test-conn="+dsn, "--jump", jump)
	s1.WaitForPane("⏱", 10*time.Second)
	s1.AssertPaneContains(table)
	s1.KillPane()

	out := harness.RunBinary(t, "--config", configPath, "--connect", "test-conn="+conflictDSN)
	if !strings.Contains(out, "already exists with a different DSN") {
		t.Errorf("expected conflict error in output, got:\n%s", out)
	}
}

func TestConnectionFormAddPostgres(t *testing.T) {
	configPath, logPath := harness.NewTestConfig(t)
	s := harness.SpawnWithConfig(t, configPath, logPath)

	AddPsqlConnectionByForm(s, "myconn", "postgresql://testuser:testpass@db.example.com:5432/testdb")

	s.AssertPaneContains("myconn")
	s.AssertPaneContains("db.example.com:5432")
	s.AssertPaneContains("testdb")

	raw, _ := os.ReadFile(configPath)
	cfg := string(raw)
	for _, want := range []string{"driver: postgres", "name: myconn", "host: db.example.com", "database: testdb"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config missing %q", want)
		}
	}
}

func TestConnectionFormAddDuplicatedNameConnection(t *testing.T) {
	configPath, logPath := harness.NewTestConfig(t)
	s := harness.SpawnWithConfig(t, configPath, logPath)

	s.WaitForPane("Pick Driver", 5*time.Second)
	AddPsqlConnectionByForm(s, "myconn", "postgresql://testuser:testpass@db.example.com:5432/testdb")
	s.KillPane()

	s = harness.SpawnWithConfig(t, configPath, logPath)
	s.WaitForPane("Saved Connections", 5*time.Second)
	s.Send("a")
	s.WaitForPane("Pick Driver", 5*time.Second)
	AddPsqlConnectionByForm(s, "myconn", "postgresql://testuser:testpass@db.example.com:5431/testdb")
	s.AssertPaneContains("Failed to save connection")
}

func AddPsqlConnectionByForm(s *harness.Session, name, dsn string) {
	// diver picker first
	s.MoveRight(3) // -> for now it's postgres
	s.Select()

	// connection corm
	s.WaitForPane("Name", 3*time.Second)
	s.Paste(name)
	s.FocusDown(1)
	s.Paste(dsn)
	s.RunQuery()
}

func TestConnectionFormAddSQLite(t *testing.T) {
	configPath, logPath := harness.NewTestConfig(t)
	s := harness.SpawnWithConfig(t, configPath, logPath)

	dbPath := filepath.Join(t.TempDir(), "test.db")
	AddSQLiteConnectionByForm(s, "sqliteconn", dbPath)

	s.AssertPaneContains("sqliteconn")

	raw, _ := os.ReadFile(configPath)
	cfg := string(raw)
	for _, want := range []string{"driver: sqlite", "name: sqliteconn"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config missing %q", want)
		}
	}
}

func TestConnectionFormDelete(t *testing.T) {
	configPath, logPath := harness.NewTestConfig(t)
	s := harness.SpawnWithConfig(t, configPath, logPath)

	AddPsqlConnectionByForm(s, "delme", "postgresql://testuser:testpass@db.example.com:5432/testdb")
	s.WaitForPane("delme", 3*time.Second)

	s.Delete()
	s.AssertPaneNotContains("delme")
	s.KillPane()

	raw, _ := os.ReadFile(configPath)
	if strings.Contains(string(raw), "delme") {
		t.Errorf("deleted connection 'delme' still found in config")
	}
}

func AddSQLiteConnectionByForm(s *harness.Session, name, dbPath string) {
	s.WaitForPane("Pick Driver", 5*time.Second)
	// drivers sorted alphabetically: cockroachdb=0, mariadb=1, mysql=2, postgres=3, sqlite=4
	s.MoveRight(4)
	s.Select()

	s.WaitForPane("Name", 3*time.Second)
	s.Paste(name)
	s.FocusDown(1) // → Path / URI
	s.Paste(dbPath)
	s.RunQuery()
}
