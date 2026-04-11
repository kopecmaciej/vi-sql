package config

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog/log"
)

//go:embed icons.yaml
var defaultIconsYAML []byte

// SymbolsStyle holds all user-configurable icons/glyphs. It is loaded from
// icons.yaml in the config directory, independent of the color theme. Use
// BetterSymbols=false to fall back to ASCII-safe characters for the tree icons.
type SymbolsStyle struct {
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
}

// TypeSymbol returns the icon for a given SQL data type.
func (s *SymbolsStyle) TypeSymbol(dataType string) string {
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
	case strings.Contains(dt, "bool"):
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

func asciiIcons() *SymbolsStyle {
	return &SymbolsStyle{
		OpenNode:      "▾",
		ClosedNode:    "▸",
		Leaf:          "⊞",
		Separator:     "|",
		TypeTimestamp: "⧗",
		TypeDate:      "▦",
		TypeTime:      "◷",
		TypeNumber:    "~#",
		TypeBool:      "◉",
		TypeJSON:      "{}",
		TypeUUID:      "⊞",
		TypeText:      "T",
		TypeBinary:    "⬡",
		TypeDefault:   "~",
	}
}

func nerdIcons() *SymbolsStyle {
	return &SymbolsStyle{
		OpenNode:      "󰦾 ",
		ClosedNode:    "󰆼 ",
		Leaf:          "󰓫 ",
		Separator:     "|",
		TypeTimestamp: "",
		TypeDate:      "",
		TypeTime:      "",
		TypeNumber:    "",
		TypeBool:      "",
		TypeJSON:      "",
		TypeUUID:      "",
		TypeText:      "",
		TypeBinary:    "",
		TypeDefault:   "",
	}
}

// LoadIcons reads icons from the user config directory, falling back to the
// embedded defaults. When useBetterSymbols is false, all symbol fields are
// replaced with ASCII fallbacks regardless of what the icons file contains.
func LoadIcons(useBetterSymbols bool) (*SymbolsStyle, error) {
	if os.Getenv("ENV") == "vi-dev" {
		if useBetterSymbols {
			return nerdIcons(), nil
		}
		return asciiIcons(), nil
	}

	if err := ExtractIcons(); err != nil {
		return nil, err
	}

	iconsPath, err := getIconsPath()
	if err != nil {
		return nil, err
	}

	// Use nerd icons as merge base so fields added after the user's icons.yaml
	// was created fall back to nerd glyphs rather than empty strings.
	icons, err := util.LoadConfigFile(nerdIcons(), iconsPath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load icons file")
		return nil, fmt.Errorf("failed to load icons file: %w", err)
	}

	if !useBetterSymbols {
		ascii := asciiIcons()
		icons.OpenNode = ascii.OpenNode
		icons.ClosedNode = ascii.ClosedNode
		icons.Leaf = ascii.Leaf
		icons.TypeTimestamp = ascii.TypeTimestamp
		icons.TypeDate = ascii.TypeDate
		icons.TypeTime = ascii.TypeTime
		icons.TypeNumber = ascii.TypeNumber
		icons.TypeBool = ascii.TypeBool
		icons.TypeJSON = ascii.TypeJSON
		icons.TypeUUID = ascii.TypeUUID
		icons.TypeText = ascii.TypeText
		icons.TypeBinary = ascii.TypeBinary
		icons.TypeDefault = ascii.TypeDefault
	}

	return icons, nil
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
