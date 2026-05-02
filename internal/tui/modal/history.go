package modal

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

const (
	HistoryModalId = "History"

	maxHistory    = 100
	previewMaxLen = 80
)

// historyEntry is a single history item with an optional timestamp.
type historyEntry struct {
	Query string
	Time  time.Time
}

// History is a two-panel modal: a filterable table of past queries on top,
// and a syntax-highlighted full-query preview on the bottom.
type History struct {
	*core.BaseElement
	*core.Flex

	entries     []historyEntry // all entries, newest-first
	filtered    []historyEntry // currently visible subset
	table       *tview.Table
	preview     *core.TextView
	searchInput *tview.InputField
	searchMode  bool
	onAccept    func(query string)
	onClose     func()
}

func NewHistoryModal() *History {
	h := &History{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		table:       tview.NewTable(),
		preview:     core.NewTextView(),
		searchInput: tview.NewInputField(),
	}
	h.SetIdentifier(HistoryModalId)
	h.SetAfterInitFunc(h.init)
	return h
}

func (h *History) init() error {
	h.setLayout()
	h.setStyle()
	h.setKeybindings()
	h.handleEvents()
	return nil
}

func (h *History) handleEvents() {
	go h.HandleEvents(h.GetIdentifier(), func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			h.setStyle()
			if len(h.filtered) > 0 {
				h.renderTable()
			}
		}
	})
}

// SetOnAccept sets the callback invoked when the user accepts a history entry.
func (h *History) SetOnAccept(fn func(query string)) { h.onAccept = fn }

// GetOnAccept returns the current onAccept callback.
func (h *History) GetOnAccept() func(query string) { return h.onAccept }

// SetOnClose sets the callback invoked when the user closes the modal.
func (h *History) SetOnClose(fn func()) { h.onClose = fn }

func (h *History) setLayout() {
	h.Flex.SetDirection(tview.FlexRow)
	h.Flex.SetBorder(true)
	h.Flex.SetTitle(" SQL History ")

	h.table.SetIdentifier(HistoryModalId)
	h.table.SetBorderPadding(0, 0, 1, 1)
	h.table.SetSelectable(true, false)
	h.table.SetFixed(1, 0)

	h.preview.SetBorder(true)
	h.preview.SetTitle(" Preview ")
	h.preview.SetBorderPadding(0, 0, 1, 1)
	h.preview.SetDynamicColors(true)
	h.preview.SetScrollable(true)
	h.preview.SetWrap(true)

	h.searchInput.SetLabel(" / ")
	h.searchInput.SetBorder(true)
}

func (h *History) setStyle() {
	styles := h.App.GetStyles()
	globalBg := styles.Global.BackgroundColor.Color()

	h.Flex.SetStyle(styles)

	h.table.SetBackgroundColor(globalBg)
	selectedStyle := tcell.StyleDefault.
		Foreground(globalBg).
		Background(styles.Global.MoreContrastBackgroundColor.Color())
	h.table.SetSelectedStyle(selectedStyle)

	h.preview.SetStyle(styles)

	h.searchInput.SetBackgroundColor(globalBg)
	h.searchInput.SetFieldBackgroundColor(styles.Global.ContrastBackgroundColor.Color())
	h.searchInput.SetFieldTextColor(styles.Global.TextColor.Color())
	h.searchInput.SetLabelStyle(tcell.StyleDefault.
		Foreground(styles.Global.FocusColor.Color()).
		Background(globalBg))
}

func (h *History) rebuildLayout() {
	h.Flex.Clear()
	if h.searchMode {
		h.Flex.AddItem(h.searchInput, 3, 0, true)
	}

	h.Flex.AddItem(h.table, 0, 3, !h.searchMode)
	h.Flex.AddItem(h.preview, 0, 2, false)
}

func (h *History) setKeybindings() {
	keys := h.App.GetKeys()

	h.table.SetInputCapture(keys.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case keys.Match(keys.Common.Select, event):
			row, _ := h.table.GetSelection()
			h.App.Pages.RemovePage(HistoryModalId)
			if h.onAccept != nil {
				if idx := row - 1; idx >= 0 && idx < len(h.filtered) {
					h.onAccept(h.filtered[idx].Query)
				}
			}
			return nil
		case keys.Match(keys.Common.Close, event):
			h.App.Pages.RemovePage(HistoryModalId)
			if h.onClose != nil {
				h.onClose()
			}
			return nil
		case keys.Match(keys.History.PurgeHistory, event):
			return h.clearHistory()
		case keys.Match(keys.Common.Delete, event):
			h.deleteCurrentEntry()
			return nil
		case keys.Match(keys.History.CopyQuery, event):
			h.copyCurrentQuery()
			return nil
		case event.Rune() == '/':
			h.enterSearchMode()
			return nil
		}
		return event
	}))

	h.searchInput.SetChangedFunc(func(text string) {
		h.filterEntries(text)
	})
	h.searchInput.SetDoneFunc(func(key tcell.Key) {
		h.exitSearchMode()
	})
}

func (h *History) enterSearchMode() {
	h.searchMode = true
	h.searchInput.SetText("")
	h.rebuildLayout()
	h.App.SetFocusOnly(h.searchInput)
}

func (h *History) exitSearchMode() {
	h.searchMode = false
	h.searchInput.SetText("")
	h.filterEntries("")
	h.rebuildLayout()
	h.App.SetFocusOnly(h.table)
}

func (h *History) clearHistory() *tcell.EventKey {
	if err := os.WriteFile(getHistoryFilePath(), []byte{}, 0644); err != nil {
		ShowError(h.App.Pages, "Failed to clear history", err)
	}
	h.App.Pages.RemovePage(HistoryModalId)
	return nil
}

func (h *History) deleteCurrentEntry() {
	row, _ := h.table.GetSelection()
	idx := row - 1
	if idx < 0 || idx >= len(h.filtered) {
		return
	}
	query := h.filtered[idx].Query

	for i, e := range h.entries {
		if e.Query == query {
			h.entries = append(h.entries[:i], h.entries[i+1:]...)
			break
		}
	}
	h.filtered = append(h.filtered[:idx], h.filtered[idx+1:]...)

	if err := h.writeEntries(reverseEntries(h.entries)); err != nil {
		ShowError(h.App.Pages, "Failed to delete history entry", err)
		return
	}
	h.renderTable()
}

func (h *History) copyCurrentQuery() {
	row, _ := h.table.GetSelection()
	idx := row - 1
	if idx < 0 || idx >= len(h.filtered) {
		return
	}
	util.Copy(h.filtered[idx].Query)
}

func (h *History) filterEntries(text string) {
	if text == "" {
		h.filtered = append([]historyEntry{}, h.entries...)
	} else {
		lower := strings.ToLower(text)
		h.filtered = h.filtered[:0]
		for _, e := range h.entries {
			if strings.Contains(strings.ToLower(e.Query), lower) {
				h.filtered = append(h.filtered, e)
			}
		}
	}
	h.renderTable()
}

func (h *History) renderTable() {
	h.table.Clear()
	h.table.SetSelectionChangedFunc(func(row, _ int) {
		h.updatePreview(row)
	})

	styles := h.App.GetStyles()
	headers := []string{"  #  ", "  TIMESTAMP   ", "  QUERY"}
	for col, header := range headers {
		h.table.SetCell(0, col, tview.NewTableCell(header).
			SetTextColor(styles.Others.TableHeaderTextColor.Color()).
			SetBackgroundColor(styles.Global.ContrastBackgroundColor.Color()).
			SetSelectable(false))
	}

	for i, e := range h.filtered {
		num := fmt.Sprintf(" %03d ", i+1)
		date := "             " // 13 chars to match "Jan 02 15:04" width
		if !e.Time.IsZero() {
			date = " " + e.Time.Local().Format("Jan 02 15:04") + " "
		}
		preview := buildPreview(e.Query)

		h.table.SetCell(i+1, 0, tview.NewTableCell(num).SetTextColor(styles.Global.TextColor.Color()))
		h.table.SetCell(i+1, 1, tview.NewTableCell(date).SetTextColor(styles.Global.DimColor.Color()))
		h.table.SetCell(i+1, 2, tview.NewTableCell(preview).SetTextColor(styles.Global.TextColor.Color()))
	}

	if len(h.filtered) > 0 {
		h.table.Select(1, 0)
		h.updatePreview(1)
	} else {
		h.preview.SetText("")
	}
}

func (h *History) updatePreview(row int) {
	idx := row - 1
	if idx < 0 || idx >= len(h.filtered) {
		h.preview.SetText("")
		return
	}
	sqlStyle := &h.App.GetStyles().SQLEditor
	h.preview.SetText(core.ColorizeSQLText(h.filtered[idx].Query, sqlStyle))
	h.preview.ScrollToBeginning()
}

// Render loads history and displays the two-panel modal.
func (h *History) Render() {
	loaded, err := h.loadHistory()
	if err != nil {
		ShowError(h.App.Pages, "Failed to load history", err)
		return
	}
	h.entries = reverseEntries(loaded)
	h.filtered = append([]historyEntry{}, h.entries...)
	h.searchMode = false
	h.rebuildLayout()
	h.renderTable()

	wrapper := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(h.Flex, 0, 5, true).
			AddItem(nil, 0, 1, false), 0, 5, true).
		AddItem(nil, 0, 1, false)

	h.App.Pages.AddPage(HistoryModalId, wrapper, true, true)
	h.App.SetFocusOnly(h.table)
}

// SaveToHistory saves text to the history file, deduplicating and capping at maxHistory.
func (h *History) SaveToHistory(text string) error {
	entries, err := h.loadHistory()
	if err != nil {
		return err
	}

	var updated []historyEntry
	for _, e := range entries {
		if e.Query != text {
			updated = append(updated, e)
		}
	}
	updated = append(updated, historyEntry{Query: text, Time: time.Now()})

	if len(updated) > maxHistory {
		updated = updated[len(updated)-maxHistory:]
	}

	return h.writeEntries(updated)
}

func (h *History) loadHistory() ([]historyEntry, error) {
	data, err := os.ReadFile(getHistoryFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			err = os.WriteFile(getHistoryFilePath(), []byte{}, 0644)
		}
		return nil, err
	}

	var entries []historyEntry
	for _, line := range strings.Split(string(data), "\n") {
		if line != "" {
			entries = append(entries, parseHistoryLine(line))
		}
	}
	return entries, nil
}

// parseHistoryLine parses a line from the history file.
// New format: "RFC3339|query". Old format (no timestamp): "query".
func parseHistoryLine(line string) historyEntry {
	var raw string
	var t time.Time
	if idx := strings.Index(line, "|"); idx != -1 {
		if parsed, err := time.Parse(time.RFC3339, line[:idx]); err == nil {
			t = parsed
			raw = line[idx+1:]
		}
	}
	if raw == "" {
		raw = line
	}
	return historyEntry{Query: strings.ReplaceAll(raw, `\n`, "\n"), Time: t}
}

func (h *History) writeEntries(entries []historyEntry) error {
	f, err := os.OpenFile(getHistoryFilePath(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	for _, e := range entries {
		escaped := strings.ReplaceAll(e.Query, "\n", `\n`)
		var line string
		if e.Time.IsZero() {
			line = escaped + "\n"
		} else {
			line = e.Time.UTC().Format(time.RFC3339) + "|" + escaped + "\n"
		}
		if _, err = f.WriteString(line); err != nil {
			return err
		}
	}
	return nil
}

func reverseEntries(entries []historyEntry) []historyEntry {
	n := len(entries)
	result := make([]historyEntry, n)
	for i, e := range entries {
		result[n-1-i] = e
	}
	return result
}

// buildPreview collapses whitespace and truncates to previewMaxLen runes.
func buildPreview(query string) string {
	s := strings.ReplaceAll(query, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	runes := []rune(s)
	if len(runes) > previewMaxLen {
		return string(runes[:previewMaxLen]) + "…"
	}
	return s
}

func getHistoryFilePath() string {
	configDir, err := util.GetConfigDir()
	if err != nil {
		return ""
	}
	return configDir + "/history.txt"
}
