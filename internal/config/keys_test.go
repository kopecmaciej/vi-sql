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

// writePartialKeybindings writes a keybindings file with only Navigation keys set,
// simulating an old config created before other sections existed.
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

// TestMissingKeysFilledInMemory verifies that keys absent from the file are
// filled from defaults in the returned struct. This should pass today.
func TestMissingKeysFilledInMemory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings.yaml")
	writePartialKeybindings(t, path)

	loaded, err := util.LoadConfigFile(defaultKB(), path)
	require.NoError(t, err)

	assert.NotEmpty(t, loaded.Global.CloseApp.Keys,
		"Global.CloseApp should be filled from defaults in memory")
	assert.NotEmpty(t, loaded.Content.PeekRow.Runes,
		"Content.PeekRow should be filled from defaults in memory")
	assert.NotEmpty(t, loaded.Schema.FilterBar.Runes,
		"Schema.FilterBar should be filled from defaults in memory")
}

// TestNewKeyInExistingStructFilledInMemory simulates adding a new field to an
// existing struct (e.g. a new ContentKeys entry). The loaded struct should get
// the default value. This should pass today.
func TestNewKeyInExistingStructFilledInMemory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings.yaml")

	// Write a file that has content keys but is missing ToggleFilterOptions
	// (as if the user's file predates that key being added).
	partial := KeyBindings{}
	partial.loadDefaults()
	partial.Content.ToggleFilterOptions = Key{} // clear it to simulate missing key
	data, err := yaml.Marshal(&partial)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))

	loaded, err := util.LoadConfigFile(defaultKB(), path)
	require.NoError(t, err)

	assert.NotEmpty(t, loaded.Content.ToggleFilterOptions.Keys,
		"Content.ToggleFilterOptions should be filled from defaults in memory")
}

// TestMissingKeysWrittenBackToFile verifies that after loading an old keybindings
// file, new default keys are written back to disk so users can see and edit them.
//
// THIS TEST EXPOSES THE BUG: currently the file is never updated after a merge,
// so new keys added to the Go structs remain invisible to users until they delete
// their keybindings.yaml.
func TestMissingKeysWrittenBackToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings.yaml")
	writePartialKeybindings(t, path)

	_, err := util.LoadConfigFile(defaultKB(), path)
	require.NoError(t, err)

	// Re-read the file from disk.
	fileBytes, err := os.ReadFile(path)
	require.NoError(t, err)

	var onDisk KeyBindings
	require.NoError(t, yaml.Unmarshal(fileBytes, &onDisk))

	assert.NotEmpty(t, onDisk.Global.CloseApp.Keys,
		"Global.CloseApp.Keys should be present in keybindings.yaml after merge, but the file was not updated")
	assert.NotEmpty(t, onDisk.Content.PeekRow.Runes,
		"Content.PeekRow.Runes should be present in keybindings.yaml after merge, but the file was not updated")
	assert.NotEmpty(t, onDisk.Schema.FilterBar.Runes,
		"Schema.FilterBar.Runes should be present in keybindings.yaml after merge, but the file was not updated")
}

// TestNewKeyInExistingStructWrittenBackToFile is the file-persistence counterpart
// of TestNewKeyInExistingStructFilledInMemory. It fails for the same reason as
// TestMissingKeysWrittenBackToFile.
func TestNewKeyInExistingStructWrittenBackToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings.yaml")

	partial := KeyBindings{}
	partial.loadDefaults()
	partial.Content.ToggleFilterOptions = Key{}
	data, err := yaml.Marshal(&partial)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))

	_, err = util.LoadConfigFile(defaultKB(), path)
	require.NoError(t, err)

	fileBytes, err := os.ReadFile(path)
	require.NoError(t, err)

	var onDisk KeyBindings
	require.NoError(t, yaml.Unmarshal(fileBytes, &onDisk))

	assert.NotEmpty(t, onDisk.Content.ToggleFilterOptions.Keys,
		"Content.ToggleFilterOptions.Keys should be written back to the file after merge, but the file was not updated")
}
