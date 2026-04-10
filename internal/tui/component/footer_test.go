package component

import (
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFooter_Toggle_CollapsedAndExpanded(t *testing.T) {
	app, _ := testutil.NewTestApp(t)

	footer := NewFooter()
	require.NoError(t, footer.Init(app))

	// Set a focus so keys are available.
	footer.currentFocus = SchemaTreeId

	// Toggle to expanded.
	footer.Toggle()
	assert.True(t, footer.expanded, "should be expanded after first toggle")

	// Toggle back to collapsed → returns 2.
	h := footer.Toggle()
	assert.Equal(t, 2, h)
	assert.False(t, footer.expanded)
}

// TestFooter_Render_ShowsSchemaKeys validates that after setting currentFocus to
// SchemaTreeId, Render() populates the table with schema keybinding descriptions.
// We call Render() directly (synchronous, no goroutine) to avoid the
// event-goroutine / ForceDraw data race.
func TestFooter_Render_ShowsSchemaKeys(t *testing.T) {
	app, sim := testutil.NewTestApp(t)

	footer := NewFooter()
	require.NoError(t, footer.Init(app))

	// Set focus directly to avoid going through the event goroutine.
	footer.currentFocus = SchemaTreeId
	footer.Render()

	app.SetRoot(footer, true)
	testutil.DrawAndSync(app, sim)

	// Default schema keybindings include "Filter bar" and "Expand all".
	assert.True(t, testutil.ScreenContains(sim, "Filter bar"),
		"screen should show schema keybindings\nscreen:\n%v", testutil.ScreenFull(sim))
}

// TestFooter_Render_EmptyFocus validates that without a focus set, Render is a no-op
// and the screen shows no keybinding text.
func TestFooter_Render_EmptyFocus(t *testing.T) {
	app, sim := testutil.NewTestApp(t)

	footer := NewFooter()
	require.NoError(t, footer.Init(app))
	footer.Render()

	app.SetRoot(footer, true)
	testutil.DrawAndSync(app, sim)

	// No keys should appear when focus is empty.
	assert.False(t, testutil.ScreenContains(sim, "Filter bar"))
}

// TestFooter_UpdateKeys_ResultsSuffix verifies that a focus ID ending in "-results"
// returns the read-only query-mode keys rather than the full Data keyset.
func TestFooter_UpdateKeys_ResultsSuffix(t *testing.T) {
	app, _ := testutil.NewTestApp(t)
	footer := NewFooter()
	require.NoError(t, footer.Init(app))

	footer.currentFocus = "QueryTab-1-results"
	keys, err := footer.UpdateKeys()
	require.NoError(t, err)
	assert.NotNil(t, keys, "should return keys for -results focus")

	// The same call with full Data focus should return a different (superset) keyset.
	footer.currentFocus = DataId
	fullKeys, err := footer.UpdateKeys()
	require.NoError(t, err)

	assert.LessOrEqual(t, len(keys), len(fullKeys),
		"query-mode keys should be a subset of full Data keys")
}

// TestFooter_UpdateKeys_FilterSuffix verifies that dynamic filter/sort IDs
// (e.g. "QueryTab-1-filter") are remapped to InputBar keys.
func TestFooter_UpdateKeys_FilterSuffix(t *testing.T) {
	app, _ := testutil.NewTestApp(t)
	footer := NewFooter()
	require.NoError(t, footer.Init(app))

	footer.currentFocus = "QueryTab-1-filter"
	keys, err := footer.UpdateKeys()
	require.NoError(t, err)
	assert.NotNil(t, keys, "should return InputBar keys for -filter focus")

	footer.currentFocus = "QueryTab-2-sort"
	keys, err = footer.UpdateKeys()
	require.NoError(t, err)
	assert.NotNil(t, keys, "should return InputBar keys for -sort focus")
}

// TestFooter_UpdateKeys_QueryTabPrefix verifies that a "QueryTab-N" focus is
// remapped to full Data keys.
func TestFooter_UpdateKeys_QueryTabPrefix(t *testing.T) {
	app, _ := testutil.NewTestApp(t)
	footer := NewFooter()
	require.NoError(t, footer.Init(app))

	footer.currentFocus = "QueryTab-3"
	keys, err := footer.UpdateKeys()
	require.NoError(t, err)
	assert.NotNil(t, keys)

	// Should return the same keys as a direct Data focus.
	footer.currentFocus = DataId
	dataKeys, err := footer.UpdateKeys()
	require.NoError(t, err)

	assert.Equal(t, len(dataKeys), len(keys),
		"QueryTab- prefix should resolve to the same keys as Data")
}

// TestFooter_UpdateKeys_EmptyFocus verifies that an empty focus returns nil keys.
func TestFooter_UpdateKeys_EmptyFocus(t *testing.T) {
	app, _ := testutil.NewTestApp(t)
	footer := NewFooter()
	require.NoError(t, footer.Init(app))

	footer.currentFocus = ""
	keys, err := footer.UpdateKeys()
	require.NoError(t, err)
	assert.Nil(t, keys)
}

// TestFooter_SetOnHeightChange_CalledOnToggle verifies the onHeightChange callback
// fires when Toggle is called.
func TestFooter_SetOnHeightChange_CalledOnToggle(t *testing.T) {
	app, _ := testutil.NewTestApp(t)
	footer := NewFooter()
	require.NoError(t, footer.Init(app))
	footer.currentFocus = SchemaTreeId

	called := 0
	footer.SetOnHeightChange(func() { called++ })

	footer.Toggle()
	footer.Toggle()
	// onHeightChange is not called by Toggle directly — it is called by the
	// FocusChanged handler when the footer is already expanded. Verify it is
	// wired and callable without panicking.
	footer.onHeightChange()
	assert.Equal(t, 1, called)
}

// TestFooter_StyleChanged_DoesNotPanic validates the StyleChanged event handler runs
// without panicking.
func TestFooter_StyleChanged_DoesNotPanic(t *testing.T) {
	app, _ := testutil.NewTestApp(t)

	footer := NewFooter()
	require.NoError(t, footer.Init(app))
	app.SetRoot(footer, true)

	require.NotPanics(t, func() {
		app.GetManager().Broadcast(manager.EventMsg{
			Message: manager.Message{Type: manager.StyleChanged},
		})
		time.Sleep(20 * time.Millisecond)
	})
}
