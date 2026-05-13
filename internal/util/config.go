package util

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/adrg/xdg"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

const (
	ConfigDir = "vi-sql"
	FileMode  = 0600
)

func MergeConfigs(loaded, defaultConfig any) {
	loadedVal := reflect.ValueOf(loaded).Elem()
	defaultVal := reflect.ValueOf(defaultConfig).Elem()
	mergeConfigsRecursive(loadedVal, defaultVal)
}

func mergeConfigsRecursive(loaded, defaultValue reflect.Value) {
	for i := 0; i < loaded.NumField(); i++ {
		field := loaded.Field(i)
		if !field.CanSet() { // omit unexported fields
			continue
		}
		defaultField := defaultValue.Field(i)

		if field.Type().Name() == "Key" {
			if !isEmptyKey(field) {
				continue
			}
			field.Set(defaultField)
			continue
		}

		switch field.Kind() {
		case reflect.String:
			if field.String() == "" {
				field.Set(defaultField)
			}
		case reflect.Slice:
			if field.Len() == 0 {
				field.Set(defaultField)
			}
		case reflect.Struct:
			mergeConfigsRecursive(field, defaultField)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if field.Int() == 0 {
				field.Set(defaultField)
			}
		}
	}
}

func isEmptyKey(keyValue reflect.Value) bool {
	for i := 0; i < keyValue.NumField(); i++ {
		field := keyValue.Field(i)
		switch field.Kind() {
		case reflect.String:
			if field.String() != "" {
				return false
			}
		case reflect.Slice:
			if field.Len() > 0 {
				return false
			}
		}
	}
	return true
}

func LoadConfigFile[T any](defaultConfig *T, configPath string) (*T, error) {
	err := ensureConfigDirExist()
	if err != nil {
		log.Error().Err(err).Msg("Failed to ensure config directory exists")
		return nil, fmt.Errorf("failed to ensure config directory exists: %w", err)
	}

	bytes, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			bytes, err = yaml.Marshal(defaultConfig)
			if err != nil {
				log.Error().Err(err).Str("path", configPath).Msg("Failed to marshal default config")
				return nil, fmt.Errorf("failed to marshal default config: %w", err)
			}
			err = os.WriteFile(configPath, bytes, FileMode)
			if err != nil {
				log.Error().Err(err).Str("path", configPath).Msg("Failed to write default config file")
				return nil, fmt.Errorf("failed to write default config file: %w", err)
			}
			return defaultConfig, nil
		}
		log.Error().Err(err).Str("path", configPath).Msg("Failed to read config file")
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := new(T)
	if err = yaml.Unmarshal(bytes, config); err != nil {
		log.Error().Err(err).Str("path", configPath).Msg("Failed to unmarshal config file")
		return nil, fmt.Errorf("failed to unmarshal config file: %w", err)
	}

	MergeConfigs(config, defaultConfig)

	// Write merged config back so any fields added to the struct after the file was
	// created appear on disk. Existing user values take priority — MergeConfigs only
	// fills fields that are empty/zero in the loaded config.
	mergedBytes, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal merged config: %w", err)
	}
	if err = os.WriteFile(configPath, mergedBytes, FileMode); err != nil {
		return nil, fmt.Errorf("failed to write merged config: %w", err)
	}

	return config, nil
}

func ensureConfigDirExist() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return os.MkdirAll(configDir, 0755)
	}
	return nil
}

func GetConfigDir() (string, error) {
	configPath, err := xdg.ConfigFile(ConfigDir)
	if err != nil {
		log.Error().Err(err).Msg("Error while getting config path directory")
		return "", err
	}
	return configPath, nil
}

func ValidateConfigPath(configPath string) error {
	if configPath == "" {
		return nil
	}

	fileInfo, err := os.Stat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			dir := filepath.Dir(configPath)
			if dir != "" && dir != "." {
				if _, dirErr := os.Stat(dir); dirErr != nil && os.IsNotExist(dirErr) {
					return fmt.Errorf("config directory does not exist: %s", dir)
				}
			}
			return nil
		}
		return fmt.Errorf("cannot access config file '%s': %w", configPath, err)
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("config path is a directory, not a file: %s", configPath)
	}

	return nil
}

func IsHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
