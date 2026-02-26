package component

import (
	"regexp"
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
	defaultText    string
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
	i.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		k := i.App.GetKeys()

		switch {
		case k.Contains(k.QueryBar.ClearInput, event.Name()):
			i.SetText("")
			if i.defaultText != "" {
				go i.SetWordAtCursor(i.defaultText)
			}
		}

		return event
	})
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
	i.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEsc:
			i.Toggle("")
			reject()
		case tcell.KeyEnter:
			i.Toggle("")
			text := i.GetText()
			accept(text)
		}
	})
}

func (i *InputBar) EnableAutocomplete() {
	sqlKeywords := database.SQLKeywords

	i.SetAutocompleteFunc(func(currentText string) (entries []tview.AutocompleteItem) {
		words := strings.Fields(currentText)
		if len(words) == 0 {
			return nil
		}

		currentWord := i.GetWordAtCursor()
		if currentWord == "" {
			return nil
		}

		escaped := regexp.QuoteMeta(currentWord)

		for _, keyword := range sqlKeywords {
			if matched, _ := regexp.MatchString("(?i)^"+escaped, keyword); matched {
				entries = append(entries, tview.AutocompleteItem{Main: keyword})
			}
		}

		if i.columnKeys != nil {
			for _, col := range i.columnKeys {
				if matched, _ := regexp.MatchString("(?i)^"+escaped, col); matched {
					entries = append(entries, tview.AutocompleteItem{Main: col})
				}
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

func (i *InputBar) LoadAutocompleteKeys(keys []string) {
	i.columnKeys = keys
}

func (i *InputBar) Toggle(text string) {
	if i.enabled {
		i.enabled = false
	} else {
		i.enabled = true
	}
	if text == "" {
		text = i.GetText()
	}
	if text == "" && i.defaultText != "" {
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
