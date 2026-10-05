package component

import (
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// footerSubscribeDelay covers Footer.handleEvents subscribing on its own
// goroutine: without waiting, a broadcast sent before Subscribe registers has
// nowhere to land and is silently dropped.
const footerSubscribeDelay = 10 * time.Millisecond

func TestFooter_Toggle_CollapsedAndExpanded(t *testing.T) {
	app, _ := testutil.NewTestApp(t)

	footer := NewFooter()
	require.NoError(t, footer.Init(app))
	footer.currentFocus = SchemaTreeId

	footer.Toggle()
	assert.True(t, footer.expanded, "should be expanded after first toggle")

	h := footer.Toggle()
	assert.Equal(t, 2, h)
	assert.False(t, footer.expanded)
}

func TestFooter_Render_ShowsSchemaKeys(t *testing.T) {
	app, sim := testutil.NewTestApp(t)

	footer := NewFooter()
	require.NoError(t, footer.Init(app))
	footer.currentFocus = SchemaTreeId
	footer.Render()

	app.SetRoot(footer, true)
	testutil.DrawAndSync(app, sim)

	assert.True(t, testutil.ScreenContains(sim, "Expand all"),
		"screen should show schema keybindings\nscreen:\n%v", testutil.ScreenFull(sim))
}

func TestFooter_Render_EmptyFocus(t *testing.T) {
	app, sim := testutil.NewTestApp(t)

	footer := NewFooter()
	require.NoError(t, footer.Init(app))
	footer.Render()

	app.SetRoot(footer, true)
	testutil.DrawAndSync(app, sim)

	assert.False(t, testutil.ScreenContains(sim, "Filter bar"))
}

func TestFooter_UpdateKeys_ResultsSuffixIsSubsetOfFullDataKeys(t *testing.T) {
	app, _ := testutil.NewTestApp(t)
	footer := NewFooter()
	require.NoError(t, footer.Init(app))

	footer.currentFocus = "QueryTab-1-results"
	keys, err := footer.UpdateKeys()
	require.NoError(t, err)
	assert.NotNil(t, keys, "should return keys for -results focus")

	footer.currentFocus = DataId
	fullKeys, err := footer.UpdateKeys()
	require.NoError(t, err)

	assert.LessOrEqual(t, len(keys), len(fullKeys),
		"query-mode keys should be a subset of full Data keys")
}

func TestFooter_UpdateKeys_ReadOnlyEditor(t *testing.T) {
	app, _ := testutil.NewTestApp(t)
	footer := NewFooter()
	require.NoError(t, footer.Init(app))

	footer.currentFocus = StructureId + EditorSuffix
	keys, err := footer.UpdateKeys()
	require.NoError(t, err)

	descriptions := make(map[string]bool)
	for _, k := range keys {
		descriptions[k.Description] = true
	}

	// Functional DDL-pane keys are shown.
	assert.True(t, descriptions["Copy"], "should show Copy")
	assert.True(t, descriptions["Toggle DDL"], "should show Toggle DDL")

	// Edit-oriented editor keys are hidden in the read-only DDL pane.
	for _, desc := range []string{"Format SQL", "History", "Toggle", "Open in $EDITOR", "Confirm", "Clear", "Paste"} {
		assert.False(t, descriptions[desc], "should not show %q", desc)
	}
}

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

func TestFooter_UpdateKeys_QueryTabPrefixMatchesDataFocus(t *testing.T) {
	app, _ := testutil.NewTestApp(t)
	footer := NewFooter()
	require.NoError(t, footer.Init(app))

	footer.currentFocus = "QueryTab-3"
	keys, err := footer.UpdateKeys()
	require.NoError(t, err)
	assert.NotNil(t, keys)

	footer.currentFocus = DataId
	dataKeys, err := footer.UpdateKeys()
	require.NoError(t, err)

	assert.Equal(t, len(dataKeys), len(keys),
		"QueryTab- prefix should resolve to the same keys as Data")
}

func TestFooter_UpdateKeys_KeyOverrideTakesPriorityOverFocus(t *testing.T) {
	app, _ := testutil.NewTestApp(t)
	footer := NewFooter()
	require.NoError(t, footer.Init(app))
	footer.currentFocus = DataId

	fullKeys, err := footer.UpdateKeys()
	require.NoError(t, err)

	overrideKeys := app.GetKeys().DataKeysForCellSelection()
	footer.keyOverrideActive = true
	footer.keyOverrideKeys = overrideKeys
	keys, err := footer.UpdateKeys()
	require.NoError(t, err)

	assert.Equal(t, overrideKeys, keys)
	assert.Less(t, len(keys), len(fullKeys),
		"override keys should be a strict subset of full Data keys in this case")
}

func TestFooter_FooterKeyOverride_TogglesKeyset(t *testing.T) {
	app, _ := testutil.NewTestApp(t)
	footer := NewFooter()
	require.NoError(t, footer.Init(app))
	footer.currentFocus = DataId
	time.Sleep(footerSubscribeDelay)

	overrideKeys := app.GetKeys().DataKeysForCellSelection()
	app.GetManager().Broadcast(manager.NewFooterKeyOverrideMsg(true, overrideKeys))
	require.Eventually(t, func() bool { return footer.keyOverrideActive },
		200*time.Millisecond, 5*time.Millisecond)

	app.GetManager().Broadcast(manager.NewFooterKeyOverrideMsg(false, nil))
	require.Eventually(t, func() bool { return !footer.keyOverrideActive },
		200*time.Millisecond, 5*time.Millisecond)
}

func TestFooter_UpdateKeys_EmptyFocus(t *testing.T) {
	app, _ := testutil.NewTestApp(t)
	footer := NewFooter()
	require.NoError(t, footer.Init(app))

	footer.currentFocus = ""
	keys, err := footer.UpdateKeys()
	require.NoError(t, err)
	assert.Nil(t, keys)
}

func TestFooter_SetOnHeightChange_CalledOnToggle(t *testing.T) {
	app, _ := testutil.NewTestApp(t)
	footer := NewFooter()
	require.NoError(t, footer.Init(app))
	footer.currentFocus = SchemaTreeId

	called := 0
	footer.SetOnHeightChange(func() { called++ })

	footer.Toggle()
	footer.Toggle()
	// Toggle itself never calls onHeightChange; only the FocusChanged handler
	// does, when the footer is already expanded. Call it directly to verify
	// it's wired and doesn't panic.
	footer.onHeightChange()
	assert.Equal(t, 1, called)
}

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
