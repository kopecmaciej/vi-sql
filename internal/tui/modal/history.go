package modal

import (
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/primitives"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

const (
	HistoryModalId = "History"
	queryBarId     = "QueryBar"

	maxHistory = 100
)

// History is a modal that displays previously executed SQL queries.
type History struct {
	*core.BaseElement
	*primitives.ListModal

	style *config.HistoryStyle
}

func NewHistoryModal() *History {
	h := &History{
		BaseElement: core.NewBaseElement(),
		ListModal:   primitives.NewListModal(),
	}

	h.SetIdentifier(HistoryModalId)
	h.SetAfterInitFunc(h.init)

	return h
}

func (h *History) init() error {
	h.setLayout()
	h.setStyle()
	h.setKeybindings()

	return nil
}

func (h *History) setLayout() {
	h.SetTitle(" SQL History ")
	h.SetBorder(true)
	h.ShowSecondaryText(false)
	h.ListModal.SetBorderPadding(1, 1, 2, 2)
}

func (h *History) setStyle() {
	h.style = &h.App.GetStyles().History
	globalBackground := h.App.GetStyles().Global.BackgroundColor.Color()

	mainStyle := tcell.StyleDefault.
		Foreground(h.style.TextColor.Color()).
		Background(globalBackground)
	h.SetMainTextStyle(mainStyle)

	selectedStyle := tcell.StyleDefault.
		Foreground(globalBackground).
		Background(h.style.SelectedBackgroundColor.Color())
	h.SetSelectedStyle(selectedStyle)
}

func (h *History) setKeybindings() {
	keys := h.App.GetKeys()
	h.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case keys.Contains(keys.History.AcceptEntry, event.Name()):
			return h.sendEventAndClose(event)
		case keys.Contains(keys.History.CloseHistory, event.Name()):
			return h.sendEventAndClose(event)
		case keys.Contains(keys.History.ClearHistory, event.Name()):
			return h.clearHistory()
		}
		return event
	})
}

func (h *History) sendEventAndClose(event *tcell.EventKey) *tcell.EventKey {
	msg := manager.EventMsg{EventKey: event, Sender: h.GetIdentifier()}
	h.SendToElement(queryBarId, msg)
	h.App.Pages.RemovePage(h.GetIdentifier())

	return nil
}

func (h *History) clearHistory() *tcell.EventKey {
	if err := os.WriteFile(getHistoryFilePath(), []byte{}, 0644); err != nil {
		ShowError(h.App.Pages, "Failed to clear history", err)
	}
	h.App.Pages.RemovePage(h.GetIdentifier())

	return nil
}

// Render loads history from file and displays it newest-first.
func (h *History) Render() {
	h.Clear()

	history, err := h.loadHistory()
	if err != nil {
		ShowError(h.App.Pages, "Failed to load history", err)
		return
	}

	for i := len(history) - 1; i >= 0; i-- {
		h.AddItem(history[i], "", 0, nil)
	}

	h.App.Pages.AddPage(h.GetIdentifier(), h, true, true)
}

// SaveToHistory saves text to the history file, deduplicating and capping at maxHistory.
func (h *History) SaveToHistory(text string) error {
	history, err := h.loadHistory()
	if err != nil {
		return err
	}

	var updated []string
	for _, line := range history {
		if line != text {
			updated = append(updated, line)
		}
	}
	updated = append(updated, text)

	if len(updated) > maxHistory {
		updated = updated[len(updated)-maxHistory:]
	}

	f, err := os.OpenFile(getHistoryFilePath(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, entry := range updated {
		if _, err = f.WriteString(entry + "\n"); err != nil {
			return err
		}
	}

	return nil
}

// GetText returns the text of the currently selected history entry.
func (h *History) GetText() string {
	return strings.TrimSpace(h.ListModal.GetText())
}

func (h *History) loadHistory() ([]string, error) {
	data, err := os.ReadFile(getHistoryFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			err = os.WriteFile(getHistoryFilePath(), []byte{}, 0644)
		}
		return nil, err
	}

	var history []string
	for _, line := range strings.Split(string(data), "\n") {
		if line != "" {
			history = append(history, line)
		}
	}

	return history, nil
}

func getHistoryFilePath() string {
	configDir, err := util.GetConfigDir()
	if err != nil {
		return ""
	}

	return configDir + "/history.txt"
}
