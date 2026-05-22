//go:build wezterm

package harness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kopecmaciej/vi-sql/internal/util"
	"gopkg.in/yaml.v3"
)

// newTestConfig creates a minimal vi-sql config in t.TempDir() with trace-level
// plain JSON logging and no connections. Returns (configPath, logPath).
func newTestConfig(t *testing.T) (configPath, logPath string) {
	t.Helper()

	dir := t.TempDir()
	logPath = filepath.Join(dir, "vi-sql.log")
	configPath = filepath.Join(dir, "config.yaml")

	raw := map[string]any{
		"log": map[string]any{
			"path":        logPath,
			"level":       "trace",
			"prettyPrint": false,
		},
	}

	out, err := yaml.Marshal(raw)
	if err != nil {
		t.Fatalf("newTestConfig: marshal: %v", err)
	}
	if err := os.WriteFile(configPath, out, 0600); err != nil {
		t.Fatalf("newTestConfig: write config: %v", err)
	}

	return configPath, logPath
}

// newTestConfigWithDSN is like newTestConfig but also writes a saved connection
// seeded from dsn so the app shows the connection list rather than the pick-driver page.
func newTestConfigWithDSN(t *testing.T, dsn string) (configPath, logPath string) {
	t.Helper()

	driver, err := util.DetectDriverFromDSN(dsn)
	if err != nil {
		t.Fatalf("newTestConfigWithDSN: %v", err)
	}

	dir := t.TempDir()
	logPath = filepath.Join(dir, "vi-sql.log")
	configPath = filepath.Join(dir, "config.yaml")

	raw := map[string]any{
		"log": map[string]any{
			"path":        logPath,
			"level":       "trace",
			"prettyPrint": false,
		},
		"connections": []map[string]any{
			{"name": "test", "driver": driver, "dsn": dsn},
		},
		"currentConnection": "test",
	}

	out, err := yaml.Marshal(raw)
	if err != nil {
		t.Fatalf("newTestConfigWithDSN: marshal: %v", err)
	}
	if err := os.WriteFile(configPath, out, 0600); err != nil {
		t.Fatalf("newTestConfigWithDSN: write config: %v", err)
	}

	return configPath, logPath
}

// hasArg reports whether any of the given flag names appear in args.
func hasArg(args []string, flags ...string) bool {
	for _, a := range args {
		for _, f := range flags {
			if a == f {
				return true
			}
		}
	}
	return false
}
