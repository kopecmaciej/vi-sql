package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureStdout redirects os.Stdout for the duration of fn and returns whatever
// was written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)

	origStdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origStdout })

	fn()

	require.NoError(t, w.Close())
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	return buf.String()
}

func TestValidateDirectNavigateFormat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid format", "public/users", false},
		{"valid with dashes", "my-schema/my-table", false},
		{"missing slash", "publicusers", true},
		{"empty schema", "/users", true},
		{"empty table", "public/", true},
		{"too many slashes", "a/b/c", true},
		{"both empty", "/", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDirectNavigateFormat(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListAvailableConnections_NoConnections(t *testing.T) {
	cfg := &config.Config{}
	out := captureStdout(t, func() { listAvailableConnections(cfg) })
	assert.Contains(t, out, "No connections available")
}

func TestListAvailableConnections_SingleConnection(t *testing.T) {
	cfg := &config.Config{
		Connections: []config.SQLConfig{
			{Name: "local", Host: "localhost", Port: 5432, Database: "mydb", Username: "user", Driver: "postgres"},
		},
	}
	out := captureStdout(t, func() { listAvailableConnections(cfg) })
	assert.Contains(t, out, "local")
	assert.Contains(t, out, "Available connections:")
}

func TestListAvailableConnections_CurrentConnectionMarked(t *testing.T) {
	cfg := &config.Config{
		CurrentConnection: "prod",
		Connections: []config.SQLConfig{
			{Name: "dev", Host: "localhost", Port: 5432, Database: "dev", Username: "u", Driver: "postgres"},
			{Name: "prod", Host: "prod.example.com", Port: 5432, Database: "prod", Username: "u", Driver: "postgres"},
		},
	}
	out := captureStdout(t, func() { listAvailableConnections(cfg) })

	// The format is: "<mark> <name> <DSN>" where mark is "*" or " ".
	// Find lines by splitting on the name field (first word after the marker).
	var devLine, prodLine string
	for _, l := range strings.Split(out, "\n") {
		fields := strings.Fields(l)
		if len(fields) < 2 {
			continue
		}
		// First field is either "*" or the connection name (when mark is space).
		// If first field is "*", name is second field; otherwise name is first field.
		var name string
		if fields[0] == "*" {
			name = fields[1]
		} else {
			name = fields[0]
		}
		switch name {
		case "dev":
			devLine = l
		case "prod":
			prodLine = l
		}
	}

	assert.True(t, strings.HasPrefix(strings.TrimLeft(prodLine, " "), "*"),
		"prod line should be marked with *: %q", prodLine)
	assert.False(t, strings.HasPrefix(strings.TrimLeft(devLine, " "), "*"),
		"dev line should not be marked: %q", devLine)
}

func TestListAvailableConnections_ColumnAlignment(t *testing.T) {
	cfg := &config.Config{
		Connections: []config.SQLConfig{
			{Name: "short", Host: "localhost", Port: 5432, Database: "db", Username: "u", Driver: "postgres"},
			{Name: "a-very-long-connection-name", Host: "localhost", Port: 5432, Database: "db", Username: "u", Driver: "postgres"},
		},
	}
	out := captureStdout(t, func() { listAvailableConnections(cfg) })

	assert.Contains(t, out, "short")
	assert.Contains(t, out, "a-very-long-connection-name")
}

func TestLogging_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"

	f := logging(logPath, zerolog.InfoLevel)
	require.NotNil(t, f)
	require.NoError(t, f.Close())

	_, err := os.Stat(logPath)
	assert.NoError(t, err, "log file should have been created")
}

func TestLogging_AppendsToExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := tmpDir + "/existing.log"

	// Pre-create file with content.
	require.NoError(t, os.WriteFile(logPath, []byte("previous\n"), 0644))

	f := logging(logPath, zerolog.InfoLevel)
	require.NotNil(t, f)
	require.NoError(t, f.Close())

	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "previous")
}

func TestRootCmdFlags_AllRegistered(t *testing.T) {
	type flagSpec struct {
		name      string
		shorthand string
	}

	persistentFlags := []flagSpec{
		{"config", "c"},
	}
	localFlags := []flagSpec{
		{"version", "v"},
		{"debug", "d"},
		{"options-page", "o"},
		{"connection-page", "p"},
		{"connection-name", "n"},
		{"connection-list", "l"},
		{"gen-key", ""},
		{"jump", "j"},
		{"reset-master-password", ""},
		{"connect", ""},
		{"name", ""},
	}

	for _, spec := range persistentFlags {
		f := rootCmd.PersistentFlags().Lookup(spec.name)
		require.NotNilf(t, f, "persistent flag --%s should be registered", spec.name)
		if spec.shorthand != "" {
			assert.Equal(t, spec.shorthand, f.Shorthand, "flag --%s shorthand", spec.name)
		}
	}

	for _, spec := range localFlags {
		f := rootCmd.Flags().Lookup(spec.name)
		require.NotNilf(t, f, "flag --%s should be registered", spec.name)
		if spec.shorthand != "" {
			assert.Equal(t, spec.shorthand, f.Shorthand, "flag --%s shorthand", spec.name)
		}
	}
}
