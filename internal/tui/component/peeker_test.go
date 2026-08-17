package component

import (
	"testing"

	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrettyFormatValue(t *testing.T) {
	tests := []struct {
		val      string
		dataType string
		want     string
	}{
		{`["New York", "Tokyo"]`, "jsonb", "New York"},
		{`{"status": "active", "count": 42}`, "jsonb", "active"},
		{`not valid json`, "jsonb", ""},
	}
	for _, tt := range tests {
		t.Run(tt.val, func(t *testing.T) {
			result := prettyFormatValue(tt.val, tt.dataType)
			if tt.want == "" {
				assert.Empty(t, result)
			} else {
				assert.Contains(t, result, tt.want)
			}
		})
	}
}

func TestPeeker_Render_JsonbArrayVisible(t *testing.T) {
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
			p := NewPeeker()
			require.NoError(t, p.Init(app))

			p.Render(database.Row{tt.col: tt.value}, []database.ColumnInfo{{Name: tt.col, DataType: "jsonb"}})
			p.ViewModal.Draw(sim)
			sim.Sync()

			assert.True(t, testutil.ScreenContains(sim, tt.value),
				"value %q should be visible in the peeker\nscreen:\n%s", tt.value, testutil.ScreenFull(sim))
		})
	}
}

func TestPeeker_Render_PostgresBooleanVisible(t *testing.T) {
	for _, tt := range []struct {
		name string
		raw  string
		want string
	}{
		{name: "true", raw: "t", want: "true"},
		{name: "false", raw: "f", want: "false"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			app, sim := testutil.NewTestApp(t)
			p := NewPeeker()
			require.NoError(t, p.Init(app))

			p.Render(database.Row{"active": tt.raw}, []database.ColumnInfo{
				{Name: "active", DataType: "boolean"},
			})
			p.ViewModal.Draw(sim)
			sim.Sync()

			assert.True(t, testutil.ScreenContains(sim, tt.want),
				"value %q should be visible in the peeker\nscreen:\n%s", tt.want, testutil.ScreenFull(sim))
		})
	}
}
