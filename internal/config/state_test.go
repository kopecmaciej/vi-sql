package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adrg/xdg"
)

const legacyConfigWithConnections = `version: test
currentConnection: prod
connections:
  - id: conn-1
    driver: postgres
    host: db.example.com
    port: 5432
    database: app
    username: admin
    password: enc:master:abc123
    name: prod
    lastUsed: 2026-08-18T14:50:00Z
  - id: conn-2
    driver: sqlite
    database: /tmp/local.db
    name: local
`

func TestConnectionStateMigration(t *testing.T) {
	tests := []struct {
		name          string
		configYAML    string
		stateYAML     string
		wantCurrent   string
		wantConnNames []string
	}{
		{
			name:          "migrates connections from legacy config.yaml",
			configYAML:    legacyConfigWithConnections,
			wantCurrent:   "prod",
			wantConnNames: []string{"prod", "local"},
		},
		{
			name:       "state.yaml wins over stale connections in config.yaml",
			configYAML: legacyConfigWithConnections,
			stateYAML: `currentConnection: staging
connections:
  - id: conn-9
    driver: postgres
    host: staging.example.com
    name: staging
`,
			wantCurrent:   "staging",
			wantConnNames: []string{"staging"},
		},
		{
			name:          "no connections anywhere",
			configYAML:    "version: test\n",
			wantCurrent:   "",
			wantConnNames: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			configPath := filepath.Join(tmp, "config.yaml")
			statePath := filepath.Join(tmp, "state.yaml")

			if err := os.WriteFile(configPath, []byte(tt.configYAML), 0600); err != nil {
				t.Fatal(err)
			}
			if tt.stateYAML != "" {
				if err := os.WriteFile(statePath, []byte(tt.stateYAML), 0600); err != nil {
					t.Fatal(err)
				}
			}

			cfg, err := LoadConfigWithVersion("test", configPath)
			if err != nil {
				t.Fatal(err)
			}

			if cfg.CurrentConnection != tt.wantCurrent {
				t.Errorf("CurrentConnection = %q, want %q", cfg.CurrentConnection, tt.wantCurrent)
			}
			if got := connNames(cfg); !equalStrings(got, tt.wantConnNames) {
				t.Errorf("connection names = %v, want %v", got, tt.wantConnNames)
			}

			raw, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(raw), "connections:") || strings.Contains(string(raw), "currentConnection:") {
				t.Errorf("config.yaml still contains connection state:\n%s", raw)
			}

			if len(tt.wantConnNames) > 0 {
				if _, err := os.Stat(statePath); err != nil {
					t.Errorf("state.yaml not written next to custom config: %v", err)
				}
				reloaded, err := LoadConfigWithVersion("test", configPath)
				if err != nil {
					t.Fatal(err)
				}
				if !equalStrings(connNames(reloaded), tt.wantConnNames) {
					t.Errorf("after reload names = %v, want %v", connNames(reloaded), tt.wantConnNames)
				}
			}
		})
	}
}

func TestConnectionStateMigrationKeepsConfigOnFailure(t *testing.T) {
	tests := []struct {
		name       string
		configYAML string
		setup      func(t *testing.T, dir string)
	}{
		{
			name: "malformed legacy connections abort migration",
			configYAML: `version: test
connections:
  - id: conn-1
    driver: postgres
    port: not-a-number
    name: prod
`,
		},
		{
			name:       "state write failure aborts migration",
			configYAML: legacyConfigWithConnections,
			setup: func(t *testing.T, dir string) {
				if os.Geteuid() == 0 {
					t.Skip("cannot simulate a write-permission failure as root")
				}
				if err := os.Chmod(dir, 0o555); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.configYAML), 0o600); err != nil {
				t.Fatal(err)
			}
			if tt.setup != nil {
				tt.setup(t, dir)
			}

			if _, err := LoadConfigWithVersion("test", configPath); err == nil {
				t.Fatal("expected migration to fail, got nil error")
			}

			raw, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(raw), "connections:") {
				t.Errorf("config.yaml lost its connections after a failed migration:\n%s", raw)
			}
		})
	}
}

func TestConnectionStateRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	cfg := &Config{
		ConfigPath: filepath.Join(tmp, "config.yaml"),
		Connections: []SQLConfig{
			{ID: "id-1", Name: "prod", Driver: "postgres", Password: "enc:master:xyz", LastUsed: time.Date(2026, 8, 18, 14, 50, 0, 0, time.UTC)},
		},
		CurrentConnection: "prod",
	}

	if err := cfg.saveState(); err != nil {
		t.Fatal(err)
	}

	loaded := &Config{ConfigPath: cfg.ConfigPath}
	if err := loaded.loadState(); err != nil {
		t.Fatal(err)
	}

	if loaded.CurrentConnection != "prod" {
		t.Errorf("CurrentConnection = %q", loaded.CurrentConnection)
	}
	if len(loaded.Connections) != 1 {
		t.Fatalf("got %d connections", len(loaded.Connections))
	}
	got := loaded.Connections[0]
	if got.ID != "id-1" || got.Password != "enc:master:xyz" {
		t.Errorf("connection not preserved: %+v", got)
	}
	if !got.LastUsed.Equal(cfg.Connections[0].LastUsed) {
		t.Errorf("LastUsed = %v, want %v", got.LastUsed, cfg.Connections[0].LastUsed)
	}
}

func TestConnectionStateDefaultPathUsesXDGState(t *testing.T) {
	tmp := t.TempDir()
	t.Cleanup(xdg.Reload) // runs after t.Setenv restores the real env
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(tmp, "state"))
	t.Setenv("HOME", tmp)
	xdg.Reload()

	cfg, err := LoadConfigWithVersion("test", "")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Connections = []SQLConfig{{ID: "id-1", Name: "prod", Driver: "postgres"}}
	if err := cfg.saveState(); err != nil {
		t.Fatal(err)
	}

	want := filepath.Join(tmp, "state", "vi-sql", "state.yaml")
	if _, err := os.Stat(want); err != nil {
		t.Errorf("state.yaml not at %s: %v", want, err)
	}
}

func connNames(c *Config) []string {
	if len(c.Connections) == 0 {
		return nil
	}
	names := make([]string, len(c.Connections))
	for i, conn := range c.Connections {
		names[i] = conn.Name
	}
	return names
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
