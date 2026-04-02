package component

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog/log"
)

type InputBar struct {
	*core.BaseElement
	*core.InputField

	style          *config.InputBarStyle
	enabled        bool
	autocompleteOn bool
	columnKeys     []string
	schemas        []database.SchemaWithTables
	defaultText    string
	historyModal   *modal.History
}

func NewInputBar(barId tview.Identifier, label string) *InputBar {
	i := &InputBar{
		BaseElement:    core.NewBaseElement(),
		InputField:     core.NewInputField(),
		enabled:        false,
		autocompleteOn: false,
	}

	i.InputField.SetLabel(" " + label + ": ")

	i.SetIdentifier(barId)
	i.SetAfterInitFunc(i.init)

	return i
}

func (i *InputBar) init() error {
	i.setStyle()
	i.setKeybindings()
	i.setLayout()

	i.SetClipboard(util.GetClipboard())

	i.handleEvents()

	return nil
}

func (i *InputBar) setLayout() {
	i.SetBorder(true)
}

func (i *InputBar) setStyle() {
	i.SetStyle(i.App.GetStyles())
	i.style = &i.App.GetStyles().InputBar
	i.SetLabelColor(i.style.LabelColor.Color())
	i.SetFieldTextColor(i.style.InputColor.Color())

	a := i.style.Autocomplete
	background := a.BackgroundColor.Color()
	main := tcell.StyleDefault.
		Background(a.BackgroundColor.Color()).
		Foreground(a.TextColor.Color())
	selected := tcell.StyleDefault.
		Background(a.ActiveBackgroundColor.Color()).
		Foreground(a.ActiveTextColor.Color())
	second := tcell.StyleDefault.
		Background(a.BackgroundColor.Color()).
		Foreground(a.SecondaryTextColor.Color()).
		Italic(true)

	i.SetAutocompleteStyles(background, main, selected, second, true)
}

func (i *InputBar) setKeybindings() {
	k := i.App.GetKeys()

	inputBarCapture := func(event *tcell.EventKey) *tcell.EventKey {
		k := i.App.GetKeys()

		switch {
		case k.Contains(k.InputBar.ClearInput, event.Name()):
			i.SetText("")
			if i.defaultText != "" {
				go i.SetWordAtCursor(i.defaultText)
			}
		case k.Contains(k.QueryBar.ShowHistory, event.Name()):
			if i.historyModal != nil {
				i.historyModal.Render()
			}
		}

		return event
	}

	i.SetInputCapture(core.DropdownInputCapture(k, inputBarCapture))
}

func (i *InputBar) handleEvents() {
	go i.HandleEvents(i.GetIdentifier(), func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			i.setStyle()
		default:
			if i.historyModal != nil && event.Sender == modal.HistoryModalId {
				i.handleHistoryEvent(event)
			}
		}
	})
}

func (i *InputBar) handleHistoryEvent(event manager.EventMsg) {
	if event.EventKey == nil {
		return
	}
	keys := i.App.GetKeys()
	switch {
	case keys.Contains(keys.History.AcceptEntry, event.EventKey.Name()):
		go i.App.QueueUpdateDraw(func() {
			i.SetText(i.historyModal.GetText())
			i.App.SetFocus(i)
		})
	case keys.Contains(keys.History.CloseHistory, event.EventKey.Name()):
		go i.App.QueueUpdateDraw(func() {
			i.App.SetFocus(i)
		})
	}
}

// EnableHistory attaches a history modal to this input bar.
func (i *InputBar) EnableHistory() {
	i.historyModal = modal.NewHistoryModal()
	if err := i.historyModal.Init(i.App); err != nil {
		log.Error().Err(err).Msg("Error initializing history modal")
	}
}

// SaveToHistory saves text to the history file (no-op if history is not enabled).
func (i *InputBar) SaveToHistory(text string) error {
	if i.historyModal == nil {
		return nil
	}
	return i.historyModal.SaveToHistory(text)
}

func (i *InputBar) SetDefaultText(text string) {
	i.defaultText = text
}

func (i *InputBar) DoneFuncHandler(accept func(string), reject func()) {
	i.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEsc:
			i.Toggle("")
			reject()
		case tcell.KeyEnter:
			// Do not toggle here — accept is responsible for disabling the bar
			// on success. On failure the bar stays enabled so focus can return to it.
			text := i.GetText()
			accept(text)
		}
	})
}

func (i *InputBar) EnableAutocomplete() {
	i.SetAutocompleteFunc(func(currentText string) []tview.AutocompleteItem {
		cursorBytePos := len(i.GetTextBeforeCursor())
		entries := database.BuildSQLAutocomplete(currentText, cursorBytePos, i.schemas, i.columnKeys)
		items := make([]tview.AutocompleteItem, len(entries))
		for j, e := range entries {
			items[j] = tview.AutocompleteItem{Main: e.Main, Secondary: e.Secondary}
		}
		return items
	})

	i.SetAutocompletedFunc(func(text string, index, source int) bool {
		if source == 0 {
			return false
		}
		before := i.GetTextBeforeCursor()
		after := i.GetText()[len(before):]
		ctx := database.DetectContext(database.Tokenize(i.GetText()), len(before))
		trimmed := strings.TrimSuffix(before, ctx.PartialWord)
		i.SetText(trimmed + text + after)
		i.SetCursorPosition(len(trimmed + text))
		return true
	})
}

// EnableColumnAutocomplete sets up column + keyword autocomplete for simple bars
// (filter, sort). It shows column names immediately and also suggests the provided
// keywords when the current word matches their prefix. No full SQL context detection.
func (i *InputBar) EnableColumnAutocomplete(keywords []string) {
	i.SetAutocompleteFunc(func(currentText string) []tview.AutocompleteItem {
		partial := strings.ToLower(currentText)
		if idx := strings.LastIndexAny(partial, " ,"); idx >= 0 {
			partial = partial[idx+1:]
		}
		var entries []tview.AutocompleteItem
		for _, col := range i.columnKeys {
			if partial == "" || strings.HasPrefix(strings.ToLower(col), partial) {
				entries = append(entries, tview.AutocompleteItem{Main: col})
			}
		}
		for _, kw := range keywords {
			if partial != "" && strings.HasPrefix(strings.ToLower(kw), partial) {
				entries = append(entries, tview.AutocompleteItem{Main: kw})
			}
		}
		return entries
	})

	i.SetAutocompletedFunc(func(text string, index, source int) bool {
		if source == 0 {
			return false
		}
		i.SetWordAtCursor(text)
		return true
	})
}

// EnableHighlighting attaches a syntax-highlighting styleFunc to the underlying
// TextArea. Call again (e.g. on StyleChanged) to update colors.
func (i *InputBar) EnableHighlighting(style *config.SQLEditorStyle) {
	type cache struct {
		text   string
		tokens []database.Token
	}
	var c cache

	i.SetStyleFunc(func(byteOffset int) tcell.Style {
		text := i.GetText()
		if c.text != text {
			c.text = text
			c.tokens = database.Tokenize(text)
		}
		return sqlTokenStyle(c.tokens, byteOffset, style)
	})
}

// SetSchemas updates the table-name list used by context-aware autocomplete.
func (i *InputBar) SetSchemas(schemas []database.SchemaWithTables) {
	i.schemas = schemas
}

func (i *InputBar) LoadAutocompleteKeys(keys []string) {
	i.columnKeys = keys
}

func (i *InputBar) Toggle(text string) {
	if i.enabled {
		i.enabled = false
		return
	}
	i.enabled = true
	if text != "" {
		go i.App.QueueUpdateDraw(func() {
			i.SetText(text)
		})
	} else if i.GetText() == "" && i.defaultText != "" {
		go i.App.QueueUpdateDraw(func() {
			i.SetWordAtCursor(i.defaultText)
		})
	}
}

func (i *InputBar) IsEnabled() bool {
	return i.enabled
}

func (i *InputBar) Enable() {
	i.enabled = true
}

func (i *InputBar) Disable() {
	i.enabled = false
}

// sqlTokenStyle returns a tcell.Style for the token that contains byteOffset.
// It performs a linear scan through tokens (they are sorted by Start).
func sqlTokenStyle(tokens []database.Token, byteOffset int, s *config.SQLEditorStyle) tcell.Style {
	for _, tok := range tokens {
		if tok.Start <= byteOffset && byteOffset < tok.End {
			switch tok.Type {
			case database.TokenKeyword:
				return tcell.StyleDefault.Foreground(s.KeywordColor.Color())
			case database.TokenString:
				return tcell.StyleDefault.Foreground(s.StringColor.Color())
			case database.TokenNumber:
				return tcell.StyleDefault.Foreground(s.NumberColor.Color())
			case database.TokenComment:
				return tcell.StyleDefault.Foreground(s.CommentColor.Color())
			case database.TokenOperator, database.TokenTypecast:
				return tcell.StyleDefault.Foreground(s.OperatorColor.Color())
			default:
				return tcell.StyleDefault.Foreground(s.IdentifierColor.Color())
			}
		}
	}
	return tcell.StyleDefault.Foreground(s.IdentifierColor.Color())
}

// Ctrl+letter names are normalized to uppercase to match tcell.KeyNames.
