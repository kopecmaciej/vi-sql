package modal

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/google/uuid"
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
	ConnectionID string
	Query        string
	Time         time.Time
}

// History is a two-panel modal: a filterable table of past queries on top,
// and a syntax-highlighted full-query preview on the bottom.
type History struct {
	*core.BaseElement
	*core.Flex

	connectionID string
	entries      []historyEntry // all entries for current connection, newest-first
	filtered     []historyEntry // currently visible subset
	table        *tview.Table
	preview      *core.TextView
	searchInput  *tview.InputField
	searchMode   bool
	onAccept     func(query string)
	onClose      func()
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

// SetConnectionID scopes this modal to a specific connection.
func (h *History) SetConnectionID(id string) { h.connectionID = id }

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
			h.App.Pages.RemoveModalPage(HistoryModalId)
			if h.onAccept != nil {
				if idx := row - 1; idx >= 0 && idx < len(h.filtered) {
					h.onAccept(h.filtered[idx].Query)
				}
			}
			return nil
		case keys.Match(keys.Common.Close, event):
			h.App.Pages.RemoveModalPage(HistoryModalId)
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
	if err := replaceConnectionEntries(h.connectionID, nil); err != nil {
		ShowError(h.App.Pages, "Failed to clear history", err)
	}
	h.App.Pages.RemoveModalPage(HistoryModalId)
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

	if err := replaceConnectionEntries(h.connectionID, reverseEntries(h.entries)); err != nil {
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
		cell := tview.NewTableCell(header).
			SetTextColor(styles.Others.TableHeaderTextColor.Color()).
			SetBackgroundColor(styles.Global.ContrastBackgroundColor.Color()).
			SetSelectable(false)
		if col == 2 {
			cell.SetExpansion(1)
		}
		h.table.SetCell(0, col, cell)
	}

	for i, e := range h.filtered {
		num := fmt.Sprintf(" %03d ", i+1)
		date := "             " // 13 chars to match "Jan 02 15:04" width
		if !e.Time.IsZero() {
			date = " " + e.Time.Local().Format("Jan 02 15:04") + " "
		}
		preview := buildPreview(e.Query)

		bg := styles.Global.BackgroundColor.Color()
		h.table.SetCell(i+1, 0, tview.NewTableCell(num).SetStyle(tcell.StyleDefault.
			Foreground(styles.Global.TextColor.Color()).Background(bg)))
		h.table.SetCell(i+1, 1, tview.NewTableCell(date).SetStyle(tcell.StyleDefault.
			Foreground(styles.Global.DimColor.Color()).Background(bg)))
		h.table.SetCell(i+1, 2, tview.NewTableCell(preview).SetStyle(tcell.StyleDefault.
			Foreground(styles.Global.TextColor.Color()).Background(bg)).SetExpansion(1))
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
	if conn := h.App.GetConfig().GetCurrentConnection(); conn != nil {
		h.Flex.SetTitle(fmt.Sprintf(" SQL History [%s] ", conn.Name))
	}

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
			AddItem(nil, 2, 0, false).
			AddItem(h.Flex, 0, 1, true).
			AddItem(nil, 2, 0, false), 0, 6, true).
		AddItem(nil, 0, 1, false)

	h.App.Pages.ShowModal(HistoryModalId, wrapper, h.table, true, true)
}

// SaveToHistory saves text to the history file, deduplicating and capping at maxHistory for this connection.
func (h *History) SaveToHistory(text string) error {
	entries, err := h.loadHistory()
	if err != nil {
		return err
	}

	var updated []historyEntry
	for _, e := range entries {
		if normalizeQuery(e.Query) != normalizeQuery(text) {
			updated = append(updated, e)
		}
	}
	updated = append(updated, historyEntry{
		ConnectionID: h.connectionID,
		Query:        strings.TrimSpace(text),
		Time:         time.Now(),
	})

	if len(updated) > maxHistory {
		updated = updated[len(updated)-maxHistory:]
	}

	return replaceConnectionEntries(h.connectionID, updated)
}

// PurgeConnectionHistory removes all history entries for the given connection ID.
func PurgeConnectionHistory(connID string) error {
	return replaceConnectionEntries(connID, nil)
}

// loadHistory returns entries for h.connectionID only, in chronological order.
func (h *History) loadHistory() ([]historyEntry, error) {
	all, err := loadAllEntries()
	if err != nil {
		return nil, err
	}
	var filtered []historyEntry
	for _, e := range all {
		if e.ConnectionID == h.connectionID {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

// replaceConnectionEntries atomically replaces all entries for connID with newEntries.
func replaceConnectionEntries(connID string, newEntries []historyEntry) error {
	all, err := loadAllEntries()
	if err != nil {
		return err
	}

	var other []historyEntry
	for _, e := range all {
		if e.ConnectionID != connID {
			other = append(other, e)
		}
	}

	return writeAllEntries(append(other, newEntries...))
}

// loadAllEntries loads every entry from the history file regardless of connection.
// If the file contains the old (pre-UUID) format, it is renamed to history.txt.bak.
func loadAllEntries() ([]historyEntry, error) {
	path := getHistoryFilePath()

	if historyNeedsMigration(path) {
		if err := os.Rename(path, path+".bak"); err != nil {
			return nil, err
		}
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var entries []historyEntry
	for line := range strings.SplitSeq(string(data), "\n") {
		if line == "" {
			continue
		}
		e, ok := parseHistoryLine(line)
		if ok {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

// historyNeedsMigration returns true when the file exists, is non-empty, and uses the old format (no UUID prefix).
func historyNeedsMigration(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return false
	}
	firstLine, _, _ := strings.Cut(string(data), "\n")
	if firstLine == "" {
		return false
	}
	field, _, ok := strings.Cut(firstLine, "|")
	if !ok {
		return true // no pipe = bare query, definitely old format
	}
	_, err = uuid.Parse(field)
	return err != nil // first field is not a UUID → old format
}

func writeAllEntries(entries []historyEntry) error {
	f, err := os.OpenFile(getHistoryFilePath(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	for _, e := range entries {
		escaped := strings.ReplaceAll(e.Query, "\n", `\n`)
		line := e.ConnectionID + "|" + e.Time.UTC().Format(time.RFC3339) + "|" + escaped + "\n"
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

// normalizeQuery collapses whitespace and lowercases for deduplication comparison.
func normalizeQuery(q string) string {
	return strings.ToLower(strings.Join(strings.Fields(q), " "))
}

// buildPreview collapses whitespace and truncates to previewMaxLen runes.
func buildPreview(query string) string {
	s := strings.Join(strings.Fields(query), " ")
	runes := []rune(s)
	if len(runes) > previewMaxLen {
		return string(runes[:previewMaxLen]) + "…"
	}
	return s
}

// parseHistoryLine parses a line in the format "UUID|RFC3339|query".
func parseHistoryLine(line string) (historyEntry, bool) {
	connID, rest, ok := strings.Cut(line, "|")
	if !ok {
		return historyEntry{}, false
	}
	if _, err := uuid.Parse(connID); err != nil {
		return historyEntry{}, false
	}

	var t time.Time
	var raw string
	if before, after, ok := strings.Cut(rest, "|"); ok {
		if parsed, err := time.Parse(time.RFC3339, before); err == nil {
			t = parsed
			raw = after
		}
	}
	if raw == "" {
		raw = rest
	}

	return historyEntry{
		ConnectionID: connID,
		Query:        strings.ReplaceAll(raw, `\n`, "\n"),
		Time:         t,
	}, true
}

func getHistoryFilePath() string {
	configDir, err := util.GetConfigDir()
	if err != nil {
		return ""
	}
	return configDir + "/history.txt"
}
