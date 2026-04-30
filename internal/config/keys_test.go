package config

import (
	"os"
	"path/filepath"
	"testing"

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
	assert.Equal(t, []string{"gg"}, kb.Navigation.GoTop.Chords, "vim GoTop should default to gg chord")
	assert.Equal(t, []string{"G"}, kb.Navigation.GoBottom.Runes, "vim GoBottom should default to G rune")
	assert.Equal(t, []string{"k"}, kb.Navigation.MoveUp.Runes, "vim MoveUp should include k rune")
	assert.Contains(t, kb.Data.FollowForeignKey.Chords, "gd", "vim FollowForeignKey should include gd chord")
}

func TestNormalProfileDefaults(t *testing.T) {
	kb := normalKB()
	assert.Equal(t, []string{"Ctrl+Home"}, kb.Navigation.GoTop.Keys, "normal GoTop should default to Ctrl+Home")
	assert.Equal(t, []string{"Ctrl+End"}, kb.Navigation.GoBottom.Keys, "normal GoBottom should default to Ctrl+End")
	assert.Empty(t, kb.Navigation.MoveUp.Runes, "normal MoveUp should have no runes")
	assert.Empty(t, kb.Data.FollowForeignKey.Chords, "normal FollowForeignKey should have no chords")
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
