package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

const StateFile = "state.yaml"

type connState struct {
	CurrentConnection string      `yaml:"currentConnection"`
	Connections       []SQLConfig `yaml:"connections"`
}

// statePath resolve to $XDG_STATE_HOME or to custom config location if set
func (c *Config) statePath() (string, error) {
	if c.ConfigPath != "" {
		if defaultPath, err := GetConfigPath(); err != nil || c.ConfigPath != defaultPath {
			return filepath.Join(filepath.Dir(c.ConfigPath), StateFile), nil
		}
	}
	return util.GetStatePath()
}

func (c *Config) saveState() error {
	path, err := c.statePath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(connState{
		CurrentConnection: c.CurrentConnection,
		Connections:       c.Connections,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	if err := os.WriteFile(path, data, FileMode); err != nil {
		log.Error().Err(err).Msg("Failed to write state file")
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

func (c *Config) loadState() error {
	path, err := c.statePath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read state file: %w", err)
	}

	var state connState
	if err := yaml.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal state file: %w", err)
	}

	c.CurrentConnection = state.CurrentConnection
	c.Connections = state.Connections
	return nil
}
