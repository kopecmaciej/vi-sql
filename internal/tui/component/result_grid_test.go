package component

import (
	"testing"

	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestResultGrid_Render_BracketValuesVisible(t *testing.T) {
	tests := []struct {
		col   string
		value string
	}{
		{"role", `["admin"]`},
		{"currency", `["EUR"]`},
		{"status", `["pending"]`},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			app, sim := testutil.NewTestApp(t)
			g := NewResultGrid()
			g.SetApp(app)
			g.SetRect(0, 0, 120, 40)

			rows := []database.Row{{tt.col: tt.value}}
			cols := []database.ColumnInfo{{Name: tt.col, DataType: "jsonb"}}
			g.Render(rows, cols, app.GetStyles())
			g.Draw(sim)
			sim.Sync()

			assert.True(t, testutil.ScreenContains(sim, tt.value),
				"cell value %q should be visible in the grid\nscreen:\n%s", tt.value, testutil.ScreenFull(sim))
		})
	}
}
