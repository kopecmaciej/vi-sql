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
	kb.loadDefaults()
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
	assert.NotEmpty(t, loaded.Schema.FilterBar.Runes,
		"Schema.FilterBar should be filled from defaults")
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
	assert.NotEmpty(t, onDisk.Schema.FilterBar.Runes,
		"Schema.FilterBar.Runes should be written back to disk")
}

func TestNewKeyInExistingStructFilledInMemory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings.yaml")

	partial := KeyBindings{}
	partial.loadDefaults()
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
	partial.loadDefaults()
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
