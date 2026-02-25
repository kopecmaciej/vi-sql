package page

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

const (
	HelpPageId = "Help"
)

type Help struct {
	*core.BaseElement
	*core.Table

	style *config.HelpStyle

	keyWidth, descWidth int
}

func NewHelp() *Help {
	h := &Help{
		BaseElement: core.NewBaseElement(),
		Table:       core.NewTable(),
	}

	h.SetIdentifier(HelpPageId)
	h.SetAfterInitFunc(h.init)

	return h
}

func (h *Help) init() error {
	h.setLayout()
	h.setStyle()
	h.setKeybindings()

	h.handleEvents()

	return nil
}

func (h *Help) handleEvents() {
	go h.HandleEvents(HelpPageId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			h.setStyle()
			go h.App.QueueUpdateDraw(func() {
				h.Render()
			})
		}
	})
}

func (h *Help) Render() {
	h.Table.Clear()

	allKeys := h.App.GetKeys().GetAvailableKeys()
	if h.keyWidth == 0 || h.descWidth == 0 {
		h.keyWidth, h.descWidth = h.calculateMaxWidth(allKeys)
	}

	secondRowElements := []config.OrderedKeys{}
	thirdRowElements := []config.OrderedKeys{}
	row := 0
	col := 0
	for _, viewKeys := range allKeys {
		if viewKeys.Element == "Global" || viewKeys.Element == "Help" {
			thirdRowElements = append(thirdRowElements, viewKeys)
		} else if viewKeys.Element == "Welcome" || viewKeys.Element == "Connection" {
			secondRowElements = append(secondRowElements, viewKeys)
		} else {
			h.renderKeySection([]config.OrderedKeys{viewKeys}, &row, col)
		}
	}

	row = 0
	col = 2
	for _, viewKeys := range secondRowElements {
		h.renderKeySection([]config.OrderedKeys{viewKeys}, &row, col)
	}

	row = 0
	col = 4
	for _, viewKeys := range thirdRowElements {
		h.renderKeySection([]config.OrderedKeys{viewKeys}, &row, col)
	}

	h.Table.ScrollToBeginning()
}

func (h *Help) calculateMaxWidth(keys []config.OrderedKeys) (int, int) {
	keyWidth, descWidth := 0, 0
	for _, viewKeys := range keys {
		for _, key := range viewKeys.Keys {
			kw := len(key.Keys)
			if len(key.Runes) > kw {
				kw = len(key.Runes)
			}
			if kw > keyWidth {
				keyWidth = kw
			}
			if len(key.Description) > descWidth {
				descWidth = len(key.Description)
			}
		}
	}
	return keyWidth, descWidth
}

func (h *Help) renderKeySection(keys []config.OrderedKeys, row *int, col int) {
	for _, viewKeys := range keys {
		viewName := viewKeys.Element
		if viewName == "Main" {
			viewName = "Main Layout"
		}
		h.addHeaderSection(viewName, *row, col)
		*row += 2
		h.addKeySection(viewKeys.Keys, row, col)
		*row++
	}
}

func (h *Help) addHeaderSection(name string, row, col int) {
	h.Table.SetCell(row+0, col, tview.NewTableCell(name).SetTextColor(h.style.HeaderColor.Color()))
	h.Table.SetCell(row+1, col, tview.NewTableCell("-------").SetTextColor(h.style.DescriptionColor.Color()))
	h.Table.SetCell(row+0, col+1, tview.NewTableCell("").SetTextColor(h.style.HeaderColor.Color()))
	h.Table.SetCell(row+1, col+1, tview.NewTableCell("").SetTextColor(h.style.DescriptionColor.Color()))
}

func (h *Help) addKeySection(keys []config.Key, row *int, col int) {
	for _, key := range keys {
		var keyString string

		if len(key.Keys) > 0 && len(key.Runes) > 0 {
			keyString = fmt.Sprintf("%s, %s",
				strings.Join(key.Keys, ", "),
				strings.Join(key.Runes, ", "))
		} else if len(key.Keys) > 0 {
			keyString = strings.Join(key.Keys, ", ")
		} else if len(key.Runes) > 0 {
			keyString = strings.Join(key.Runes, ", ")
		}

		h.Table.SetCell(*row, col, tview.NewTableCell(keyString).SetTextColor(h.style.KeyColor.Color()))
		h.Table.SetCell(*row, col+1, tview.NewTableCell(key.Description).SetTextColor(h.style.DescriptionColor.Color()))
		*row++
		h.Table.SetCell(*row, col, tview.NewTableCell(""))
		h.Table.SetCell(*row, col+1, tview.NewTableCell(""))
	}
}

func (h *Help) setStyle() {
	h.style = &h.App.GetStyles().Help
	h.SetStyle(h.App.GetStyles())
	h.Table.SetScrollBarStyle(
		tcell.StyleDefault.Foreground(h.style.ScrollBarThumbColor.Color()),
		tcell.StyleDefault.Foreground(h.style.ScrollBarTrackColor.Color()),
	)
}

func (h *Help) setLayout() {
	h.Table.SetBorder(true)
	h.Table.SetTitle(" Help ")
	h.Table.SetBorderPadding(1, 1, 3, 3)
	h.Table.SetSelectable(false, false)
	h.Table.SetTitleAlign(tview.AlignLeft)
	h.Table.SetEvaluateAllRows(true)
	h.Table.SetScrollBarEnabled(true)
}

func (h *Help) setKeybindings() {
	k := h.App.GetKeys()

	h.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Contains(k.Help.Close, event.Name()):
			h.App.Pages.RemovePage(HelpPageId)
			return nil
		}
		return event
	})
}
