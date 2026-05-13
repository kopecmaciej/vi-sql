package testutil

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

// ScreenRow extracts a single row of text from the simulation screen.
// Rune 0 (unwritten cell) is converted to space. Trailing spaces are stripped.
func ScreenRow(sim tcell.SimulationScreen, y int) string {
	w, _ := sim.Size()
	var sb strings.Builder
	for x := 0; x < w; x++ {
		str, _, _ := sim.Get(x, y)
		if str == "" {
			str = " "
		}
		sb.WriteString(str)
	}
	return strings.TrimRight(sb.String(), " ")
}

// ScreenContains returns true if any row of the screen contains the given string.
func ScreenContains(sim tcell.SimulationScreen, text string) bool {
	_, h := sim.Size()
	for y := 0; y < h; y++ {
		if strings.Contains(ScreenRow(sim, y), text) {
			return true
		}
	}
	return false
}

// ScreenFull returns all rows of the screen as a slice of trimmed strings.
func ScreenFull(sim tcell.SimulationScreen) []string {
	_, h := sim.Size()
	rows := make([]string, h)
	for y := 0; y < h; y++ {
		rows[y] = ScreenRow(sim, y)
	}
	return rows
}

// DrawAndSync triggers a synchronous draw and flushes the simulation screen buffer.
// Uses ForceDraw to bypass the event queue (which requires app.Run()).
func DrawAndSync(app *core.App, sim tcell.SimulationScreen) {
	app.ForceDraw()
	sim.Sync()
}

// WaitForDraw polls with short sleeps until the screen contains want or timeout expires.
// Use after operations that trigger async goroutines that modify UI state.
// Uses ForceDraw to bypass the event queue (which requires app.Run()).
func WaitForDraw(t *testing.T, app *core.App, sim tcell.SimulationScreen, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		app.ForceDraw()
		sim.Sync()
		if ScreenContains(sim, want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %q on screen after %v\nscreen dump:\n%s",
		want, timeout, strings.Join(ScreenFull(sim), "\n"))
}
