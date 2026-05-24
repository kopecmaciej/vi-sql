//go:build wezterm

package harness

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// HarnessLogPath is where fixture DB operations (CREATE TABLE, INSERT, etc.)
// log when running in the test process. Separate from per-session log files.
const HarnessLogPath = "/tmp/vi-sql-test-harness.log"

// InitTestLogger redirects zerolog's global logger to HarnessLogPath.
// Call it from TestMain so fixture DB operations don't leak to stderr.
func InitTestLogger() {
	f, err := os.OpenFile(HarnessLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		// Non-fatal: fall back to discard rather than polluting stderr.
		log.Logger = zerolog.New(os.Stderr)
		return
	}
	log.Logger = zerolog.New(f).With().Timestamp().Logger()
}
