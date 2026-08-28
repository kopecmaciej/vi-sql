package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

const (
	ConfigFile = "config.yaml"
	FileMode   = 0600
)

var LogPath = filepath.Join(os.TempDir(), "vi-sql.log")

type SQLOptions struct {
	AlwaysConfirmActions *bool  `yaml:"alwaysConfirmActions,omitempty"`
	FetchLimit           *int64 `yaml:"limit,omitempty"`
}

type SQLConfig struct {
	ID       string `yaml:"id"`
	Driver   string `yaml:"driver"`
	DSN      string `yaml:"dsn"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	SSLMode  string `yaml:"sslMode"`
	Name     string `yaml:"name"`
	Timeout  int    `yaml:"timeout"`
	// DriverOptions holds per-driver extras keyed by DSN parameter names.
	DriverOptions map[string]string `yaml:"driverOptions,omitempty"`
	Options       SQLOptions        `yaml:"options"`
	LastUsed      time.Time         `yaml:"lastUsed,omitempty"`
}

type LogConfig struct {
	Path        string `yaml:"path"`
	Level       string `yaml:"level"`
	PrettyPrint bool   `yaml:"prettyPrint"`
}

type EditorConfig struct {
	Command string `yaml:"command"`
	Env     string `yaml:"env"`
	Enabled bool   `yaml:"enabled"` // enables $EDITOR integration; built-in editor is always available
}

type StylesConfig struct {
	NerdFont     bool   `yaml:"nerdFont"`
	CurrentStyle string `yaml:"currentStyle"`
}

type UIConfig struct {
	SchemaPanelWidth int  `yaml:"schemaPanelWidth,omitempty"`
	VimMode          bool `yaml:"vimMode,omitempty"`
	Mouse            bool `yaml:"mouse,omitempty"`
}

type MCPConfig struct {
	Enabled    bool `yaml:"enabled"`
	Port       int  `yaml:"port"`
	AllowRead  bool `yaml:"allowRead"`
	AllowWrite bool `yaml:"allowWrite"`
	MaxRows    int  `yaml:"maxRows,omitempty"`
}

type Config struct {
	Version            string         `yaml:"version"`
	Log                LogConfig      `yaml:"log"`
	Editor             EditorConfig   `yaml:"editor"`
	UI                 UIConfig       `yaml:"ui"`
	MCP                MCPConfig      `yaml:"mcp"`
	Security           SecurityConfig `yaml:"security"`
	ShowConnectionPage bool           `yaml:"showConnectionPage"`
	ShowOptionsPage    bool           `yaml:"-"`
	CurrentConnection  string         `yaml:"-"`
	Connections        []SQLConfig    `yaml:"-"`
	Styles             StylesConfig   `yaml:"styles"`
	JumpInto           string         `yaml:"-"`
	ConfigPath         string         `yaml:"-"`
	FirstLaunch        bool           `yaml:"-"`
	PendingConnect     string         `yaml:"-"`
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

	_, statErr := os.Stat(configPath)
	firstLaunch := os.IsNotExist(statErr)

	statePath, err := defaultConfig.statePath()
	if err != nil {
		return nil, err
	}
	_, statErr = os.Stat(statePath)
	stateExists := statErr == nil

	// Connection used to live in config.yaml, capture it before
	// util.LoadConfigFile rewrites the file
	// TODO: delete in future
	var legacy connState
	if !firstLaunch && !stateExists {
		if b, readErr := os.ReadFile(configPath); readErr == nil {
			_ = yaml.Unmarshal(b, &legacy)
		}
	}

	cfg, err := util.LoadConfigFile(defaultConfig, configPath)
	if err != nil {
		return nil, err
	}

	cfg.ConfigPath = configPath
	cfg.FirstLaunch = firstLaunch

	switch {
	case stateExists:
		if err := cfg.loadState(); err != nil {
			return nil, err
		}
	case len(legacy.Connections) > 0 || legacy.CurrentConnection != "":
		cfg.Connections = legacy.Connections
		cfg.CurrentConnection = legacy.CurrentConnection
		if err := cfg.saveState(); err != nil {
			return nil, err
		}
	}

	if err := cfg.ensureConnectionIDs(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) ensureConnectionIDs() error {
	needsSave := false
	for i := range c.Connections {
		if c.Connections[i].ID == "" {
			c.Connections[i].ID = uuid.New().String()
			needsSave = true
		}
	}
	if needsSave {
		return c.saveState()
	}
	return nil
}

func (c *Config) UpdateVersion(version string) error {
	c.Version = version
	return c.UpdateConfig()
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
		VimMode:          true,
		Mouse:            false,
	}
	c.Styles = StylesConfig{
		NerdFont:     true,
		CurrentStyle: "default.yaml",
	}
	c.ShowConnectionPage = true
	c.ShowOptionsPage = false
	c.Security = SecurityConfig{
		Method: SecurityMethodKeyring,
	}
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

func (c *Config) CopyFrom(other *Config) {
	if other == nil {
		return
	}

	*c = *other
	c.Connections = slices.Clone(other.Connections)
}

// Clone returns a copy of the configuration.
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}

	cloned := &Config{}
	cloned.CopyFrom(c)
	return cloned
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
	for i := range c.Connections {
		if c.Connections[i].Name == name {
			c.Connections[i].LastUsed = time.Now()
			break
		}
	}

	return c.saveState()
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

	if sqlConfig.ID == "" {
		sqlConfig.ID = uuid.New().String()
	}

	if EncryptionKey != "" && sqlConfig.Password != "" && !util.IsEncrypted(sqlConfig.Password) {
		encryptedPass, err := util.EncryptPasswordWithMethod(sqlConfig.Password, EncryptionKey, c.Security.Method)
		if err != nil {
			return fmt.Errorf("failed to encrypt password: %w", err)
		}
		sqlConfig.Password = encryptedPass
	}

	c.Connections = append(c.Connections, *sqlConfig)

	return c.saveState()
}

func (c *Config) AddConnectionFromDSN(sqlConfig *SQLConfig) error {
	log.Info().Msgf("Adding connection from DSN: %s", sqlConfig.GetSafeDSN())

	parsed, err := util.ParsePostgresDSN(sqlConfig.DSN)
	if err != nil {
		return err
	}
	sqlConfig.Host = parsed.Host
	sqlConfig.Username = parsed.Username
	sqlConfig.Database = parsed.Database
	sqlConfig.SSLMode = parsed.SSLMode
	if port, err := strconv.Atoi(parsed.Port); err == nil {
		sqlConfig.Port = port
	}
	if parsed.Password != "" {
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

	return c.saveState()
}

func (c *Config) UpdateConnection(originalName string, sqlConfig *SQLConfig) error {
	found := false
	for i, connection := range c.Connections {
		if connection.Name == originalName {
			if sqlConfig.Password != "" && EncryptionKey != "" && !util.IsEncrypted(sqlConfig.Password) {
				encryptedPass, err := util.EncryptPasswordWithMethod(sqlConfig.Password, EncryptionKey, c.Security.Method)
				if err != nil {
					return fmt.Errorf("failed to encrypt password: %w", err)
				}
				sqlConfig.Password = encryptedPass
			}
			// A config rebuilt from the edit form does not carry over row
			// identity or usage tracking — preserve them on update.
			if sqlConfig.ID == "" {
				sqlConfig.ID = connection.ID
			}
			if sqlConfig.LastUsed.IsZero() {
				sqlConfig.LastUsed = connection.LastUsed
			}
			c.Connections[i] = *sqlConfig
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("connection '%s' not found", originalName)
	}

	return c.saveState()
}

func (c *Config) UpdateConnectionFromDSN(originalName string, sqlConfig *SQLConfig) error {
	parsed, err := util.ParsePostgresDSN(sqlConfig.DSN)
	if err != nil {
		return err
	}
	sqlConfig.Host = parsed.Host
	sqlConfig.Username = parsed.Username
	sqlConfig.Database = parsed.Database
	sqlConfig.SSLMode = parsed.SSLMode
	if port, err := strconv.Atoi(parsed.Port); err == nil {
		sqlConfig.Port = port
	}
	if parsed.Password != "" {
		sqlConfig.Password = parsed.Password
		sqlConfig.DSN = sqlConfig.GetSafeDSN()
	}
	return c.UpdateConnection(originalName, sqlConfig)
}

func (c *Config) GetConnectionByName(name string) (*SQLConfig, error) {
	for _, connection := range c.Connections {
		if connection.Name == name {
			conn := connection
			if util.IsEncrypted(conn.Password) && EncryptionKey != "" {
				decryptedPass, _, err := util.DecryptPasswordWithMethod(conn.Password, EncryptionKey)
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

func (m *SQLConfig) GetDriver() string {
	if m.Driver == "" {
		return "postgres"
	}
	return m.Driver
}

func (m *SQLConfig) GetDriverOption(key string) string {
	return m.DriverOptions[key]
}

func (m *SQLConfig) SetDriverOption(key, value string) {
	if m.DriverOptions == nil {
		m.DriverOptions = make(map[string]string)
	}
	m.DriverOptions[key] = value
}

func (m *SQLConfig) IsPasswordReadable() bool {
	if !util.IsEncrypted(m.Password) {
		return true
	}
	if EncryptionKey == "" {
		return false
	}
	_, _, err := util.DecryptPasswordWithMethod(m.Password, EncryptionKey)
	return err == nil
}

func (m *SQLConfig) buildDSNFromFields(password string) string {
	switch m.GetDriver() {
	case "mysql", "mariadb":
		params := map[string]string{}
		if m.SSLMode != "" && m.SSLMode != "false" {
			params["tls"] = m.SSLMode
		}
		return util.BuildDSN(m.GetDriver(), m.Host, m.Port, m.Database, m.Username, password, params)
	case "sqlserver":
		encrypt := m.SSLMode
		if encrypt == "" {
			encrypt = "disable"
		}
		return util.BuildSQLServerDSN(m.Host, m.Port, m.Database, m.Username, password, encrypt)
	case "oracle":
		return util.BuildOracleDSN(m.Host, m.Port, m.Database, m.Username, password)
	case "gaussdb":
		return util.BuildGaussDBDSN(m.Host, m.Port, m.Database, m.Username, password, m.DriverOptions)
	default: // postgres
		sslMode := m.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		return util.BuildDSN("postgres", m.Host, m.Port, m.Database, m.Username, password, map[string]string{"sslmode": sslMode})
	}
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
	return m.buildDSNFromFields(m.Password)
}

// GetDecryptedDSN returns the DSN with decrypted password.
func (m *SQLConfig) GetDecryptedDSN() string {
	dsn := m.GetDSN()
	if util.IsMultiHostDSN(dsn) && m.Password != "" {
		pw := m.Password
		if util.IsEncrypted(pw) && EncryptionKey != "" {
			if dec, _, err := util.DecryptPasswordWithMethod(pw, EncryptionKey); err == nil {
				pw = dec
			}
		}
		return util.InjectDSNPassword(dsn, pw)
	}
	if m.DSN != "" || m.Username == "" || !util.IsEncrypted(m.Password) || EncryptionKey == "" {
		return dsn
	}

	decryptedPass, _, err := util.DecryptPasswordWithMethod(m.Password, EncryptionKey)
	if err != nil {
		log.Error().Err(err).Msg("Failed to decrypt password")
		return dsn
	}

	return m.buildDSNFromFields(decryptedPass)
}

// GetSafeDSN returns the DSN with password replaced by asterisks.
func (m *SQLConfig) GetSafeDSN() string {
	masked, _ := util.SplitDSNPassword(m.GetDSN())
	return masked
}

func (c *SQLConfig) GetOptions() SQLOptions {
	if c.Options.AlwaysConfirmActions == nil {
		boolPtr := true
		c.Options.AlwaysConfirmActions = &boolPtr
	}
	return c.Options
}
