package config

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

//go:embed icons.yaml
var defaultIconsYAML []byte

// IconsConfig holds both nerd-font and ASCII icon sets. It is the top-level
// structure of icons.yaml.
type IconsConfig struct {
	Nerd  IconStyle `yaml:"nerd"`
	Ascii IconStyle `yaml:"ascii"`
}

// IconStyle holds all user-configurable icons/glyphs. The active set is
// chosen by the nerdFont config flag.
type IconStyle struct {
	NerdFont bool `yaml:"-"` // set at load time, not persisted

	OpenNode      Style `yaml:"openNode"`
	ClosedNode    Style `yaml:"closedNode"`
	Leaf          Style `yaml:"leaf"`
	Separator     Style `yaml:"separator"`
	TypeTimestamp Style `yaml:"typeTimestamp"`
	TypeDate      Style `yaml:"typeDate"`
	TypeTime      Style `yaml:"typeTime"`
	TypeNumber    Style `yaml:"typeNumber"`
	TypeBool      Style `yaml:"typeBool"`
	TypeJSON      Style `yaml:"typeJSON"`
	TypeUUID      Style `yaml:"typeUUID"`
	TypeText      Style `yaml:"typeText"`
	TypeBinary    Style `yaml:"typeBinary"`
	TypeDefault   Style `yaml:"typeDefault"`
	HealthUp      Style `yaml:"healthUp"`
	HealthDown    Style `yaml:"healthDown"`
	VimMode       Style `yaml:"vimMode"`
	MCP           Style `yaml:"mcp"`
	Update        Style `yaml:"update"`
	TabTable      Style `yaml:"tabTable"`
	TabQuery      Style `yaml:"tabQuery"`
	TabStructure  Style `yaml:"tabStructure"`
	TabIndex      Style `yaml:"tabIndex"`
	View          Style `yaml:"view"`

	PrimaryKey         Style `yaml:"primaryKey"`
	ForeignKey         Style `yaml:"foreignKey"`
	CompletionKeyword  Style `yaml:"completionKeyword"`
	CompletionCTE      Style `yaml:"completionCTE"`
	CompletionAlias    Style `yaml:"completionAlias"`
	CompletionFunction Style `yaml:"completionFunction"`

	DriverPostgres    Style `yaml:"driverPostgres"`
	DriverMySQL       Style `yaml:"driverMySQL"`
	DriverMariaDB     Style `yaml:"driverMariaDB"`
	DriverSQLite      Style `yaml:"driverSQLite"`
	DriverCockroachDB Style `yaml:"driverCockroachDB"`
	DriverGaussDB     Style `yaml:"driverGaussDB"`
	DriverDefault     Style `yaml:"driverDefault"`
}

func (s *IconStyle) DriverIcon(driver string) string {
	switch driver {
	case "postgres":
		return string(s.DriverPostgres)
	case "mysql":
		return string(s.DriverMySQL)
	case "mariadb":
		return string(s.DriverMariaDB)
	case "sqlite":
		return string(s.DriverSQLite)
	case "cockroachdb":
		return string(s.DriverCockroachDB)
	case "gaussdb":
		return string(s.DriverGaussDB)
	default:
		return string(s.DriverDefault)
	}
}

// TypeSymbol returns the icon for a given SQL data type.
func (s *IconStyle) TypeSymbol(dataType string) string {
	dt := strings.ToLower(dataType)
	switch {
	case strings.Contains(dt, "timestamp"):
		return string(s.TypeTimestamp)
	case strings.Contains(dt, "date"):
		return string(s.TypeDate)
	case strings.Contains(dt, "time"):
		return string(s.TypeTime)
	case strings.Contains(dt, "int") || strings.Contains(dt, "serial") ||
		strings.Contains(dt, "numeric") || strings.Contains(dt, "decimal") ||
		strings.Contains(dt, "real") || strings.Contains(dt, "double") ||
		strings.Contains(dt, "float") || strings.Contains(dt, "money"):
		return string(s.TypeNumber)
	case strings.Contains(dt, "bool") || dt == "bit":
		return string(s.TypeBool)
	case strings.Contains(dt, "json"):
		return string(s.TypeJSON)
	case strings.Contains(dt, "uuid"):
		return string(s.TypeUUID)
	case strings.Contains(dt, "char") || strings.Contains(dt, "text") ||
		strings.Contains(dt, "varchar") || dt == "name":
		return string(s.TypeText)
	case strings.Contains(dt, "byte") || strings.Contains(dt, "binary") ||
		strings.Contains(dt, "blob"):
		return string(s.TypeBinary)
	default:
		return string(s.TypeDefault)
	}
}

func embeddedIcons() (*IconsConfig, error) {
	cfg := &IconsConfig{}
	if err := yaml.Unmarshal(defaultIconsYAML, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse embedded icons: %w", err)
	}
	return cfg, nil
}

func pickIcons(cfg *IconsConfig, nerdFont bool) *IconStyle {
	var s IconStyle
	if nerdFont {
		s = cfg.Nerd
	} else {
		s = cfg.Ascii
	}
	s.NerdFont = nerdFont
	return &s
}

func (s *IconStyle) spacing() string {
	if s.NerdFont {
		return "  "
	}
	return " "
}

// Icon returns the glyph with appropriate trailing space — two spaces for nerd
// font glyphs (which render double-wide) and one for ASCII.
func (s *IconStyle) Icon(icon Style) string {
	return string(icon) + s.spacing()
}

// IconWithColor wraps the glyph in a tview color tag with appropriate spacing.
func (s *IconStyle) IconWithColor(icon Style, color Style) string {
	return fmt.Sprintf("[%s]%s[-:-:-]%s", color.String(), icon.String(), s.spacing())
}

// LoadIcons reads icons from the user config directory, falling back to the
// embedded defaults. nerdFont selects the nerd or ascii symbol set.
func LoadIcons(nerdFont bool) (*IconStyle, error) {
	defaults, err := embeddedIcons()
	if err != nil {
		return nil, err
	}

	if os.Getenv("ENV") == "vi-dev" {
		return pickIcons(defaults, nerdFont), nil
	}

	if err := ExtractIcons(); err != nil {
		return nil, err
	}

	iconsPath, err := getIconsPath()
	if err != nil {
		return nil, err
	}

	icons, err := util.LoadConfigFile(defaults, iconsPath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load icons file")
		return nil, fmt.Errorf("failed to load icons file: %w", err)
	}

	return pickIcons(icons, nerdFont), nil
}

// ExtractIcons copies the embedded icons.yaml to the user config directory
// if it does not already exist there.
func ExtractIcons() error {
	iconsPath, err := getIconsPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(iconsPath); err == nil {
		return nil // already exists
	}

	if err := os.WriteFile(iconsPath, defaultIconsYAML, 0644); err != nil {
		log.Error().Err(err).Msg("icons: failed to write icons file")
		return fmt.Errorf("failed to write icons file: %w", err)
	}

	return nil
}

func getIconsPath() (string, error) {
	configDir, err := util.GetConfigDir()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/icons.yaml", configDir), nil
}
