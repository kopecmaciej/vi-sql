//go:build wezterm

package harness

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kopecmaciej/vi-sql/internal/security"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"gopkg.in/yaml.v3"
)

// appLogPath is the shared log file for all wezterm test sessions.
// os.TempDir() keeps it in /tmp without the per-test cleanup t.TempDir() would register.
var appLogPath = filepath.Join(os.TempDir(), "vi-sql-test-harness.log")

func newTestConfig(t *testing.T, vimMode bool) (configPath, logPath string) {
	t.Helper()

	dir := t.TempDir()
	logPath = appLogPath
	configPath = filepath.Join(dir, "config.yaml")

	raw := map[string]any{
		"log": map[string]any{
			"path":        logPath,
			"level":       "trace",
			"prettyPrint": false,
		},
		"ui": map[string]any{
			"vimMode": vimMode,
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

func newTestConfigWithDSN(t *testing.T, dsn string) (configPath, logPath string) {
	t.Helper()

	driver, err := util.DetectDriverFromDSN(dsn)
	if err != nil {
		t.Fatalf("newTestConfigWithDSN: %v", err)
	}

	dir := t.TempDir()
	logPath = appLogPath
	configPath = filepath.Join(dir, "config.yaml")

	raw := map[string]any{
		"log": map[string]any{
			"path":        logPath,
			"level":       "trace",
			"prettyPrint": false,
		},
		"connections": []map[string]any{
			{"name": "test", "driver": driver, "dsn": dsn, "timeout": 10},
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

// fastMasterParams keeps Argon2id derivation under ~10ms so master-password
// tests aren't dominated by KDF time. Real defaults are 64MiB/3/4.
var fastMasterParams = security.Argon2idParams{Memory: 8 * 1024, Iterations: 1, Parallelism: 1}

func NewTestConfigWithMasterPassword(t *testing.T, password string) (configPath, logPath string) {
	t.Helper()

	dsn := os.Getenv(EnvDSN)
	if dsn == "" {
		t.Skipf("%s not set — skipping master password test", EnvDSN)
	}
	driver, err := util.DetectDriverFromDSN(dsn)
	if err != nil {
		t.Fatalf("NewTestConfigWithMasterPassword: %v", err)
	}

	salt, err := security.GenerateSalt()
	if err != nil {
		t.Fatalf("NewTestConfigWithMasterPassword: salt: %v", err)
	}
	kek, err := security.DeriveKey(password, salt, fastMasterParams)
	if err != nil {
		t.Fatalf("NewTestConfigWithMasterPassword: derive: %v", err)
	}
	dataKey, err := util.GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("NewTestConfigWithMasterPassword: data key: %v", err)
	}
	wrapped, err := util.EncryptPassword(dataKey, kek)
	if err != nil {
		t.Fatalf("NewTestConfigWithMasterPassword: wrap: %v", err)
	}

	dir := t.TempDir()
	logPath = appLogPath
	configPath = filepath.Join(dir, "config.yaml")

	raw := map[string]any{
		"log": map[string]any{
			"path":        logPath,
			"level":       "trace",
			"prettyPrint": false,
		},
		"security": map[string]any{
			"method":            "master",
			"masterSalt":        salt,
			"masterMemory":      fastMasterParams.Memory,
			"masterIterations":  fastMasterParams.Iterations,
			"masterParallelism": fastMasterParams.Parallelism,
			"masterWrappedKey":  wrapped,
		},
		"connections": []map[string]any{
			{"name": "test", "driver": driver, "dsn": dsn, "timeout": 10},
		},
		"currentConnection": "test",
	}

	out, err := yaml.Marshal(raw)
	if err != nil {
		t.Fatalf("NewTestConfigWithMasterPassword: marshal: %v", err)
	}
	if err := os.WriteFile(configPath, out, 0600); err != nil {
		t.Fatalf("NewTestConfigWithMasterPassword: write config: %v", err)
	}
	return configPath, logPath
}

func NewTestConfigWithSecurityMethod(t *testing.T, method string) (configPath, logPath string) {
	t.Helper()

	dir := t.TempDir()
	logPath = appLogPath
	configPath = filepath.Join(dir, "config.yaml")

	raw := map[string]any{
		"log": map[string]any{
			"path":        logPath,
			"level":       "trace",
			"prettyPrint": false,
		},
		"security": map[string]any{
			"method": method,
		},
	}

	out, err := yaml.Marshal(raw)
	if err != nil {
		t.Fatalf("NewTestConfigWithSecurityMethod: marshal: %v", err)
	}
	if err := os.WriteFile(configPath, out, 0600); err != nil {
		t.Fatalf("NewTestConfigWithSecurityMethod: write config: %v", err)
	}
	return configPath, logPath
}

func hasArg(args []string, flags ...string) bool {
	for _, a := range args {
		if slices.Contains(flags, a) {
			return true
		}
	}
	return false
}
