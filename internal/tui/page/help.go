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

	VimMotionsSectionName = "Vim Motions"
)

// sectionForFocusID maps a focused component's identifier to its help section name.
func sectionForFocusID(id string) string {
	switch {
	case strings.HasSuffix(id, component.EditorSuffix):
		return VimMotionsSectionName
	case strings.HasPrefix(id, "QueryTab-"):
		return "Data"
	case strings.HasPrefix(id, string(component.PeekerId)):
		return "Peeker"
	case id == component.IndexId:
		return "IndexAddForm"
	case id == component.SQLQueryEditorId:
		return VimMotionsSectionName
	}
	return id
}

// sectionOrder defines the display order; unlisted sections are appended last.
var sectionOrder = []string{
	"Navigation", "Common", "Global", "Help", "Connection",
	"Main", "Schema", "Data",
	"Peeker", "SQLQueryEditor", "IndexAddForm", "Structure", "History", "ExplainViewer", VimMotionsSectionName,
}

// vimEditorKeys is a static, non-editable section documenting the built-in vim
// motions available in the SQL query editor. It is injected at render time so
// it appears in the help page without polluting KeyBindings or the YAML config.
var vimEditorKeys = config.OrderedKeys{
	Element: VimMotionsSectionName,
	Keys: []config.Key{
		{Keys: []string{"Esc"}, Description: "enter normal mode"},
		{Keys: []string{"i / a / I / A / o / O"}, Description: "enter insert mode"},
		{Keys: []string{"v"}, Description: "enter visual select"},
		{Keys: []string{"h / j / k / l"}, Description: "move ← ↓ ↑ →"},
		{Keys: []string{"w / e / b"}, Description: "word forward / end / backward"},
		{Keys: []string{"0 / ^ / $"}, Description: "line start / first non-blank / end"},
		{Sequences: []string{"gg"}, Description: "go to first line"},
		{Keys: []string{"G"}, Description: "go to last line"},
		{Keys: []string{"d / c / y + motion"}, Description: "delete / change / yank"},
		{Keys: []string{"dd / cc / yy"}, Description: "delete / change / yank line"},
		{Keys: []string{"r"}, Description: "replace char under cursor"},
		{Keys: []string{"f / F / t / T"}, Description: "find char on line"},
		{Keys: []string{"x / s"}, Description: "delete char / delete and enter insert"},
		{Keys: []string{"p / P"}, Description: "paste after / before cursor"},
		{Keys: []string{"u"}, Description: "undo"},
	},
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
	// dataRowMap maps visual row index → section.Keys index for the Data section.
	// -1 marks the separator row. Nil when Data is not the active section.
	dataRowMap []int
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
	captureHint.SetText(" [::d]any key=add  2 runes=sequence  Enter=save  Ctrl+Q=cancel  Backspace=undo[-:-:-]")

	h.captureDisplay.SetDynamicColors(true)
	h.captureDisplay.SetText(" [::d]Press a key combination to bind...[-:-:-]")

	h.capturePanel.SetBorder(true)
	h.capturePanel.SetDirection(tview.FlexRow)
	h.capturePanel.AddItem(h.captureDisplay, 1, 0, true)
	h.capturePanel.AddItem(captureHint, 1, 0, false)

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

	h.sectionList.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Match(k.Common.Close, event):
			h.App.Pages.RemovePage(HelpPageId)
			return nil
		case k.Match(k.Navigation.FocusRight, event):
			h.App.SetFocusOnly(h.keysTable)
			return nil
		case k.Match(k.Common.Filter, event):
			h.enterSearchMode()
			return nil
		case k.Match(k.Navigation.MoveDown, event):
			curr := h.sectionList.GetCurrentItem()
			vim := h.vimMotionsIndex()
			next := curr + 1
			if vim > 0 && next == vim-1 {
				next = vim // skip separator
			}
			if next < h.sectionList.GetItemCount() {
				h.sectionList.SetCurrentItem(next)
			}
			return nil
		case k.Match(k.Navigation.MoveUp, event):
			curr := h.sectionList.GetCurrentItem()
			vim := h.vimMotionsIndex()
			prev := curr - 1
			if vim > 0 && curr == vim {
				prev = vim - 2 // skip separator
			}
			if prev >= 0 {
				h.sectionList.SetCurrentItem(prev)
			}
			return nil
		}
		return event
	}))

	h.keysTable.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Match(k.Common.Close, event):
			h.App.Pages.RemovePage(HelpPageId)
			return nil
		case k.Match(k.Navigation.FocusLeft, event):
			h.App.SetFocusOnly(h.sectionList)
			return nil
		case k.Match(k.Common.Edit, event):
			row, _ := h.keysTable.GetSelection()
			h.enterEditMode(row)
			return nil
		case k.Match(k.Navigation.MoveDown, event):
			row, _ := h.keysTable.GetSelection()
			next := row + 1
			if next < len(h.dataRowMap) && h.dataRowMap[next] < 0 {
				next++ // skip separator
			}
			if next < h.keysTable.GetRowCount() {
				h.keysTable.Select(next, 0)
			}
			return nil
		case k.Match(k.Navigation.MoveUp, event):
			row, _ := h.keysTable.GetSelection()
			prev := row - 1
			if prev >= 0 && prev < len(h.dataRowMap) && h.dataRowMap[prev] < 0 {
				prev-- // skip separator
			}
			if prev >= 0 {
				h.keysTable.Select(prev, 0)
			}
			return nil
		}
		return event
	}))

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
	listIdx := h.sectionList.GetCurrentItem()
	vim := h.vimMotionsIndex()
	if vim > 0 && listIdx >= vim-1 {
		return // separator or vim motions — not editable
	}
	if listIdx >= len(h.filteredSections) || row < 0 {
		return
	}

	section := h.filteredSections[listIdx]
	keyIdx := row
	if h.dataRowMap != nil && section.Element == "Data" {
		if row >= len(h.dataRowMap) || h.dataRowMap[row] < 0 {
			return // separator row
		}
		keyIdx = h.dataRowMap[row]
	} else if row >= len(section.Keys) {
		return
	}

	h.editMode = true
	h.editSectionIdx = listIdx
	h.editKeyIdx = keyIdx
	h.capturedKey = config.Key{}

	desc := section.Keys[keyIdx].Description
	h.capturePanel.SetTitle(fmt.Sprintf(" Editing: %s ", desc))
	h.updateCaptureDisplay()

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
	if h.capturedKey.Keys == nil && h.capturedKey.Runes == nil && h.capturedKey.Sequences == nil {
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
		newKey.Sequences = []string{strings.Join(h.capturedKey.Runes, "")}
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

	keyIdx := h.editKeyIdx
	h.exitEditMode()

	visualRow := keyIdx
	for vr, si := range h.dataRowMap {
		if si == keyIdx {
			visualRow = vr
			break
		}
	}
	s := h.App.GetStyles()
	h.keysTable.SetCell(visualRow, 0,
		tview.NewTableCell(formatHelpKeyString(newKey)).SetStyle(tcell.StyleDefault.
			Foreground(s.Global.SecondaryTextColor.Color()).
			Background(s.Global.BackgroundColor.Color())))
	h.keysTable.Select(visualRow, 0)
}

func (h *Help) updateCaptureDisplay() {
	var parts []string
	parts = append(parts, h.capturedKey.Keys...)
	if len(h.capturedKey.Runes) == 2 {
		parts = append(parts, "<"+strings.Join(h.capturedKey.Runes, "")+">"+" (sequence)")
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
	var filtered []config.OrderedKeys
	if query == "" {
		filtered = h.allSections
	} else {
		for _, s := range h.allSections {
			if strings.Contains(strings.ToLower(s.Element), query) {
				filtered = append(filtered, s)
				continue
			}
			for _, key := range s.Keys {
				if strings.Contains(strings.ToLower(key.Description), query) {
					filtered = append(filtered, s)
					break
				}
			}
		}
	}
	h.filteredSections = filtered
	h.renderSectionList(0)
	if len(h.filteredSections) > 0 {
		h.renderKeysForSection(0)
	} else {
		h.keysTable.Clear()
	}
}

func (h *Help) Render() {
	h.render("")
}

// OpenAt renders the help page, pre-selecting the section that corresponds to
// the given focused component identifier.
func (h *Help) OpenAt(focusID string) {
	h.render(sectionForFocusID(focusID))
}

func (h *Help) render(startSection string) {
	keys := h.App.GetKeys()
	allSections := append(keys.GetAvailableKeys(), vimEditorKeys)
	h.allSections = h.sortAndFilter(allSections)
	h.filteredSections = h.allSections

	startListIdx := 0
	if startSection != "" {
		vim := h.vimMotionsIndex()
		for i, s := range h.filteredSections {
			if s.Element == startSection {
				startListIdx = i
				if vim > 0 && i == len(h.filteredSections)-1 {
					startListIdx = vim // vim section sits after the separator in the list
				}
				break
			}
		}
	}
	h.renderSectionList(startListIdx)
	if len(h.filteredSections) > 0 {
		h.renderKeysForSection(startListIdx)
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

func (h *Help) vimMotionsIndex() int {
	n := len(h.filteredSections)
	if n > 0 && h.filteredSections[n-1].Element == VimMotionsSectionName {
		return n
	}
	return -1
}

func (h *Help) renderSectionList(selectListIdx int) {
	h.sectionList.Clear()
	vim := h.vimMotionsIndex()
	for i, s := range h.filteredSections {
		if vim > 0 && i == len(h.filteredSections)-1 {
			h.sectionList.AddItem("──────────────────────────", "", 0, nil)
		}
		h.sectionList.AddItem(s.Element, "", 0, nil)
	}
	total := h.sectionList.GetItemCount()
	if total > 0 {
		if selectListIdx >= total {
			selectListIdx = 0
		}
		h.sectionList.SetCurrentItem(selectListIdx)
	}
}

func (h *Help) renderKeysForSection(listIdx int) {
	h.keysTable.Clear()
	h.dataRowMap = nil
	vim := h.vimMotionsIndex()

	// Separator selected — clear table.
	if vim > 0 && listIdx == vim-1 {
		h.keysTable.SetTitle(" Keys ")
		return
	}

	// Remap vim list index to filteredSections index.
	sectionIdx := listIdx
	if vim > 0 && listIdx == vim {
		sectionIdx = vim - 1
	}

	if sectionIdx >= len(h.filteredSections) {
		return
	}
	section := h.filteredSections[sectionIdx]
	styles := h.App.GetStyles()

	if section.Element == "Data" {
		h.renderDataSection(section)
		return
	}

	bg := styles.Global.BackgroundColor.Color()
	keyStyle := tcell.StyleDefault.Foreground(styles.Global.SecondaryTextColor.Color()).Background(bg)
	descStyle := tcell.StyleDefault.Foreground(styles.Global.TextColor.Color()).Background(bg)

	if section.Element == VimMotionsSectionName {
		h.keysTable.SetTitle(" Vim Motions (read-only) ")
		h.keysTable.SetCell(0, 0, tview.NewTableCell("").SetSelectable(false))
		h.keysTable.SetCell(0, 1, tview.NewTableCell("Active in SQL editor: query tab, row add / edit / duplicate").
			SetSelectable(false).SetStyle(keyStyle))
		for row, key := range section.Keys {
			keyString := formatHelpKeyString(key)
			h.keysTable.SetCell(row+1, 0, tview.NewTableCell(keyString).SetStyle(keyStyle))
			h.keysTable.SetCell(row+1, 1, tview.NewTableCell(key.Description).SetStyle(descStyle))
		}
		h.keysTable.ScrollToBeginning()
		if h.keysTable.GetRowCount() > 1 {
			h.keysTable.Select(1, 0)
		}
		return
	}

	h.keysTable.SetTitle(" Keys ")
	for row, key := range section.Keys {
		keyString := formatHelpKeyString(key)
		h.keysTable.SetCell(row, 0, tview.NewTableCell(keyString).SetStyle(keyStyle))
		h.keysTable.SetCell(row, 1, tview.NewTableCell(key.Description).SetStyle(descStyle))
	}
	h.keysTable.ScrollToBeginning()
	if h.keysTable.GetRowCount() > 0 {
		h.keysTable.Select(0, 0)
	}
}

func (h *Help) renderDataSection(section config.OrderedKeys) {
	h.keysTable.SetTitle(" Keys ")
	styles := h.App.GetStyles()
	bg := styles.Global.BackgroundColor.Color()
	keyStyle := tcell.StyleDefault.Foreground(styles.Global.SecondaryTextColor.Color()).Background(bg)
	descStyle := tcell.StyleDefault.Foreground(styles.Global.TextColor.Color()).Background(bg)

	// Build a description→struct-index map so we can translate back to section.Keys.
	descToIdx := make(map[string]int, len(section.Keys))
	for i, k := range section.Keys {
		descToIdx[k.Description] = i
	}

	queryKeys, tableKeys := h.App.GetKeys().DataKeysSplit()
	row := 0
	for _, key := range queryKeys {
		idx := descToIdx[key.Description]
		h.dataRowMap = append(h.dataRowMap, idx)
		h.keysTable.SetCell(row, 0, tview.NewTableCell(formatHelpKeyString(key)).SetStyle(keyStyle))
		h.keysTable.SetCell(row, 1, tview.NewTableCell(key.Description).SetStyle(descStyle))
		row++
	}

	h.dataRowMap = append(h.dataRowMap, -1)
	h.keysTable.SetCell(row, 0, tview.NewTableCell("").SetSelectable(false))
	h.keysTable.SetCell(row, 1, tview.NewTableCell("─── Table only ───────────────").
		SetSelectable(false).SetStyle(keyStyle))
	row++

	for _, key := range tableKeys {
		idx := descToIdx[key.Description]
		h.dataRowMap = append(h.dataRowMap, idx)
		h.keysTable.SetCell(row, 0, tview.NewTableCell(formatHelpKeyString(key)).SetStyle(keyStyle))
		h.keysTable.SetCell(row, 1, tview.NewTableCell(key.Description).SetStyle(descStyle))
		row++
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
	for _, seq := range key.Sequences {
		parts = append(parts, "<"+seq+">")
	}
	return strings.Join(parts, ", ")
}
