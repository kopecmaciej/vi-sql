package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func writePartialKeybindings(t *testing.T, path string) {
	t.Helper()
	partial := KeyBindings{}
	partial.Navigation = NavigationKeys{
		MoveDown: Key{Runes: []string{"j"}, Keys: []string{"Down"}, Description: "Move down"},
		MoveUp:   Key{Runes: []string{"k"}, Keys: []string{"Up"}, Description: "Move up"},
	}
	data, err := yaml.Marshal(&partial)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))
}

func defaultKB() *KeyBindings {
	kb := &KeyBindings{}
	kb.loadDefaults(true) // vim mode for backwards-compatibility with existing tests
	return kb
}

func TestMissingKeysFilledInMemory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings.yaml")
	writePartialKeybindings(t, path)

	loaded, err := util.LoadConfigFile(defaultKB(), path)
	require.NoError(t, err)

	assert.NotEmpty(t, loaded.Global.CloseApp.Keys,
		"Global.CloseApp should be filled from defaults")
	assert.NotEmpty(t, loaded.Data.PeekRow.Runes,
		"Content.PeekRow should be filled from defaults")
	assert.NotEmpty(t, loaded.Schema.ExpandAll.Runes,
		"Schema.ExpandAll should be filled from defaults")
}

func TestMissingKeysWrittenBackToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings.yaml")
	writePartialKeybindings(t, path)

	_, err := util.LoadConfigFile(defaultKB(), path)
	require.NoError(t, err)

	fileBytes, err := os.ReadFile(path)
	require.NoError(t, err)

	var onDisk KeyBindings
	require.NoError(t, yaml.Unmarshal(fileBytes, &onDisk))

	assert.NotEmpty(t, onDisk.Global.CloseApp.Keys,
		"Global.CloseApp.Keys should be written back to disk")
	assert.NotEmpty(t, onDisk.Data.PeekRow.Runes,
		"Content.PeekRow.Runes should be written back to disk")
	assert.NotEmpty(t, onDisk.Schema.ExpandAll.Runes,
		"Schema.ExpandAll.Runes should be written back to disk")
}

func TestNewKeyInExistingStructFilledInMemory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings.yaml")

	partial := KeyBindings{}
	partial.loadDefaults(true)
	partial.Data.ExplainQuery = Key{}
	data, err := yaml.Marshal(&partial)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))

	loaded, err := util.LoadConfigFile(defaultKB(), path)
	require.NoError(t, err)

	assert.NotEmpty(t, loaded.Data.ExplainQuery.Keys,
		"Content.ExplainQuery should be filled from defaults")
}

func TestNewKeyInExistingStructWrittenBackToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings.yaml")

	partial := KeyBindings{}
	partial.loadDefaults(true)
	partial.Data.ExplainQuery = Key{}
	data, err := yaml.Marshal(&partial)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))

	_, err = util.LoadConfigFile(defaultKB(), path)
	require.NoError(t, err)

	fileBytes, err := os.ReadFile(path)
	require.NoError(t, err)

	var onDisk KeyBindings
	require.NoError(t, yaml.Unmarshal(fileBytes, &onDisk))

	assert.NotEmpty(t, onDisk.Data.ExplainQuery.Keys,
		"Content.ExplainQuery.Keys should be written back to disk")
}

func TestUserOverridesPreservedAfterMerge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings.yaml")

	custom := defaultKB()
	custom.Navigation.MoveDown = Key{Runes: []string{"x"}, Keys: []string{"F1"}, Description: "custom move down"}
	data, err := yaml.Marshal(custom)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))

	loaded, err := util.LoadConfigFile(defaultKB(), path)
	require.NoError(t, err)

	assert.Equal(t, []string{"x"}, loaded.Navigation.MoveDown.Runes,
		"user-set runes must not be overwritten by defaults")
	assert.Equal(t, []string{"F1"}, loaded.Navigation.MoveDown.Keys,
		"user-set keys must not be overwritten by defaults")
}

func TestFileCreatedFromDefaultsWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings.yaml")

	loaded, err := util.LoadConfigFile(defaultKB(), path)
	require.NoError(t, err)

	assert.NotEmpty(t, loaded.Global.CloseApp.Keys,
		"returned config should have defaults")

	_, err = os.Stat(path)
	assert.NoError(t, err, "file should be created on disk")

	fileBytes, err := os.ReadFile(path)
	require.NoError(t, err)

	var onDisk KeyBindings
	require.NoError(t, yaml.Unmarshal(fileBytes, &onDisk))

	assert.NotEmpty(t, onDisk.Global.CloseApp.Keys,
		"created file should contain default keys")
}

func vimKB() *KeyBindings {
	kb := &KeyBindings{}
	kb.loadDefaults(true)
	return kb
}

func normalKB() *KeyBindings {
	kb := &KeyBindings{}
	kb.loadDefaults(false)
	return kb
}

func TestVimProfileDefaults(t *testing.T) {
	kb := vimKB()
	assert.Equal(t, []string{"gg"}, kb.Navigation.GoTop.Sequences, "vim GoTop should default to gg sequence")
	assert.Equal(t, []string{"G"}, kb.Navigation.GoBottom.Runes, "vim GoBottom should default to G rune")
	assert.Equal(t, []string{"k"}, kb.Navigation.MoveUp.Runes, "vim MoveUp should include k rune")
	assert.Contains(t, kb.Data.FollowForeignKey.Sequences, "gd", "vim FollowForeignKey should include gd sequence")
	assert.Equal(t, "Search loaded results", kb.Data.SearchWithinResults.Description)
}

func TestNormalProfileDefaults(t *testing.T) {
	kb := normalKB()
	assert.Equal(t, []string{"Ctrl+Home"}, kb.Navigation.GoTop.Keys, "normal GoTop should default to Ctrl+Home")
	assert.Equal(t, []string{"Ctrl+End"}, kb.Navigation.GoBottom.Keys, "normal GoBottom should default to Ctrl+End")
	assert.Empty(t, kb.Navigation.MoveUp.Runes, "normal MoveUp should have no runes")
	assert.Empty(t, kb.Data.FollowForeignKey.Sequences, "normal FollowForeignKey should have no sequences")
	assert.Equal(t, "Search loaded results", kb.Data.SearchWithinResults.Description)
}

func TestProfilesAreIndependent(t *testing.T) {
	dir := t.TempDir()
	vimPath := filepath.Join(dir, "keybindings-vim.yaml")
	normalPath := filepath.Join(dir, "keybindings-normal.yaml")

	// Write a vim profile that overrides GoTop to F9.
	vim := vimKB()
	vim.Navigation.GoTop = Key{Keys: []string{"F9"}, Description: "vim-custom"}
	vimData, err := yaml.Marshal(vim)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(vimPath, vimData, 0600))

	// Write a normal profile that overrides GoTop to F10.
	normal := normalKB()
	normal.Navigation.GoTop = Key{Keys: []string{"F10"}, Description: "normal-custom"}
	normalData, err := yaml.Marshal(normal)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(normalPath, normalData, 0600))

	// Load each profile from its own file — values must not bleed across.
	loadedVim, err := util.LoadConfigFile(vimKB(), vimPath)
	require.NoError(t, err)
	loadedNormal, err := util.LoadConfigFile(normalKB(), normalPath)
	require.NoError(t, err)

	assert.Equal(t, []string{"F9"}, loadedVim.Navigation.GoTop.Keys,
		"vim profile should load its own override")
	assert.Equal(t, []string{"F10"}, loadedNormal.Navigation.GoTop.Keys,
		"normal profile should load its own override, not the vim one")
}

func TestUserOverridesPreservedPerProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings-vim.yaml")

	custom := vimKB()
	custom.Navigation.MoveDown = Key{Runes: []string{"n"}, Keys: []string{"Down"}, Description: "custom move down"}
	data, err := yaml.Marshal(custom)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0600))

	loaded, err := util.LoadConfigFile(vimKB(), path)
	require.NoError(t, err)

	assert.Equal(t, []string{"n"}, loaded.Navigation.MoveDown.Runes,
		"user-set rune must survive reload")
}

// TestGetAvailableKeysSkipsNonStructFields guards against the panic caused by
// calling reflect.Value.NumField on the unexported vimMode bool field.
func TestGetAvailableKeysSkipsNonStructFields(t *testing.T) {
	kb := vimKB()
	sections := kb.GetAvailableKeys()
	assert.NotEmpty(t, sections, "GetAvailableKeys should return sections")
}

func TestNewKeyAddedToProfileFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings-vim.yaml")

	// Simulate an old vim profile file that predates Schema.ExpandAll.
	old := vimKB()
	old.Schema.ExpandAll = Key{}
	data, err := yaml.Marshal(old)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0600))

	// Load — merge must restore ExpandAll from vim defaults.
	loaded, err := util.LoadConfigFile(vimKB(), path)
	require.NoError(t, err)

	assert.NotEmpty(t, loaded.Schema.ExpandAll.Runes,
		"new key must be filled from profile defaults and written back")
}

func TestBuildSequencePrefixes_VimMode(t *testing.T) {
	kb := vimKB()
	kb.vimMode = true
	kb.buildSequencePrefixes()

	assert.NotEmpty(t, kb.sequencePrefixes, "vim mode should build sequence prefixes")
	_, hasG := kb.sequencePrefixes["g"]
	assert.True(t, hasG, "'g' must be a prefix (gg, gd are defined in vim defaults)")
}

func TestBuildSequencePrefixes_NormalMode(t *testing.T) {
	kb := normalKB()
	kb.buildSequencePrefixes()

	assert.Empty(t, kb.sequencePrefixes, "normal mode should have no sequence prefixes")
}

func mkRune(r rune) *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone)
}

func mkKey(k tcell.Key) *tcell.EventKey {
	return tcell.NewEventKey(k, 0, tcell.ModNone)
}

func TestMatchSingleRune(t *testing.T) {
	kb := &KeyBindings{}
	key := Key{Runes: []string{"j"}}
	assert.True(t, kb.Match(key, mkRune('j')))
	assert.False(t, kb.Match(key, mkRune('k')))
}

func TestMatchNamedKey(t *testing.T) {
	kb := &KeyBindings{}
	key := Key{Keys: []string{"Enter"}}
	assert.True(t, kb.Match(key, mkKey(tcell.KeyEnter)))
	assert.False(t, kb.Match(key, mkKey(tcell.KeyEsc)))
}

func TestMatchSequenceSecondRune(t *testing.T) {
	kb := &KeyBindings{sequenceState: sequenceState{pending: "g"}}
	key := Key{Sequences: []string{"gg"}}
	assert.True(t, kb.Match(key, mkRune('g')))
	assert.False(t, kb.Match(key, mkRune('d')))
}

func TestMatchSequenceRequiresPending(t *testing.T) {
	kb := &KeyBindings{} // no pending
	key := Key{Sequences: []string{"gg"}}
	// 'g' alone with no pending should NOT fire the sequence arm
	assert.False(t, kb.Match(key, mkRune('g')))
}

func TestWrapInputCapture_PrefixAbsorbed(t *testing.T) {
	kb := &KeyBindings{sequenceState: sequenceState{vimMode: true, sequencePrefixes: map[string]struct{}{"g": {}}}}
	var notified []string
	kb.OnPendingChanged = func(s string) { notified = append(notified, s) }

	innerCalled := false
	fn := kb.WrapInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		innerCalled = true
		return ev
	})

	out := fn(mkRune('g'))
	assert.Nil(t, out, "prefix rune must be consumed (return nil)")
	assert.False(t, innerCalled, "inner must not be called for a prefix rune")
	assert.Equal(t, "g", kb.pending)
	assert.Equal(t, []string{"g"}, notified)
}

func TestWrapInputCapture_SecondRuneDeliveredToInner(t *testing.T) {
	kb := &KeyBindings{sequenceState: sequenceState{vimMode: true, sequencePrefixes: map[string]struct{}{"g": {}}, pending: "g"}}
	var notified []string
	kb.OnPendingChanged = func(s string) { notified = append(notified, s) }

	innerReceived := rune(0)
	fn := kb.WrapInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyRune {
			innerReceived = ev.Rune()
		}
		return nil
	})

	out := fn(mkRune('g'))
	assert.Nil(t, out, "second rune must always be consumed")
	assert.Equal(t, rune('g'), innerReceived, "inner must receive the second rune")
	assert.Equal(t, "", kb.pending, "pending must be cleared after second rune")
	assert.Equal(t, []string{""}, notified, "OnPendingChanged(\"\") must fire")
}

func TestWrapInputCapture_NonRuneResets(t *testing.T) {
	kb := &KeyBindings{sequenceState: sequenceState{vimMode: true, sequencePrefixes: map[string]struct{}{"g": {}}, pending: "g"}}
	var notified []string
	kb.OnPendingChanged = func(s string) { notified = append(notified, s) }

	innerCalled := false
	fn := kb.WrapInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		innerCalled = true
		return ev
	})

	// Use a non-rune, non-Escape key to verify the general reset+forward path.
	// (Escape while pending is a special case: it consumes the event.)
	f1Ev := mkKey(tcell.KeyF1)
	out := fn(f1Ev)
	assert.Equal(t, f1Ev, out, "inner return value must pass through")
	assert.True(t, innerCalled)
	assert.Equal(t, "", kb.pending, "non-rune must reset pending")
	assert.Equal(t, []string{""}, notified)
}

func TestWrapInputCapture_NormalModeNeverAbsorbs(t *testing.T) {
	kb := &KeyBindings{sequenceState: sequenceState{vimMode: false}} // sequencePrefixes is nil/empty

	innerCalled := false
	fn := kb.WrapInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		innerCalled = true
		return ev
	})

	out := fn(mkRune('g'))
	assert.NotNil(t, out, "normal mode must not absorb any rune")
	assert.True(t, innerCalled)
}

func TestWrapInputCapture_OnPendingChangedEveryTransition(t *testing.T) {
	kb := &KeyBindings{sequenceState: sequenceState{vimMode: true, sequencePrefixes: map[string]struct{}{"g": {}}}}
	var notified []string
	kb.OnPendingChanged = func(s string) { notified = append(notified, s) }

	fn := kb.WrapInputCapture(func(ev *tcell.EventKey) *tcell.EventKey { return nil })

	fn(mkRune('g')) // prefix: notified("g")
	fn(mkRune('g')) // completion: notified("")

	assert.Equal(t, []string{"g", ""}, notified)
}

func TestReset_DoesNotFireWhenAlreadyClear(t *testing.T) {
	kb := &KeyBindings{}
	called := 0
	kb.OnPendingChanged = func(string) { called++ }

	kb.Reset()
	assert.Equal(t, 0, called, "Reset on already-clear state must not fire OnPendingChanged")
}

func TestWrapInputCapture_SkipAbsorbBypassesPrefix(t *testing.T) {
	kb := &KeyBindings{sequenceState: sequenceState{
		vimMode:           true,
		sequencePrefixes:  map[string]struct{}{"g": {}},
		SequencesDisabled: func() bool { return true },
	}}

	innerReceived := rune(0)
	fn := kb.WrapInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		innerReceived = ev.Rune()
		return ev
	})

	out := fn(mkRune('g'))
	assert.NotNil(t, out, "SkipAbsorb must let the rune through")
	assert.Equal(t, rune('g'), innerReceived, "inner must receive the prefix rune verbatim")
	assert.Equal(t, "", kb.pending, "no pending state must be set when absorption is skipped")
}

func TestWrapInputCapture_SkipAbsorbResetsStalePending(t *testing.T) {
	kb := &KeyBindings{sequenceState: sequenceState{
		vimMode:           true,
		sequencePrefixes:  map[string]struct{}{"g": {}},
		pending:           "g",
		SequencesDisabled: func() bool { return true },
	}}
	var notified []string
	kb.OnPendingChanged = func(s string) { notified = append(notified, s) }

	innerReceived := rune(0)
	fn := kb.WrapInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		innerReceived = ev.Rune()
		return ev
	})

	out := fn(mkRune('x'))
	assert.NotNil(t, out, "SkipAbsorb must pass the event through even with stale pending")
	assert.Equal(t, rune('x'), innerReceived)
	assert.Equal(t, "", kb.pending, "stale pending must be cleared on bypass")
	assert.Equal(t, []string{""}, notified, "OnPendingChanged(\"\") must fire when clearing stale pending")
}

func TestWrapInputCapture_SkipAbsorbFalseStillAbsorbs(t *testing.T) {
	kb := &KeyBindings{sequenceState: sequenceState{
		vimMode:           true,
		sequencePrefixes:  map[string]struct{}{"g": {}},
		SequencesDisabled: func() bool { return false },
	}}

	fn := kb.WrapInputCapture(func(ev *tcell.EventKey) *tcell.EventKey { return ev })

	out := fn(mkRune('g'))
	assert.Nil(t, out, "SkipAbsorb returning false must keep absorption behavior")
	assert.Equal(t, "g", kb.pending)
}
