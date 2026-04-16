package component

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

type InputBar struct {
	*core.BaseElement
	*core.InputField

	style          *config.InputBarStyle
	enabled        bool
	autocompleteOn bool
	columnKeys     []string
	schemas        []database.Schema
	defaultText    string
	acceptFunc     func(string)
	rejectFunc     func()
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
	styles := i.App.GetStyles()
	i.SetStyle(styles)
	i.style = &styles.InputBar
	i.SetLabelColor(styles.Global.SecondaryTextColor.Color())
	i.SetFieldTextColor(styles.Global.TextColor.Color())

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

	i.SetAutocompleteStyles(background, main, selected, second, false)
	i.SetAutocompleteBorderColor(i.style.Autocomplete.BorderColor.Color())
}

func (i *InputBar) setKeybindings() {
	k := i.App.GetKeys()

	inputBarCapture := func(event *tcell.EventKey) *tcell.EventKey {
		k := i.App.GetKeys()

		switch {
		case k.Contains(k.Common.Close, event.Name()):
			if i.rejectFunc != nil {
				i.Toggle("")
				i.rejectFunc()
			}
			return nil
		case k.Contains(k.Common.Execute, event.Name()):
			if i.acceptFunc != nil {
				i.acceptFunc(i.GetText())
			}
			return nil
		case k.Contains(k.Common.Clear, event.Name()):
			i.SetText("")
			if i.defaultText != "" {
				go i.SetWordAtCursor(i.defaultText)
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
		}
	})
}

func (i *InputBar) SetDefaultText(text string) {
	i.defaultText = text
}

func (i *InputBar) DoneFuncHandler(accept func(string), reject func()) {
	i.acceptFunc = accept
	i.rejectFunc = reject
}

func (i *InputBar) EnableAutocomplete() {
	i.SetAutocompleteFunc(func(currentText string) []tview.AutocompleteItem {
		cursorBytePos := len(i.GetTextBeforeCursor())
		entries := database.BuildSQLAutocomplete(currentText, cursorBytePos, i.schemas, i.columnKeys, nil)
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
		return core.SQLTokenStyle(c.tokens, byteOffset, style)
	})
}

// SetSchemas updates the table-name list used by context-aware autocomplete.
func (i *InputBar) SetSchemas(schemas []database.Schema) {
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

// Ctrl+letter names are normalized to uppercase to match tcell.KeyNames.
