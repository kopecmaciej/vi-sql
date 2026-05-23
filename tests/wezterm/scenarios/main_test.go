//go:build wezterm

package wezterm_test

import (
	"os"
	"testing"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

// TestMain sweeps schemas leaked by crashed prior runs before any test runs,
// so fixture schemas never accumulate in the test database.
func TestMain(m *testing.M) {
	harness.SweepFixtureSchemas()
	os.Exit(m.Run())
}
