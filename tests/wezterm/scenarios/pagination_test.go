//go:build wezterm

package wezterm_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/tests/wezterm/harness"
)

func TestScrollLoadsMoreRows(t *testing.T) {
	schema, table, db := harness.NewFixtureTable(t, "id serial primary key, name text")
	db.Exec(fmt.Sprintf(
		"INSERT INTO %s (name) SELECT i::text FROM generate_series(1, 150) AS t(i)",
		db.Qualified(schema, table),
	))

	s := harness.SpawnConnected(t, "--jump", schema+"."+table, "--debug")
	s.WaitForPane("⏱", 10*time.Second)

	s.GoBottom()

	s.WaitForPane("150 rows", 10*time.Second)
}
