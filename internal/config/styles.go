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
		Global      GlobalStyles     `yaml:"global"`
		Welcome     WelcomeStyle     `yaml:"welcome"`
		Connection  ConnectionStyle  `yaml:"connection"`
		Header      HeaderStyle      `yaml:"header"`
		TabBar      TabBarStyle      `yaml:"tabBar"`
		Schemas     SchemasStyle     `yaml:"schemas"`
		Content     ContentStyle     `yaml:"content"`
		AIPrompt    AIQueryStyle     `yaml:"aiQuery"`
		RowPeeker   RowPeekerStyle   `yaml:"rowPeeker"`
		InputBar    InputBarStyle    `yaml:"inputBar"`
		History     HistoryStyle     `yaml:"history"`
		Help        HelpStyle        `yaml:"help"`
		Others      OthersStyle      `yaml:"others"`
		StyleChange StyleChangeStyle `yaml:"styleChange"`
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
	}

	WelcomeStyle struct {
		FormLabelColor           Style `yaml:"formLabelColor"`
		FormInputColor           Style `yaml:"formInputColor"`
		FormInputBackgroundColor Style `yaml:"formInputBackgroundColor"`
	}

	ConnectionStyle struct {
		FormLabelColor               Style `yaml:"formLabelColor"`
		FormInputBackgroundColor     Style `yaml:"formInputBackgroundColor"`
		FormInputColor               Style `yaml:"formInputColor"`
		ListTextColor                Style `yaml:"listTextColor"`
		ListSelectedTextColor        Style `yaml:"listSelectedTextColor"`
		ListSelectedBackgroundColor  Style `yaml:"listSelectedBackgroundColor"`
		ListSecondaryTextColor       Style `yaml:"listSecondaryTextColor"`
		ListSecondaryBackgroundColor Style `yaml:"listSecondaryBackgroundColor"`
	}

	HeaderStyle struct {
		KeyColor       Style `yaml:"keyColor"`
		ValueColor     Style `yaml:"valueColor"`
		ActiveSymbol   Style `yaml:"activeSymbol"`
		InactiveSymbol Style `yaml:"inactiveSymbol"`
	}

	TabBarStyle struct {
		ActiveTextColor       Style `yaml:"activeTextColor"`
		ActiveBackgroundColor Style `yaml:"activeBackgroundColor"`
	}

	SchemasStyle struct {
		NodeTextColor    Style `yaml:"nodeTextColor"`
		LeafTextColor    Style `yaml:"leafTextColor"`
		NodeSymbolColor  Style `yaml:"nodeSymbolColor"`
		LeafSymbolColor  Style `yaml:"leafSymbolColor"`
		OpenNodeSymbol   Style `yaml:"openNodeSymbol"`
		ClosedNodeSymbol Style `yaml:"closedNodeSymbol"`
		LeafSymbol       Style `yaml:"leafSymbol"`
	}

	ContentStyle struct {
		StatusTextColor          Style `yaml:"statusTextColor"`
		HeaderRowBackgroundColor Style `yaml:"headerRowColor"`
		ColumnKeyColor           Style `yaml:"columnKeyColor"`
		ColumnTypeColor          Style `yaml:"columnTypeColor"`
		CellTextColor            Style `yaml:"cellTextColor"`
		SelectedRowColor         Style `yaml:"selectedRowColor"`
		MultiSelectedRowColor    Style `yaml:"multiSelectedRowColor"`
	}

	RowPeekerStyle struct {
		KeyColor       Style `yaml:"keyColor"`
		ValueColor     Style `yaml:"valueColor"`
		BracketColor   Style `yaml:"bracketColor"`
		HighlightColor Style `yaml:"highlightColor"`
	}

	InputBarStyle struct {
		LabelColor   Style             `yaml:"labelColor"`
		InputColor   Style             `yaml:"inputColor"`
		Autocomplete AutocompleteStyle `yaml:"autocomplete"`
	}

	AutocompleteStyle struct {
		BackgroundColor       Style `yaml:"backgroundColor"`
		TextColor             Style `yaml:"textColor"`
		ActiveBackgroundColor Style `yaml:"activeBackgroundColor"`
		ActiveTextColor       Style `yaml:"activeTextColor"`
		SecondaryTextColor    Style `yaml:"secondaryTextColor"`
	}

	HistoryStyle struct {
		TextColor               Style `yaml:"textColor"`
		SelectedTextColor       Style `yaml:"selectedTextColor"`
		SelectedBackgroundColor Style `yaml:"selectedBackgroundColor"`
	}

	HelpStyle struct {
		HeaderColor         Style `yaml:"headerColor"`
		KeyColor            Style `yaml:"keyColor"`
		DescriptionColor    Style `yaml:"descriptionColor"`
		ScrollBarThumbColor Style `yaml:"scrollBarThumbColor"`
		ScrollBarTrackColor Style `yaml:"scrollBarTrackColor"`
	}

	OthersStyle struct {
		ButtonsTextColor                    Style `yaml:"buttonsTextColor"`
		ButtonsBackgroundColor              Style `yaml:"buttonsBackgroundColor"`
		DeleteButtonSelectedBackgroundColor Style `yaml:"deleteButtonSelectedBackgroundColor"`
		ModalTextColor                      Style `yaml:"modalTextColor"`
		ModalSecondaryTextColor             Style `yaml:"modalSecondaryTextColor"`
		SeparatorSymbol                     Style `yaml:"separatorSymbol"`
		SeparatorColor                      Style `yaml:"separatorColor"`
	}

	StyleChangeStyle struct {
		TextColor               Style `yaml:"textColor"`
		SelectedTextColor       Style `yaml:"selectedTextColor"`
		SelectedBackgroundColor Style `yaml:"selectedBackgroundColor"`
	}

	AIQueryStyle struct {
		FormLabelColor           Style `yaml:"formLabelColor"`
		FormInputBackgroundColor Style `yaml:"formInputBackgroundColor"`
		FormInputColor           Style `yaml:"formInputColor"`
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
	}

	s.Welcome = WelcomeStyle{
		FormLabelColor:           "#FDE68A",
		FormInputColor:           "#E2E8F0",
		FormInputBackgroundColor: "#1E293B",
	}

	s.Connection = ConnectionStyle{
		FormLabelColor:               "#F1FA8C",
		FormInputBackgroundColor:     "#163694",
		FormInputColor:               "#F1FA8C",
		ListTextColor:                "#F1FA8C",
		ListSelectedTextColor:        "#F1FA8C",
		ListSelectedBackgroundColor:  "#2563EB",
		ListSecondaryTextColor:       "#2563EB",
		ListSecondaryBackgroundColor: "#0F172A",
	}

	s.Header = HeaderStyle{
		KeyColor:       "#FDE68A",
		ValueColor:     "#2563EB",
		ActiveSymbol:   "●",
		InactiveSymbol: "○",
	}

	s.TabBar = TabBarStyle{
		ActiveTextColor:       "#FDE68A",
		ActiveBackgroundColor: "#2563EB",
	}

	s.Schemas = SchemasStyle{
		NodeTextColor:    "#2563EB",
		LeafTextColor:    "#E2E8F0",
		NodeSymbolColor:  "#FDE68A",
		LeafSymbolColor:  "#2563EB",
		OpenNodeSymbol:   "▼",
		ClosedNodeSymbol: "▶",
		LeafSymbol:       "◆",
	}

	s.Content = ContentStyle{
		StatusTextColor:          "#FDE68A",
		HeaderRowBackgroundColor: "#1E293B",
		ColumnKeyColor:           "#FDE68A",
		ColumnTypeColor:          "#2563EB",
		CellTextColor:            "#2563EB",
		SelectedRowColor:         "#60A5FA",
		MultiSelectedRowColor:    "#1D4ED8",
	}

	s.RowPeeker = RowPeekerStyle{
		KeyColor:       "#2563EB",
		ValueColor:     "#E2E8F0",
		BracketColor:   "#FDE68A",
		HighlightColor: "#3A4963",
	}

	s.InputBar = InputBarStyle{
		LabelColor: "#FDE68A",
		InputColor: "#E2E8F0",
		Autocomplete: AutocompleteStyle{
			BackgroundColor:       "#1E293B",
			TextColor:             "#E2E8F0",
			ActiveBackgroundColor: "#2563EB",
			ActiveTextColor:       "#0F172A",
			SecondaryTextColor:    "#FDE68A",
		},
	}

	s.History = HistoryStyle{
		TextColor:               "#E2E8F0",
		SelectedTextColor:       "#0F172A",
		SelectedBackgroundColor: "#2563EB",
	}

	s.Help = HelpStyle{
		HeaderColor:         "#2563EB",
		KeyColor:            "#FDE68A",
		DescriptionColor:    "#E2E8F0",
		ScrollBarThumbColor: "#FDE68A",
		ScrollBarTrackColor: "#4A5568",
	}

	s.Others = OthersStyle{
		ButtonsTextColor:                    "#FDE68A",
		ButtonsBackgroundColor:              "#2563EB",
		DeleteButtonSelectedBackgroundColor: "#DA3312",
		ModalTextColor:                      "#FDE68A",
		ModalSecondaryTextColor:             "#2563EB",
		SeparatorSymbol:                     "|",
		SeparatorColor:                      "#334155",
	}

	s.StyleChange = StyleChangeStyle{
		TextColor:               "#E2E8F0",
		SelectedTextColor:       "#0F172A",
		SelectedBackgroundColor: "#2563EB",
	}

	s.AIPrompt = AIQueryStyle{
		FormLabelColor:           "#F1FA8C",
		FormInputBackgroundColor: "#163694",
		FormInputColor:           "#F1FA8C",
	}
}

func SymbolWithColor(symbol Style, color Style) string {
	return fmt.Sprintf("[%s]%s[-:-:-]", color.String(), symbol.String())
}

func LoadStyles(styleName string, useBetterSymbols bool) (*Styles, error) {
	defaultStyles := &Styles{}
	defaultStyles.loadDefaults()

	if os.Getenv("ENV") == "vi-dev" {
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

	if !useBetterSymbols {
		styles.Schemas.OpenNodeSymbol = defaultStyles.Schemas.OpenNodeSymbol
		styles.Schemas.ClosedNodeSymbol = defaultStyles.Schemas.ClosedNodeSymbol
		styles.Schemas.LeafSymbol = defaultStyles.Schemas.LeafSymbol
	}
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

	if info, err := os.Stat(stylesDir); err == nil && info.IsDir() {
		entries, err := os.ReadDir(stylesDir)
		if err != nil {
			return err
		}
		if len(entries) > 0 {
			return nil
		}
	} else if os.IsNotExist(err) {
		if err := os.MkdirAll(stylesDir, 0755); err != nil {
			return err
		}
	} else {
		return err
	}

	entries, err := stylesFS.ReadDir("styles")
	if err != nil {
		log.Error().Err(err).Msg("Failed to read embedded styles directory")
		return fmt.Errorf("failed to read embedded styles directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			content, err := stylesFS.ReadFile("styles/" + entry.Name())
			if err != nil {
				log.Error().Err(err).Str("File", entry.Name()).Msg("styles: failed to read embedded style file")
				return fmt.Errorf("failed to read embedded style file: %w", err)
			}

			err = os.WriteFile(stylesDir+"/"+entry.Name(), content, 0644)
			if err != nil {
				log.Error().Err(err).Str("File", entry.Name()).Msg("styles: failed to write style file")
				return fmt.Errorf("failed to write style file: %w", err)
			}
		}
	}

	return nil
}
