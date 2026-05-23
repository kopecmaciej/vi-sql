//go:build wezterm

package harness

import (
	"fmt"
	"testing"
	"time"
)

// WaitFor polls cond every 150 ms until it returns true or timeout elapses.
func WaitFor(t *testing.T, timeout time.Duration, desc string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", desc)
}

// WaitForRowCount polls the database until schema.table contains want rows.
func WaitForRowCount(t *testing.T, db *FixtureDB, schema, table string, want int64) {
	t.Helper()
	WaitFor(t, 8*time.Second, fmt.Sprintf("row count %d in %s.%s", want, schema, table),
		func() bool { return db.Count(schema, table) == want })
}
