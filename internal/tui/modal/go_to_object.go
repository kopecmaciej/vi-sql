package modal

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

const GoToObjectModalId = "GoToObject"

type GoToObjectModal struct {
	*core.BaseElement
	*core.Flex

	input *core.InputField
}

func NewGoToObjectModal() *GoToObjectModal {
	g := &GoToObjectModal{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		input:       core.NewInputField(),
	}
	g.SetIdentifier(GoToObjectModalId)
	g.SetAfterInitFunc(g.init)
	return g
}

func (g *GoToObjectModal) init() error {
	g.setLayout()
	g.setStyle()
	g.handleEvents()
	return nil
}

func (g *GoToObjectModal) handleEvents() {
	go g.HandleEvents(GoToObjectModalId, func(event manager.EventMsg) {
		if event.Message.Type == manager.StyleChanged {
			g.setStyle()
		}
	})
}

func (g *GoToObjectModal) setLayout() {
	g.Flex.SetDirection(tview.FlexRow)
	g.Flex.SetBorder(true)
	g.Flex.SetTitle(" Go to ")
	g.Flex.AddItem(g.input, 3, 0, true)

	g.input.SetLabel(" > ")
}

func (g *GoToObjectModal) setStyle() {
	styles := g.App.GetStyles()
	g.Flex.SetStyle(styles)
	g.input.SetStyle(styles)
	g.input.SetLabelColor(styles.Global.SecondaryTextColor.Color())
	g.input.SetFieldTextColor(styles.Global.TextColor.Color())

	a := styles.Autocomplete
	background := a.BackgroundColor.Color()
	main := tcell.StyleDefault.Background(background).Foreground(a.TextColor.Color())
	selected := tcell.StyleDefault.Background(a.ActiveBackgroundColor.Color()).Foreground(a.ActiveTextColor.Color())
	second := tcell.StyleDefault.Background(background).Foreground(a.SecondaryTextColor.Color()).Italic(true)
	g.input.SetAutocompleteStyles(background, main, selected, second, false)
	g.input.SetAutocompleteBorderColor(a.BorderColor.Color())
	g.input.InputField.SetAutocompleteMaxHeight(core.AutocompleteMaxItems)
}

// Open displays the modal and wires autocomplete from schemas.
func (g *GoToObjectModal) Open(schemas []database.Schema, title string, getNames func(database.Schema) []string, jumpFunc func(schema, name string) error) {
	k := g.App.GetKeys()
	g.Flex.SetTitle(title)
	g.input.SetText("")

	type pair struct{ schema, name string }
	var pairs []pair
	for _, s := range schemas {
		for _, n := range getNames(s) {
			pairs = append(pairs, pair{s.Schema, n})
		}
	}

	g.input.SetAutocompleteFunc(func(text string) []tview.AutocompleteItem {
		lower := strings.ToLower(text)
		var items []tview.AutocompleteItem
		for _, p := range pairs {
			label := p.schema + "." + p.name
			if lower == "" || strings.Contains(strings.ToLower(label), lower) {
				items = append(items, tview.AutocompleteItem{Main: label})
			}
		}
		return items
	})

	g.input.SetAutocompletedFunc(func(text string, index, source int) bool {
		if source == 0 {
			return false
		}
		g.input.SetText(text)
		return true
	})

	doJump := func() {
		text := strings.TrimSpace(g.input.GetText())
		g.App.Pages.RemovePage(GoToObjectModalId)
		parts := strings.SplitN(text, ".", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return
		}
		if err := jumpFunc(parts[0], parts[1]); err != nil {
			ShowError(g.App.Pages, "Not found", err)
		}
	}

	g.input.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			doJump()
		}
	})

	g.input.SetInputCapture(core.DropdownInputCapture(k, func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Match(k.Common.Confirm, event):
			doJump()
			return nil
		case k.Match(k.Common.Close, event):
			g.App.Pages.RemovePage(GoToObjectModalId)
			return nil
		}
		return event
	}))

	wrapper := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(g.Flex, 5, 0, true).
			AddItem(nil, 0, 1, false), 0, 2, true).
		AddItem(nil, 0, 1, false)

	g.App.Pages.AddPage(GoToObjectModalId, wrapper, true, true)
	g.App.SetFocusOnly(g.input)
}
