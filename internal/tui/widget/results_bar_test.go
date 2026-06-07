package widget

import (
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/testutil"
	"github.com/stretchr/testify/assert"
)

func TestResultsBar_Render_BracketValuesVisible(t *testing.T) {
	tests := []struct {
		name    string
		where   string
		orderBy string
		check   string
	}{
		{"where with jsonb filter", `permissions @> '["admin"]'`, "", `["admin"]`},
		{"where single bracket value", `status = '["pending"]'`, "", `["pending"]`},
		{"orderby with bracket value", "", `["priority"]`, `["priority"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, sim := testutil.NewTestApp(t)

			bar := NewResultsBar()
			bar.SetStyle(app.GetStyles())
			bar.SetRect(0, 0, 120, 4)

			state := database.NewTableState("public", "orders")
			state.SetWhere(tt.where)
			state.SetOrderBy(tt.orderBy)
			state.AppendRows([]database.Row{{"id": "1"}})

			bar.Render(state, time.Millisecond)
			bar.Draw(sim)
			sim.Sync()

			assert.True(t, testutil.ScreenContains(sim, tt.check),
				"value %q should be visible in results bar\nscreen:\n%s", tt.check, testutil.ScreenFull(sim))
		})
	}
}
