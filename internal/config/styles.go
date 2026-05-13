package config

import (
	"embed"
	"fmt"
	"os"
	"strconv"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog/log"
)

//go:embed styles
var stylesFS embed.FS

type (
	Style string

	Styles struct {
		Global       GlobalStyles      `yaml:"global"`
		Icons        IconStyle         `yaml:"-"` // loaded from icons.yaml, not from theme YAML
		Connection   ConnectionStyle   `yaml:"connection"`
		Data         DataStyle         `yaml:"data"`
		TabBar       TabBarStyle       `yaml:"tabBar"`
		Autocomplete AutocompleteStyle `yaml:"autocomplete"`
		Others       OthersStyle       `yaml:"others"`
		SQLEditor    SQLEditorStyle    `yaml:"sqlEditor"`
	}

	GlobalStyles struct {
		BackgroundColor             Style `yaml:"backgroundColor"`
		ContrastBackgroundColor     Style `yaml:"contrastBackgroundColor"`
		MoreContrastBackgroundColor Style `yaml:"moreContrastBackgroundColor"`
		TextColor                   Style `yaml:"textColor"`
		SecondaryTextColor          Style `yaml:"secondaryTextColor"`
		BorderColor                 Style `yaml:"borderColor"`
		FocusColor                  Style `yaml:"focusColor"`
		TitleColor                  Style `yaml:"titleColor"`
		GraphicsColor               Style `yaml:"graphicsColor"`
		DimColor                    Style `yaml:"dimColor"`
		SuccessColor                Style `yaml:"successColor"`
		ErrorColor                  Style `yaml:"errorColor"`
		WarningColor                Style `yaml:"warningColor"`
	}

	ConnectionStyle struct {
		ListTextColor               Style `yaml:"listTextColor"`
		ListSelectedTextColor       Style `yaml:"listSelectedTextColor"`
		ListSelectedBackgroundColor Style `yaml:"listSelectedBackgroundColor"`
		ListSecondaryTextColor      Style `yaml:"listSecondaryTextColor"`
		TableHeaderTextColor        Style `yaml:"tableHeaderTextColor"`
		TableHeaderBackgroundColor  Style `yaml:"tableHeaderBackgroundColor"`
	}

	DataStyle struct {
		CellTextColor         Style `yaml:"cellTextColor"`
		SelectedRowColor      Style `yaml:"selectedRowColor"`
		MultiSelectedRowColor Style `yaml:"multiSelectedRowColor"`
	}

	TabBarStyle struct {
		ActiveTextColor Style `yaml:"activeTextColor"`
	}

	AutocompleteStyle struct {
		BackgroundColor       Style `yaml:"backgroundColor"`
		TextColor             Style `yaml:"textColor"`
		ActiveBackgroundColor Style `yaml:"activeBackgroundColor"`
		ActiveTextColor       Style `yaml:"activeTextColor"`
		SecondaryTextColor    Style `yaml:"secondaryTextColor"`
		BorderColor           Style `yaml:"borderColor"`
	}

	OthersStyle struct {
		LeafIconColor                       Style `yaml:"leafIconColor"`
		SeparatorColor                      Style `yaml:"separatorColor"`
		ButtonsTextColor                    Style `yaml:"buttonsTextColor"`
		DeleteButtonSelectedBackgroundColor Style `yaml:"deleteButtonSelectedBackgroundColor"`
		TableHeaderTextColor                Style `yaml:"tableHeaderTextColor"`
		PeekerHighlightColor                Style `yaml:"peekerHighlightColor"`
	}

	SQLEditorStyle struct {
		KeywordColor    Style `yaml:"keywordColor"`
		StringColor     Style `yaml:"stringColor"`
		NumberColor     Style `yaml:"numberColor"`
		CommentColor    Style `yaml:"commentColor"`
		OperatorColor   Style `yaml:"operatorColor"`
		IdentifierColor Style `yaml:"identifierColor"`
	}
)

func (s *Styles) loadDefaults() {
	s.Global = GlobalStyles{
		BackgroundColor:             "#0F172A",
		ContrastBackgroundColor:     "#1E293B",
		MoreContrastBackgroundColor: "#2563EB",
		TextColor:                   "#E2E8F0",
		SecondaryTextColor:          "#FDE68A",
		BorderColor:                 "#2563EB",
		FocusColor:                  "#60A5FA",
		TitleColor:                  "#2563EB",
		GraphicsColor:               "#2563EB",
		DimColor:                    "#64748B",
		SuccessColor:                "#4ADE80",
		ErrorColor:                  "#F87171",
		WarningColor:                "#FACC15",
	}

	s.Connection = ConnectionStyle{
		ListTextColor:               "#F1FA8C",
		ListSelectedTextColor:       "#F1FA8C",
		ListSelectedBackgroundColor: "#2563EB",
		ListSecondaryTextColor:      "#2563EB",
		TableHeaderTextColor:        "#94A3B8",
		TableHeaderBackgroundColor:  "#1E293B",
	}

	s.Data = DataStyle{
		CellTextColor:         "#2563EB",
		SelectedRowColor:      "#60A5FA",
		MultiSelectedRowColor: "#1D4ED8",
	}

	s.TabBar = TabBarStyle{
		ActiveTextColor: "#FDE68A",
	}

	s.Autocomplete = AutocompleteStyle{
		BackgroundColor:       "#1E293B",
		TextColor:             "#E2E8F0",
		ActiveBackgroundColor: "#60A5FA",
		ActiveTextColor:       "#0F172A",
		SecondaryTextColor:    "#FDE68A",
		BorderColor:           "#60A5FA",
	}

	s.Others = OthersStyle{
		LeafIconColor:                       "#2563EB",
		SeparatorColor:                      "#334155",
		ButtonsTextColor:                    "#FDE68A",
		DeleteButtonSelectedBackgroundColor: "#DA3312",
		TableHeaderTextColor:                "#94A3B8",
		PeekerHighlightColor:                "#3A4963",
	}

	s.SQLEditor = SQLEditorStyle{
		KeywordColor:    "#60A5FA",
		StringColor:     "#4ADE80",
		NumberColor:     "#FB923C",
		CommentColor:    "#64748B",
		OperatorColor:   "#FDE68A",
		IdentifierColor: "#E2E8F0",
	}
}

func LoadStyles(styleName string, useNerdFonts bool) (*Styles, error) {
	defaultStyles := &Styles{}
	defaultStyles.loadDefaults()

	if os.Getenv("ENV") == "vi-dev" {
		icons, err := LoadIcons(useNerdFonts)
		if err != nil {
			return nil, err
		}
		defaultStyles.Icons = *icons
		return defaultStyles, nil
	}

	stylePath, err := getStylePath(styleName)
	if err != nil {
		return nil, err
	}

	if err := ExtractStyles(); err != nil {
		return nil, err
	}

	styles, err := util.LoadConfigFile(defaultStyles, stylePath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load config file")
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}

	icons, err := LoadIcons(useNerdFonts)
	if err != nil {
		return nil, err
	}
	styles.Icons = *icons

	return styles, nil
}

func (s *Styles) LoadMainStyles() {
	tview.Styles.PrimitiveBackgroundColor = s.loadColor(s.Global.BackgroundColor)
	tview.Styles.ContrastBackgroundColor = s.loadColor(s.Global.ContrastBackgroundColor)
	tview.Styles.MoreContrastBackgroundColor = s.loadColor(s.Global.MoreContrastBackgroundColor)
	tview.Styles.PrimaryTextColor = s.loadColor(s.Global.TextColor)
	tview.Styles.SecondaryTextColor = s.loadColor(s.Global.SecondaryTextColor)
	tview.Styles.TertiaryTextColor = s.loadColor(s.Global.SecondaryTextColor)
	tview.Styles.InverseTextColor = s.loadColor(s.Global.SecondaryTextColor)
	tview.Styles.ContrastSecondaryTextColor = s.loadColor(s.Global.SecondaryTextColor)
	tview.Styles.BorderColor = s.loadColor(s.Global.BorderColor)
	tview.Styles.FocusColor = s.loadColor(s.Global.FocusColor)
	tview.Styles.TitleColor = s.loadColor(s.Global.TitleColor)
	tview.Styles.GraphicsColor = s.loadColor(s.Global.GraphicsColor)

	tview.Borders.TopLeft = tview.BoxDrawingsLightArcDownAndRight
	tview.Borders.TopRight = tview.BoxDrawingsLightArcDownAndLeft
	tview.Borders.BottomLeft = tview.BoxDrawingsLightArcUpAndRight
	tview.Borders.BottomRight = tview.BoxDrawingsLightArcUpAndLeft

	tview.Borders.HorizontalFocus = tview.BoxDrawingsLightHorizontal
	tview.Borders.VerticalFocus = tview.BoxDrawingsLightVertical
	tview.Borders.TopLeftFocus = tview.BoxDrawingsLightArcDownAndRight
	tview.Borders.TopRightFocus = tview.BoxDrawingsLightArcDownAndLeft
	tview.Borders.BottomLeftFocus = tview.BoxDrawingsLightArcUpAndRight
	tview.Borders.BottomRightFocus = tview.BoxDrawingsLightArcUpAndLeft
}

func (s *Styles) loadColor(color Style) tcell.Color {
	strColor := string(color)
	if isHexColor(strColor) {
		intColor, _ := strconv.ParseInt(strColor[1:], 16, 32)
		return tcell.NewHexColor(int32(intColor))
	}
	return tcell.GetColor(strColor)
}

func (s *Style) Color() tcell.Color {
	return tcell.GetColor(string(*s))
}

func (s *Style) GetWithColor(color tcell.Color) string {
	return fmt.Sprintf("[%s]%s[%s]", color.String(), s.String(), tcell.ColorReset.String())
}

func (s *Style) String() string {
	return string(*s)
}

func (s *Style) Rune() rune {
	return rune(s.String()[0])
}

func isHexColor(s string) bool {
	return util.IsHexColor(s)
}

func getStylePath(styleName string) (string, error) {
	configPath, err := util.GetConfigDir()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/styles/%s", configPath, styleName), nil
}

func GetAllStyles() ([]string, error) {
	configPath, err := util.GetConfigDir()
	if err != nil {
		return nil, err
	}

	files, err := os.ReadDir(fmt.Sprintf("%s/styles", configPath))
	if err != nil {
		return nil, err
	}

	styleNames := make([]string, 0, len(files))
	for _, file := range files {
		styleNames = append(styleNames, file.Name())
	}
	return styleNames, nil
}

func ExtractStyles() error {
	configDir, err := util.GetConfigDir()
	if err != nil {
		return err
	}

	stylesDir := fmt.Sprintf("%s/styles", configDir)

	if info, err := os.Stat(stylesDir); os.IsNotExist(err) {
		if err := os.MkdirAll(stylesDir, 0755); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if !info.IsDir() {
		return fmt.Errorf("styles path is not a directory: %s", stylesDir)
	}

	entries, err := stylesFS.ReadDir("styles")
	if err != nil {
		log.Error().Err(err).Msg("Failed to read embedded styles directory")
		return fmt.Errorf("failed to read embedded styles directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, err := stylesFS.ReadFile("styles/" + entry.Name())
		if err != nil {
			log.Error().Err(err).Str("File", entry.Name()).Msg("styles: failed to read embedded style file")
			return fmt.Errorf("failed to read embedded style file: %w", err)
		}
		dest := stylesDir + "/" + entry.Name()
		if err := os.WriteFile(dest, content, 0644); err != nil {
			log.Error().Err(err).Str("File", entry.Name()).Msg("styles: failed to write style file")
			return fmt.Errorf("failed to write style file: %w", err)
		}
	}

	return nil
}
