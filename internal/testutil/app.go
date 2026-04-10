package testutil

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

// NewTestApp creates a core.App wired to a SimulationScreen with all XDG
// paths redirected to a temp directory so no real user config is read or written.
// Must be called before any config loading — it sets env vars via t.Setenv.
// Registers t.Cleanup(app.Stop()) to unblock HandleEvents goroutines on test exit.
func NewTestApp(t *testing.T) (*core.App, tcell.SimulationScreen) {
	t.Helper()
	tmpDir := t.TempDir()

	// Redirect XDG BEFORE calling core.NewApp, which calls config.LoadKeybindings().
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("XDG_CACHE_HOME", tmpDir)
	// HOME fallback for systems that ignore XDG.
	t.Setenv("HOME", tmpDir)

	cfg := &config.Config{
		ConfigPath: tmpDir + "/config.yaml",
		Styles:     config.StylesConfig{CurrentStyle: "default.yaml"},
		Log:        config.LogConfig{Level: "error", Path: tmpDir + "/vi-sql.log"},
		UI:         config.UIConfig{SchemaPanelWidth: 30},
	}

	app := core.NewApp(cfg)

	sim := tcell.NewSimulationScreen("")
	if err := sim.Init(); err != nil {
		t.Fatalf("sim screen init: %v", err)
	}
	sim.SetSize(120, 40)
	app.SetScreen(sim)

	t.Cleanup(func() { app.Stop() })

	return app, sim
}
