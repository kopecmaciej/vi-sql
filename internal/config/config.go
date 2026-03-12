package config

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

const (
	ConfigFile = "config.yaml"
	LogPath    = "/tmp/vi-sql.log"
	FileMode   = 0600
)

var (
	EncryptionKey = ""
)

type SQLOptions struct {
	AlwaysConfirmActions *bool  `yaml:"alwaysConfirmActions,omitempty"`
	DefaultSchema        string `yaml:"defaultSchema,omitempty"`
	Limit                *int64 `yaml:"limit,omitempty"`
}

type SQLConfig struct {
	Driver   string     `yaml:"driver"`
	DSN      string     `yaml:"dsn"`
	Host     string     `yaml:"host"`
	Port     int        `yaml:"port"`
	Database string     `yaml:"database"`
	Username string     `yaml:"username"`
	Password string     `yaml:"password"`
	SSLMode  string     `yaml:"sslMode"`
	Name     string     `yaml:"name"`
	Timeout  int        `yaml:"timeout"`
	Options  SQLOptions `yaml:"options"`
}

// GetDriver returns the configured driver name, defaulting to "postgres".
func (m *SQLConfig) GetDriver() string {
	if m.Driver == "" {
		return "postgres"
	}
	return m.Driver
}

type LogConfig struct {
	Path        string `yaml:"path"`
	Level       string `yaml:"level"`
	PrettyPrint bool   `yaml:"prettyPrint"`
}

type EditorConfig struct {
	Command string `yaml:"command"`
	Env     string `yaml:"env"`
}

type StylesConfig struct {
	BetterSymbols bool   `yaml:"betterSymbols"`
	CurrentStyle  string `yaml:"currentStyle"`
}

type UIConfig struct {
	SchemaPanelWidth int `yaml:"schemaPanelWidth,omitempty"`
}

type Config struct {
	Version            string       `yaml:"version"`
	Log                LogConfig    `yaml:"log"`
	Editor             EditorConfig `yaml:"editor"`
	UI                 UIConfig     `yaml:"ui"`
	ShowConnectionPage bool         `yaml:"showConnectionPage"`
	ShowWelcomePage    bool         `yaml:"showWelcomePage"`
	CurrentConnection  string       `yaml:"currentConnection"`
	Connections        []SQLConfig  `yaml:"connections"`
	Styles             StylesConfig `yaml:"styles"`
	EncryptionKeyPath  *string      `yaml:"encryptionKeyPath,omitempty"`
	JumpInto           string       `yaml:"-"`
	ConfigPath         string       `yaml:"-"`
}

func LoadConfig() (*Config, error) {
	return LoadConfigWithVersion("1.0.0", "")
}

func LoadConfigWithVersion(version string, customPath string) (*Config, error) {
	defaultConfig := &Config{}
	defaultConfig.loadDefaults(version)

	var configPath string
	var err error

	if customPath != "" {
		configPath = customPath
	} else {
		configPath, err = GetConfigPath()
		if err != nil {
			return nil, err
		}
	}

	defaultConfig.ConfigPath = configPath

	cfg, err := util.LoadConfigFile(defaultConfig, configPath)
	if err != nil {
		return nil, err
	}

	cfg.ConfigPath = configPath

	if cfg.Version != version {
		cfg.Version = version
		if err := cfg.UpdateConfig(); err != nil {
			log.Error().Err(err).Msg("Failed to update config with new version")
		}
	}

	return cfg, nil
}

func (c *Config) loadDefaults(version string) {
	c.Version = version
	c.Log = LogConfig{
		Path:        LogPath,
		Level:       "info",
		PrettyPrint: true,
	}
	c.Editor = EditorConfig{
		Command: "",
		Env:     "EDITOR",
	}
	c.UI = UIConfig{
		SchemaPanelWidth: 30,
	}
	c.Styles = StylesConfig{
		BetterSymbols: true,
		CurrentStyle:  "default.yaml",
	}
	c.ShowConnectionPage = true
	c.ShowWelcomePage = true
}

func GetConfigPath() (string, error) {
	configPath, err := util.GetConfigDir()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/%s", configPath, ConfigFile), nil
}

func (c *Config) GetCurrentConfigPath() (string, error) {
	if c.ConfigPath != "" {
		return c.ConfigPath, nil
	}
	return GetConfigPath()
}

func (c *Config) UpdateConfig() error {
	updatedConfig, err := yaml.Marshal(c)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal config")
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath, err := c.GetCurrentConfigPath()
	if err != nil {
		return err
	}

	if err := os.WriteFile(configPath, updatedConfig, FileMode); err != nil {
		log.Error().Err(err).Msg("Failed to write config file")
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (c *Config) GetEditorCmd() (string, error) {
	if c.Editor.Env == "" && c.Editor.Command == "" {
		return "", fmt.Errorf("editor not set")
	}
	if c.Editor.Command != "" {
		return c.Editor.Command, nil
	}

	return os.Getenv(c.Editor.Env), nil
}

func (c *Config) SetCurrentConnection(name string) error {
	c.CurrentConnection = name

	updatedConfig, err := yaml.Marshal(c)
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal config")
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configPath, err := c.GetCurrentConfigPath()
	if err != nil {
		return err
	}

	if err := os.WriteFile(configPath, updatedConfig, FileMode); err != nil {
		log.Error().Err(err).Msg("Failed to write config file")
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (c *Config) GetCurrentConnection() *SQLConfig {
	for _, connection := range c.Connections {
		if connection.Name == c.CurrentConnection {
			return &connection
		}
	}
	return nil
}

func (c *Config) AddConnection(sqlConfig *SQLConfig) error {
	log.Info().Msgf("Adding connection: %s", sqlConfig.Name)
	if c.Connections == nil {
		c.Connections = []SQLConfig{}
	}
	for _, connection := range c.Connections {
		if connection.Name == sqlConfig.Name {
			return fmt.Errorf("connection with name %s already exists", sqlConfig.Name)
		}
	}

	if EncryptionKey != "" && sqlConfig.Password != "" {
		encryptedPass, err := util.EncryptPassword(sqlConfig.Password, EncryptionKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt password: %w", err)
		}
		sqlConfig.Password = encryptedPass
	}

	c.Connections = append(c.Connections, *sqlConfig)

	updatedConfig, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	configPath, err := c.GetCurrentConfigPath()
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, updatedConfig, FileMode)
}

func (c *Config) AddConnectionFromDSN(sqlConfig *SQLConfig) error {
	log.Info().Msgf("Adding connection from DSN: %s", sqlConfig.GetSafeDSN())

	parsed, err := util.ParsePostgresDSN(sqlConfig.DSN)
	if err != nil {
		return err
	}
	sqlConfig.Host = parsed.Host
	sqlConfig.Database = parsed.Database
	sqlConfig.SSLMode = parsed.SSLMode
	if port, err := strconv.Atoi(parsed.Port); err == nil {
		sqlConfig.Port = port
	}
	if parsed.Password != "" && EncryptionKey != "" {
		sqlConfig.Password = parsed.Password
		sqlConfig.DSN = sqlConfig.GetSafeDSN()
	}
	return c.AddConnection(sqlConfig)
}

func (c *Config) DeleteConnection(name string) error {
	log.Info().Msgf("Deleting connection: %s", name)
	for i, connection := range c.Connections {
		if connection.Name == name {
			c.Connections = slices.Delete(c.Connections, i, i+1)
			break
		}
	}

	updatedConfig, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	configPath, err := c.GetCurrentConfigPath()
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, updatedConfig, FileMode)
}

func (c *Config) UpdateConnection(originalName string, sqlConfig *SQLConfig) error {
	found := false
	for i, connection := range c.Connections {
		if connection.Name == originalName {
			if sqlConfig.Password != "" && EncryptionKey != "" {
				encryptedPass, err := util.EncryptPassword(sqlConfig.Password, EncryptionKey)
				if err != nil {
					return fmt.Errorf("failed to encrypt password: %w", err)
				}
				sqlConfig.Password = encryptedPass
			}
			c.Connections[i] = *sqlConfig
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("connection '%s' not found", originalName)
	}

	updatedConfig, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	configPath, err := c.GetCurrentConfigPath()
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, updatedConfig, FileMode)
}

func (c *Config) UpdateConnectionFromDSN(originalName string, sqlConfig *SQLConfig) error {
	parsed, err := util.ParsePostgresDSN(sqlConfig.DSN)
	if err != nil {
		return err
	}
	sqlConfig.Host = parsed.Host
	sqlConfig.Database = parsed.Database
	sqlConfig.SSLMode = parsed.SSLMode
	if port, err := strconv.Atoi(parsed.Port); err == nil {
		sqlConfig.Port = port
	}
	if parsed.Password != "" && EncryptionKey != "" {
		sqlConfig.Password = parsed.Password
		sqlConfig.DSN = sqlConfig.GetSafeDSN()
	}
	return c.UpdateConnection(originalName, sqlConfig)
}

func (c *Config) GetConnectionByName(name string) (*SQLConfig, error) {
	for _, connection := range c.Connections {
		if connection.Name == name {
			conn := connection
			if conn.Password != "" && EncryptionKey != "" {
				decryptedPass, err := util.DecryptPassword(conn.Password, EncryptionKey)
				if err != nil {
					log.Warn().Err(err).Msg("Failed to decrypt password")
				} else {
					conn.Password = decryptedPass
				}
			}
			return &conn, nil
		}
	}
	return nil, fmt.Errorf("connection '%s' not found", name)
}

func (c *Config) LoadEncryptionKey() error {
	if c.EncryptionKeyPath != nil {
		key, err := os.ReadFile(*c.EncryptionKeyPath)
		if err != nil {
			return fmt.Errorf("failed to load encryption key: %s", err)
		}
		EncryptionKey = strings.TrimSpace(string(key))
	} else {
		key := util.GetEncryptionKey()
		if key != "" {
			EncryptionKey = key
		}
	}
	return nil
}

// GetDSN returns the DSN from config. If the stored DSN starts with "$" it is
// treated as an environment-variable reference and expanded via os.ExpandEnv.
func (m *SQLConfig) GetDSN() string {
	if m.DSN != "" {
		if strings.HasPrefix(strings.TrimSpace(m.DSN), "$") {
			return os.ExpandEnv(m.DSN)
		}
		return m.DSN
	}

	switch m.GetDriver() {
	default: // "postgres"
		sslMode := m.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		return util.BuildPostgresDSN(m.Host, m.Port, m.Database, m.Username, m.Password, sslMode)
	}
}

// GetDecryptedDSN returns the DSN with decrypted password.
func (m *SQLConfig) GetDecryptedDSN() string {
	dsn := m.GetDSN()
	if m.DSN != "" || m.Username == "" || m.Password == "" || EncryptionKey == "" {
		return dsn
	}

	decryptedPass, err := util.DecryptPassword(m.Password, EncryptionKey)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decrypt password")
		return dsn
	}

	sslMode := m.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	return util.BuildPostgresDSN(m.Host, m.Port, m.Database, m.Username, decryptedPass, sslMode)
}

// GetSafeDSN returns the DSN with password replaced by asterisks.
func (m *SQLConfig) GetSafeDSN() string {
	dsn := m.GetDSN()
	return util.HidePasswordInDSN(dsn)
}

func (c *SQLConfig) GetOptions() SQLOptions {
	defaults := SQLOptions{
		DefaultSchema: "public",
	}
	if c.Options.DefaultSchema == "" {
		c.Options.DefaultSchema = defaults.DefaultSchema
	}
	if c.Options.AlwaysConfirmActions == nil {
		boolPtr := true
		c.Options.AlwaysConfirmActions = &boolPtr
	}
	return c.Options
}
