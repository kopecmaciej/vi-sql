package modal

import (
	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

const (
	StyleChangeModalId = "StyleChangeModal"
)

type StyleChangeModal struct {
	*core.BaseElement
	*core.List

	style      *config.StyleChangeStyle
	applyStyle func(styleName string) error
}

func NewStyleChangeModal() *StyleChangeModal {
	sc := &StyleChangeModal{
		BaseElement: core.NewBaseElement(),
		List:        core.NewList(),
	}

	sc.SetIdentifier(StyleChangeModalId)
	sc.SetAfterInitFunc(sc.init)

	return sc
}

func (sc *StyleChangeModal) init() error {
	sc.setLayout()
	sc.setStyle()
	sc.setKeybindings()
	sc.setContent()

	return nil
}

func (sc *StyleChangeModal) setLayout() {
	sc.SetTitle(" Change Style ")
	sc.SetBorder(true)
	sc.ShowSecondaryText(false)
	sc.SetBorderPadding(0, 0, 1, 1)
}

func (sc *StyleChangeModal) setStyle() {
	sc.style = &sc.App.GetStyles().StyleChange
	globalBackground := sc.App.GetStyles().Global.BackgroundColor.Color()

	mainStyle := tcell.StyleDefault.
		Foreground(sc.style.TextColor.Color()).
		Background(globalBackground)
	sc.SetMainTextStyle(mainStyle)

	selectedStyle := tcell.StyleDefault.
		Foreground(sc.style.SelectedTextColor.Color()).
		Background(sc.style.SelectedBackgroundColor.Color())
	sc.SetSelectedStyle(selectedStyle)
}

func (sc *StyleChangeModal) setKeybindings() {
	sc.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape, tcell.KeyCtrlT:
			sc.App.Pages.RemovePage(StyleChangeModalId)
			return nil
		case tcell.KeyEnter:
			sc.App.Pages.RemovePage(StyleChangeModalId)
			text, _ := sc.GetItemText(sc.GetCurrentItem())
			if sc.applyStyle != nil {
				sc.applyStyle(text)
			}
			sc.setStyle()
			return nil
		}
		return event
	})
}

func (sc *StyleChangeModal) setContent() {
	allStyles, err := config.GetAllStyles()
	if err != nil {
		ShowError(sc.App.Pages, "Failed to load styles", err)
		return
	}

	for i, style := range allStyles {
		shortcut := rune('1' + i)
		sc.AddItem(style, "", int32(shortcut), nil)
	}
}

func (sc *StyleChangeModal) SetApplyStyle(applyStyle func(styleName string) error) {
	sc.applyStyle = applyStyle
}

func (sc *StyleChangeModal) Render() {
	modal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(sc.List, 15, 0, true).
			AddItem(nil, 0, 1, false), 40, 0, true).
		AddItem(nil, 0, 1, false)

	sc.App.Pages.AddPage(StyleChangeModalId, modal, true, true)
}
