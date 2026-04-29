package page

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/component"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/widget"
)

const (
	HelpPageId = "Help"
)

// sectionOrder defines the preferred display order for key sections.
// Sections absent from this list are appended at the end.
var sectionOrder = []string{
	"Navigation", "Common", "Global", "Help", "Connection",
	"Main", "Schema", "Data",
	"Peeker", "SQLQueryEditor", "IndexAddForm", "Structure", "History",
}

type Help struct {
	*core.BaseElement
	*core.Flex

	style     *config.HelpStyle
	leftFlex  *core.Flex
	rightFlex *core.Flex

	sectionList *core.List
	keysTable   *core.Table
	footer      *component.Footer
	searchInput *core.InputField
	hints       *widget.Hints

	// capture-based key edit
	capturePanel   *core.Flex
	captureDisplay *core.TextView
	capturedKey    config.Key

	allSections      []config.OrderedKeys
	filteredSections []config.OrderedKeys
	searchMode       bool
	editMode         bool
	editSectionIdx   int
	editKeyIdx       int
}

func NewHelp() *Help {
	h := &Help{
		BaseElement:    core.NewBaseElement(),
		Flex:           core.NewFlex(),
		leftFlex:       core.NewFlex(),
		rightFlex:      core.NewFlex(),
		sectionList:    core.NewList(),
		keysTable:      core.NewTable(),
		footer:         component.NewFooter(),
		searchInput:    core.NewInputField(),
		capturePanel:   core.NewFlex(),
		captureDisplay: core.NewTextView(),
		hints:          widget.NewHints(),
	}

	h.SetIdentifier(HelpPageId)
	h.SetAfterInitFunc(h.init)

	return h
}

func (h *Help) init() error {
	if err := h.footer.Init(h.App); err != nil {
		return err
	}
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
		}
	})
}

func (h *Help) setLayout() {
	h.Flex.SetBorder(true)
	h.Flex.SetTitle(" Help ")
	h.Flex.SetTitleAlign(tview.AlignCenter)
	h.Flex.SetDirection(tview.FlexRow)

	h.leftFlex.SetDirection(tview.FlexRow)
	h.rightFlex.SetDirection(tview.FlexRow)

	h.sectionList.SetTitle(" Sections ")
	h.sectionList.SetBorder(true)
	h.sectionList.ShowSecondaryText(false)
	h.sectionList.SetBorderPadding(0, 0, 1, 0)

	h.keysTable.SetTitle(" Keys ")
	h.keysTable.SetBorder(true)
	h.keysTable.SetBorderPadding(0, 0, 1, 1)
	h.keysTable.SetSelectable(true, false)
	h.keysTable.SetScrollBarEnabled(true)
	h.keysTable.SetEvaluateAllRows(true)

	h.searchInput.SetLabel(" / ")
	h.searchInput.SetBorder(true)

	captureHint := tview.NewTextView()
	captureHint.SetDynamicColors(true)
	captureHint.SetText(" [::d]any key=add  Enter=save  Ctrl+Q=cancel  Backspace=undo[-:-:-]")

	h.captureDisplay.SetDynamicColors(true)
	h.captureDisplay.SetText(" [::d]Press a key combination to bind...[-:-:-]")

	h.capturePanel.SetBorder(true)
	h.capturePanel.SetDirection(tview.FlexRow)
	h.capturePanel.AddItem(h.captureDisplay, 1, 0, true)
	h.capturePanel.AddItem(captureHint, 1, 0, false)
	// height 4 = 2 border rows + 1 display + 1 hint

	contentFlex := tview.NewFlex()
	contentFlex.AddItem(h.leftFlex, 28, 0, true)
	contentFlex.AddItem(h.rightFlex, 0, 1, false)

	h.leftFlex.AddItem(h.sectionList, 0, 1, true)
	h.rightFlex.AddItem(h.keysTable, 0, 1, false)

	h.Flex.AddItem(h.hints, 1, 0, false)
	h.Flex.AddItem(contentFlex, 0, 1, true)
	h.Flex.AddItem(h.footer, 2, 0, false)
}

func (h *Help) setStyle() {
	h.style = &h.App.GetStyles().Help
	s := h.App.GetStyles()
	h.SetStyle(s)
	h.leftFlex.SetStyle(s)
	h.rightFlex.SetStyle(s)
	h.sectionList.SetStyle(s)
	h.keysTable.SetStyle(s)
	h.searchInput.SetStyle(s)
	h.capturePanel.SetStyle(s)
	h.footer.SetStyle(s)
	h.hints.SetStyle(s)

	selectedFg := s.Global.BackgroundColor.Color()
	selectedBg := h.style.SelectedBackgroundColor.Color()

	h.sectionList.SetSelectedStyle(tcell.StyleDefault.
		Foreground(selectedFg).
		Background(selectedBg))

	h.keysTable.SetSelectedStyle(tcell.StyleDefault.
		Foreground(selectedFg).
		Background(selectedBg))

	h.keysTable.SetScrollBarStyle(
		tcell.StyleDefault.Foreground(s.Global.SecondaryTextColor.Color()),
		tcell.StyleDefault.Foreground(h.style.ScrollBarTrackColor.Color()),
	)
}

func (h *Help) setKeybindings() {
	k := h.App.GetKeys()

	h.sectionList.SetChangedFunc(func(index int, _, _ string, _ rune) {
		h.renderKeysForSection(index)
	})

	h.sectionList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Contains(k.Common.Close, event.Name()):
			h.App.Pages.RemovePage(HelpPageId)
			return nil
		case k.Contains(k.Navigation.FocusRight, event.Name()):
			h.App.SetFocusOnly(h.keysTable)
			return nil
		case k.Contains(k.Common.Filter, event.Name()):
			h.enterSearchMode()
			return nil
		case k.Contains(k.Navigation.MoveDown, event.Name()):
			curr := h.sectionList.GetCurrentItem()
			h.sectionList.SetCurrentItem(curr + 1)
			return nil
		case k.Contains(k.Navigation.MoveUp, event.Name()):
			if curr := h.sectionList.GetCurrentItem(); curr > 0 {
				h.sectionList.SetCurrentItem(curr - 1)
			}
			return nil
		}
		return event
	})

	h.keysTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Contains(k.Common.Close, event.Name()):
			h.App.Pages.RemovePage(HelpPageId)
			return nil
		case k.Contains(k.Navigation.FocusLeft, event.Name()):
			h.App.SetFocusOnly(h.sectionList)
			return nil
		case k.Contains(k.Common.Edit, event.Name()):
			row, _ := h.keysTable.GetSelection()
			h.enterEditMode(row)
			return nil
		case k.Contains(k.Navigation.MoveDown, event.Name()):
			row, _ := h.keysTable.GetSelection()
			if row < h.keysTable.GetRowCount()-1 {
				h.keysTable.Select(row+1, 0)
			}
			return nil
		case k.Contains(k.Navigation.MoveUp, event.Name()):
			if row, _ := h.keysTable.GetSelection(); row > 0 {
				h.keysTable.Select(row-1, 0)
			}
			return nil
		}
		return event
	})

	h.searchInput.SetDoneFunc(func(key tcell.Key) {
		h.exitSearchMode(key == tcell.KeyEsc)
	})

	h.searchInput.SetChangedFunc(func(text string) {
		h.filterSections(text)
	})

	h.captureDisplay.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		k := event.Key()
		switch {
		case k == tcell.KeyCtrlQ:
			h.exitEditMode()
		case k == tcell.KeyEnter && event.Modifiers() == tcell.ModNone:
			h.saveEdit()
		case (k == tcell.KeyBackspace || k == tcell.KeyBackspace2 || k == tcell.KeyDelete) && event.Modifiers() == tcell.ModNone:
			if len(h.capturedKey.Runes) > 0 {
				h.capturedKey.Runes = h.capturedKey.Runes[:len(h.capturedKey.Runes)-1]
			} else if len(h.capturedKey.Keys) > 0 {
				h.capturedKey.Keys = h.capturedKey.Keys[:len(h.capturedKey.Keys)-1]
			} else {
				h.capturedKey = config.Key{}
			}
			h.updateCaptureDisplay()
		default:
			captured := eventKeyToConfigKey(event)
			if captured.Keys != nil || captured.Runes != nil {
				h.capturedKey.Keys = append(h.capturedKey.Keys, captured.Keys...)
				h.capturedKey.Runes = append(h.capturedKey.Runes, captured.Runes...)
				h.updateCaptureDisplay()
			}
		}
		return nil
	})
}

func (h *Help) enterSearchMode() {
	h.searchMode = true
	h.leftFlex.AddItem(h.searchInput, 3, 0, true)
	h.App.SetFocusOnly(h.searchInput)
}

func (h *Help) exitSearchMode(reset bool) {
	h.searchMode = false
	h.leftFlex.RemoveItem(h.searchInput)
	if reset {
		h.searchInput.SetText("")
		h.filteredSections = h.allSections
		h.renderSectionList(0)
	}
	h.App.SetFocusOnly(h.sectionList)
}

func (h *Help) enterEditMode(row int) {
	sectionIdx := h.sectionList.GetCurrentItem()
	if sectionIdx >= len(h.filteredSections) {
		return
	}
	section := h.filteredSections[sectionIdx]
	if row < 0 || row >= len(section.Keys) {
		return
	}

	h.editMode = true
	h.editSectionIdx = sectionIdx
	h.editKeyIdx = row
	h.capturedKey = config.Key{}

	desc := section.Keys[row].Description
	h.capturePanel.SetTitle(fmt.Sprintf(" Editing: %s ", desc))
	h.updateCaptureDisplay()

	// Insert capturePanel above keysTable.
	h.rightFlex.RemoveItem(h.keysTable)
	h.rightFlex.AddItem(h.capturePanel, 4, 0, false)
	h.rightFlex.AddItem(h.keysTable, 0, 1, false)
	h.App.SetFocusOnly(h.captureDisplay)
}

func (h *Help) exitEditMode() {
	h.editMode = false
	h.rightFlex.RemoveItem(h.capturePanel)
	h.App.SetFocusOnly(h.keysTable)
}

func (h *Help) saveEdit() {
	if h.capturedKey.Keys == nil && h.capturedKey.Runes == nil && h.capturedKey.Chords == nil {
		h.exitEditMode()
		return
	}
	if h.editSectionIdx >= len(h.filteredSections) {
		h.exitEditMode()
		return
	}
	section := h.filteredSections[h.editSectionIdx]
	if h.editKeyIdx >= len(section.Keys) {
		h.exitEditMode()
		return
	}

	newKey := config.Key{Description: section.Keys[h.editKeyIdx].Description}
	if len(h.capturedKey.Runes) == 2 {
		newKey.Chords = []string{strings.Join(h.capturedKey.Runes, "")}
	} else {
		newKey.Keys = h.capturedKey.Keys
		newKey.Runes = h.capturedKey.Runes
	}

	kb := h.App.GetKeys()
	if err := kb.SetKeyAt(section.Element, h.editKeyIdx, newKey); err != nil {
		h.exitEditMode()
		return
	}
	if err := kb.SaveKeybindings(); err != nil {
		h.exitEditMode()
		return
	}

	row := h.editKeyIdx
	h.exitEditMode()

	h.keysTable.SetCell(row, 0,
		tview.NewTableCell(formatHelpKeyString(newKey)).SetTextColor(h.App.GetStyles().Global.SecondaryTextColor.Color()))
	h.keysTable.Select(row, 0)
}

func (h *Help) updateCaptureDisplay() {
	var parts []string
	parts = append(parts, h.capturedKey.Keys...)
	if len(h.capturedKey.Runes) == 2 {
		parts = append(parts, "<"+strings.Join(h.capturedKey.Runes, "")+">"+" (chord)")
	} else {
		parts = append(parts, h.capturedKey.Runes...)
	}
	if len(parts) == 0 {
		h.captureDisplay.SetText(" [::d]Press a key combination to bind...[-:-:-]")
	} else {
		h.captureDisplay.SetText(fmt.Sprintf(" [::b]► %s[-:-:-]", strings.Join(parts, ", ")))
	}
}

// eventKeyToConfigKey converts a tcell key event to a config.Key entry.
// Alt+rune and Ctrl+rune combos arrive as KeyRune with a modifier bit set;
// tcell KeyNames uses "Ctrl-X" (hyphen) which is normalised to "Ctrl+X" (plus).
func eventKeyToConfigKey(event *tcell.EventKey) config.Key {
	var key config.Key

	if event.Key() == tcell.KeyRune {
		runeName := string(event.Rune())
		if event.Rune() == ' ' {
			runeName = "Space"
		}
		switch {
		case event.Modifiers()&tcell.ModAlt != 0:
			key.Keys = []string{"Alt+" + runeName}
		case event.Modifiers()&tcell.ModCtrl != 0:
			key.Keys = []string{"Ctrl+" + runeName}
		default:
			key.Runes = []string{string(event.Rune())}
		}
		return key
	}

	name, ok := tcell.KeyNames[event.Key()]
	if !ok || name == "" {
		return key
	}
	// Single letter: lowercase (e.g. "Ctrl-L" → "Ctrl+l"). Multi-word: swap separator only.
	if strings.HasPrefix(name, "Ctrl-") {
		if len(name) == 6 {
			name = "Ctrl+" + strings.ToLower(string(name[5]))
		} else {
			name = "Ctrl+" + name[5:]
		}
	}
	// Prefix Alt/Ctrl modifiers for named keys (e.g. Ctrl+Enter, Alt+Escape).
	mods := event.Modifiers()
	switch {
	case mods&tcell.ModAlt != 0 && !strings.HasPrefix(name, "Alt+"):
		name = "Alt+" + name
	case mods&tcell.ModCtrl != 0 && !strings.HasPrefix(name, "Ctrl+"):
		name = "Ctrl+" + name
	}
	key.Keys = []string{name}
	return key
}

func (h *Help) filterSections(query string) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		h.filteredSections = h.allSections
	} else {
		h.filteredSections = nil
		for _, s := range h.allSections {
			if strings.Contains(strings.ToLower(s.Element), query) {
				h.filteredSections = append(h.filteredSections, s)
				continue
			}
			for _, key := range s.Keys {
				if strings.Contains(strings.ToLower(key.Description), query) {
					h.filteredSections = append(h.filteredSections, s)
					break
				}
			}
		}
	}
	h.renderSectionList(0)
	if len(h.filteredSections) > 0 {
		h.renderKeysForSection(0)
	} else {
		h.keysTable.Clear()
	}
}

func (h *Help) Render() {
	keys := h.App.GetKeys()
	h.allSections = h.sortAndFilter(keys.GetAvailableKeys())
	h.filteredSections = h.allSections

	h.renderSectionList(0)
	if len(h.filteredSections) > 0 {
		h.renderKeysForSection(0)
	}
	h.hints.Render(keys, h.App.GetConfig().Styles.NerdFont)
	h.renderFooter()
}

func (h *Help) renderFooter() {
	k := h.App.GetKeys()
	h.footer.SetKeys([]config.Key{
		k.Navigation.FocusRight,
		k.Navigation.FocusLeft,
		k.Common.Filter,
		k.Common.Edit,
		k.Common.Close,
	})
}

func (h *Help) sortAndFilter(keys []config.OrderedKeys) []config.OrderedKeys {
	orderIndex := make(map[string]int, len(sectionOrder))
	for i, name := range sectionOrder {
		orderIndex[name] = i
	}

	known := make([]config.OrderedKeys, len(sectionOrder))
	var unknown []config.OrderedKeys
	for _, ok := range keys {
		if len(ok.Keys) == 0 {
			continue
		}
		if idx, exists := orderIndex[ok.Element]; exists {
			known[idx] = ok
		} else {
			unknown = append(unknown, ok)
		}
	}

	var result []config.OrderedKeys
	for _, ok := range known {
		if ok.Element != "" {
			result = append(result, ok)
		}
	}
	return append(result, unknown...)
}

func (h *Help) renderSectionList(selectIdx int) {
	h.sectionList.Clear()
	for _, s := range h.filteredSections {
		h.sectionList.AddItem(s.Element, "", 0, nil)
	}
	if len(h.filteredSections) > 0 {
		if selectIdx >= len(h.filteredSections) {
			selectIdx = 0
		}
		h.sectionList.SetCurrentItem(selectIdx)
	}
}

func (h *Help) renderKeysForSection(idx int) {
	h.keysTable.Clear()
	if idx >= len(h.filteredSections) {
		return
	}
	section := h.filteredSections[idx]
	for row, key := range section.Keys {
		keyString := formatHelpKeyString(key)
		h.keysTable.SetCell(row, 0,
			tview.NewTableCell(keyString).SetTextColor(h.App.GetStyles().Global.SecondaryTextColor.Color()))
		h.keysTable.SetCell(row, 1,
			tview.NewTableCell(key.Description).SetTextColor(h.App.GetStyles().Global.TextColor.Color()))
	}
	h.keysTable.ScrollToBeginning()
	if h.keysTable.GetRowCount() > 0 {
		h.keysTable.Select(0, 0)
	}
}

func formatHelpKeyString(key config.Key) string {
	var parts []string
	parts = append(parts, key.Keys...)
	parts = append(parts, key.Runes...)
	for _, ch := range key.Chords {
		parts = append(parts, "<"+ch+">")
	}
	return strings.Join(parts, ", ")
}
