package modal

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

const ActionsModalId = "Actions"

// ActionEntry is a single item in the actions modal.
type ActionEntry struct {
	Label    string
	KeyHint  string
	Handler  func()
	Disabled bool
}

// ActionsModal is a curated, fuzzy-filterable list of app-wide actions.
// The filter input is always focused;
type ActionsModal struct {
	*core.BaseElement
	*core.Flex

	filter   *core.InputField
	list     *core.Table
	entries  []ActionEntry
	filtered []ActionEntry
}

func NewActionsModal() *ActionsModal {
	a := &ActionsModal{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		filter:      core.NewInputField(),
		list:        core.NewTable(),
	}
	a.SetIdentifier(ActionsModalId)
	a.SetAfterInitFunc(a.init)
	return a
}

func (a *ActionsModal) init() error {
	a.setLayout()
	a.setStyle()
	a.setKeybindings()
	a.handleEvents()
	return nil
}

func (a *ActionsModal) handleEvents() {
	go a.HandleEvents(ActionsModalId, func(event manager.EventMsg) {
		if event.Message.Type == manager.StyleChanged {
			a.setStyle()
		}
	})
}

func (a *ActionsModal) setLayout() {
	a.Flex.SetDirection(tview.FlexRow)
	a.Flex.SetBorder(true)
	a.Flex.SetTitle(" Actions ")
	a.Flex.AddItem(a.filter, 3, 0, true)
	a.Flex.AddItem(a.list, 0, 1, false)

	a.filter.SetLabel(" > ")
	a.filter.SetBorder(true)

	a.list.SetSelectable(true, false)
	a.list.SetBorderPadding(0, 0, 1, 1)
}

func (a *ActionsModal) setStyle() {
	styles := a.App.GetStyles()
	a.Flex.SetStyle(styles)
	a.filter.SetStyle(styles)
	a.list.SetStyle(styles)
}

func (a *ActionsModal) setKeybindings() {
	k := a.App.GetKeys()
	a.filter.SetChangedFunc(func(text string) {
		a.filterEntries(text)
		a.renderList()
	})

	a.filter.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyDown || k.Match(k.Navigation.FocusDown, event):
			a.moveSelection(1)
			return nil
		case event.Key() == tcell.KeyUp || k.Match(k.Navigation.FocusUp, event):
			a.moveSelection(-1)
			return nil
		case event.Key() == tcell.KeyEnter:
			a.executeSelected()
			return nil
		case event.Key() == tcell.KeyEscape:
			a.App.Pages.RemovePage(ActionsModalId)
			return nil
		}
		return event
	}))
}

func (a *ActionsModal) moveSelection(delta int) {
	if len(a.filtered) == 0 {
		return
	}
	row, _ := a.list.GetSelection()
	next := row + delta
	for next >= 0 && next < len(a.filtered) && a.filtered[next].Disabled {
		next += delta
	}
	if next < 0 || next >= len(a.filtered) {
		return
	}
	a.list.Select(next, 0)
}

func (a *ActionsModal) executeSelected() {
	row, _ := a.list.GetSelection()
	if row < 0 || row >= len(a.filtered) || a.filtered[row].Disabled {
		return
	}
	handler := a.filtered[row].Handler
	a.App.Pages.RemovePage(ActionsModalId)
	if handler != nil {
		handler()
	}
}

func (a *ActionsModal) filterEntries(text string) {
	if text == "" {
		a.filtered = append([]ActionEntry{}, a.entries...)
		return
	}
	lower := strings.ToLower(text)
	a.filtered = a.filtered[:0]
	for _, e := range a.entries {
		if strings.Contains(strings.ToLower(e.Label), lower) {
			a.filtered = append(a.filtered, e)
		}
	}
}

func (a *ActionsModal) renderList() {
	a.list.Clear()
	styles := a.App.GetStyles()
	firstSelectable := -1
	for i, e := range a.filtered {
		textColor := styles.Global.TextColor.Color()
		if e.Disabled {
			textColor = styles.Global.DimColor.Color()
		}
		a.list.SetCell(i, 0, tview.NewTableCell(" "+e.Label).
			SetTextColor(textColor))
		hint := ""
		if e.KeyHint != "" {
			hint = e.KeyHint + " "
		}
		a.list.SetCell(i, 1, tview.NewTableCell(hint).
			SetTextColor(styles.Global.DimColor.Color()).
			SetAlign(tview.AlignRight).
			SetExpansion(1))
		if firstSelectable == -1 && !e.Disabled {
			firstSelectable = i
		}
	}
	if firstSelectable != -1 {
		a.list.Select(firstSelectable, 0)
	}
}

func (a *ActionsModal) Open(entries []ActionEntry) {
	a.entries = entries
	a.filtered = append([]ActionEntry{}, entries...)
	a.filter.SetText("")
	a.renderList()

	wrapper := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(a.Flex, 0, 3, true).
			AddItem(nil, 0, 1, false), 0, 2, true).
		AddItem(nil, 0, 1, false)

	a.App.Pages.AddPage(ActionsModalId, wrapper, true, true)
	a.App.SetFocusOnly(a.filter)
}
