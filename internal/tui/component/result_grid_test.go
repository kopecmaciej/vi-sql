package component

import (
	"testing"

	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/testutil"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResultGrid_CopyRowAsJSON_NestedJSONStringEmbeddedAsObject(t *testing.T) {
	var buf string
	origWrite, origRead := util.ClipboardWrite, util.ClipboardRead
	util.ClipboardWrite = func(s string) error { buf = s; return nil }
	util.ClipboardRead = func() (string, error) { return buf, nil }
	t.Cleanup(func() {
		util.ClipboardWrite = origWrite
		util.ClipboardRead = origRead
	})

	app, sim := testutil.NewTestApp(t)
	g := NewResultGrid()
	g.SetApp(app)
	g.SetRect(0, 0, 120, 40)

	rows := []database.Row{{"id": "1", "meta": `{"key":"value"}`}}
	cols := []database.ColumnInfo{{Name: "id", DataType: "int"}, {Name: "meta", DataType: "jsonb"}}
	g.Render(rows, cols, app.GetStyles())
	g.Draw(sim)

	ok := g.CopyRowAs(util.ExportJSON, 1, rows, cols)
	require.True(t, ok)

	content := util.Paste()
	assert.Contains(t, content, `"meta": {`, "nested JSON string should be embedded as an object, not escaped")
	assert.NotContains(t, content, `"meta": "{"`, "nested JSON should not be double-encoded as a string")
	assert.Contains(t, content, "    \"key\": \"value\"", "nested JSON content should be indented under its parent key")
}

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
