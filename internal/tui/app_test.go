package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kopecmaciej/vi-sql/internal/config"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/mysql"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/postgres"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyPendingConnect_NoopWhenAbsent(t *testing.T) {
	cfg := &config.Config{}
	require.NoError(t, applyPendingConnect(cfg))
	assert.Empty(t, cfg.CurrentConnection)
}

func TestApplyPendingConnect_PersistsAndEncrypts(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.yaml")

	origKey := config.EncryptionKey
	t.Cleanup(func() { config.EncryptionKey = origKey })

	cfg, err := config.LoadConfigWithVersion("test", cfgPath)
	require.NoError(t, err)
	cfg.Security.Method = config.SecurityMethodEnv
	// Simulate runApp state right after LoadEncryptionKey.
	config.EncryptionKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

	cfg.PendingConnect = "myconn=mysql://root:topsecret@localhost:3306/tui_sample_db"

	require.NoError(t, applyPendingConnect(cfg))
	assert.Empty(t, cfg.PendingConnect, "PendingConnect must be consumed")
	assert.Equal(t, "myconn", cfg.CurrentConnection)

	raw, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "topsecret", "plaintext password must not be on disk")

	reloaded, err := config.LoadConfigWithVersion("test", cfgPath)
	require.NoError(t, err)
	require.Len(t, reloaded.Connections, 1)
	conn := reloaded.Connections[0]
	assert.Equal(t, "myconn", conn.Name)
	assert.Equal(t, "mysql", conn.Driver)
	assert.Equal(t, "localhost", conn.Host)
	assert.Equal(t, 3306, conn.Port)
	assert.True(t, util.IsEncrypted(conn.Password))
}

func TestApplyPendingConnect_InvalidDSN(t *testing.T) {
	cfg := &config.Config{PendingConnect: "bad=ftp://nope"}
	assert.Error(t, applyPendingConnect(cfg))
}

func TestParseJumpTarget(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantSchema string
		wantTable  string
		wantErr    bool
	}{
		{
			name:       "valid schema/table",
			input:      "public/users",
			wantSchema: "public",
			wantTable:  "users",
		},
		{
			name:       "whitespace trimmed",
			input:      " public / users ",
			wantSchema: "public",
			wantTable:  "users",
		},
		{
			name:       "table name with slash kept as-is",
			input:      "public/schema/table",
			wantSchema: "public",
			wantTable:  "schema/table",
		},
		{
			name:    "no slash — error, not panic",
			input:   "public",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "empty schema",
			input:   "/users",
			wantErr: true,
		},
		{
			name:    "empty table",
			input:   "public/",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, table, err := parseJumpTarget(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantSchema, schema)
			assert.Equal(t, tt.wantTable, table)
		})
	}
}
