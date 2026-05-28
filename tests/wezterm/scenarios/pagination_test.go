//go:build wezterm

package wezterm_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestScrollLoadsMoreRows(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name text")
	vals := make([]string, 150)
	for i := range vals {
		vals[i] = fmt.Sprintf("('%d')", i+1)
	}
	db.Exec(fmt.Sprintf(
		"INSERT INTO %s (name) VALUES %s",
		db.Qualified(schema, table), strings.Join(vals, ","),
	))

	s := harness.SpawnConnected(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPaneTimeout("⏱", 10*time.Second)

	s.GoBottom()

	s.WaitForPaneTimeout("150 rows", 10*time.Second)
}
