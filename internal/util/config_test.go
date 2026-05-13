package util

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsHexColor(t *testing.T) {
	tests := []struct {
		input   string
		isValid bool
	}{
		{"#FF0000", true},
		{"#00ff00", true},
		{"#aabbcc", true},
		{"#123456", true},
		{"#AABBCC", true},
		{"red", false},
		{"FF0000", false},
		{"#GGG000", false},
		{"#12345", false},
		{"#1234567", false},
		{"##123456", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.isValid, IsHexColor(tt.input))
		})
	}
}

type mergeTestConfig struct {
	Name   string
	Port   int
	Tags   []string
	Nested mergeTestNested
}

type mergeTestNested struct {
	Value string
}

func TestMergeConfigs(t *testing.T) {
	t.Run("zero fields filled from default", func(t *testing.T) {
		loaded := &mergeTestConfig{}
		defaults := &mergeTestConfig{Name: "default", Port: 5432, Tags: []string{"a"}, Nested: mergeTestNested{Value: "x"}}
		MergeConfigs(loaded, defaults)
		assert.Equal(t, "default", loaded.Name)
		assert.Equal(t, 5432, loaded.Port)
		assert.Equal(t, []string{"a"}, loaded.Tags)
		assert.Equal(t, "x", loaded.Nested.Value)
	})

	t.Run("non-zero fields not overwritten", func(t *testing.T) {
		loaded := &mergeTestConfig{Name: "custom", Port: 9999, Tags: []string{"b"}, Nested: mergeTestNested{Value: "y"}}
		defaults := &mergeTestConfig{Name: "default", Port: 5432, Tags: []string{"a"}, Nested: mergeTestNested{Value: "x"}}
		MergeConfigs(loaded, defaults)
		assert.Equal(t, "custom", loaded.Name)
		assert.Equal(t, 9999, loaded.Port)
		assert.Equal(t, []string{"b"}, loaded.Tags)
		assert.Equal(t, "y", loaded.Nested.Value)
	})

	t.Run("partial fill", func(t *testing.T) {
		loaded := &mergeTestConfig{Name: "custom"}
		defaults := &mergeTestConfig{Name: "default", Port: 5432}
		MergeConfigs(loaded, defaults)
		assert.Equal(t, "custom", loaded.Name)
		assert.Equal(t, 5432, loaded.Port)
	})
}

func TestValidateConfigPath(t *testing.T) {
	t.Run("empty path is valid", func(t *testing.T) {
		assert.NoError(t, ValidateConfigPath(""))
	})

	t.Run("existing file is valid", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		require.NoError(t, os.WriteFile(path, []byte("key: value"), 0600))
		assert.NoError(t, ValidateConfigPath(path))
	})

	t.Run("path pointing to directory is invalid", func(t *testing.T) {
		dir := t.TempDir()
		assert.Error(t, ValidateConfigPath(dir))
	})

	t.Run("non-existent file in existing directory is valid", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		assert.NoError(t, ValidateConfigPath(path))
	})

	t.Run("non-existent parent directory is invalid", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nonexistent", "config.yaml")
		assert.Error(t, ValidateConfigPath(path))
	})
}
