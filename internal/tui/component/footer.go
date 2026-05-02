package component

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/rs/zerolog/log"

	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

const (
	FooterId = "Footer"
)

type (
	info struct {
		label string
		value string
	}

	Footer struct {
		*core.BaseElement
		*core.Table

		keys           []config.Key
		currentFocus   tview.Identifier
		expanded       bool
		centered       bool
		pinnedKeys     []config.Key
		chordPending   rune
		onHeightChange func()
	}
)

func (f *Footer) SetCentered(centered bool) {
	f.centered = centered
}

func (f *Footer) SetPinnedKeys(keys []config.Key) {
	f.pinnedKeys = keys
}

func NewFooter() *Footer {
	f := &Footer{
		BaseElement: core.NewBaseElement(),
		Table:       core.NewTable(),
	}
	f.SetIdentifier(FooterId)
	f.SetAfterInitFunc(f.init)
	return f
}

func (f *Footer) init() error {
	f.setStyle()
	f.setLayout()
	f.handleEvents()
	return nil
}

func (f *Footer) setLayout() {
	f.Table.SetBorder(false)
	f.Table.SetBorderPadding(0, 1, 1, 1)
}

func (f *Footer) setStyle() {
	f.SetStyle(f.App.GetStyles())
}

// SetOnHeightChange registers a callback invoked when the footer height needs
// to change due to a focus switch while expanded.
func (f *Footer) SetOnHeightChange(fn func()) {
	f.onHeightChange = fn
}

// Toggle flips the expanded state and returns the new required height.
func (f *Footer) Toggle() int {
	f.expanded = !f.expanded
	if f.expanded {
		return f.ExpandedHeight()
	}
	return 2 // 1 row + padding
}

func (f *Footer) collectPairs() []info {
	keys, _ := f.UpdateKeys()
	pairs := make([]info, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, info{formatKeyString(key), key.Description})
	}
	return pairs
}

func (f *Footer) expandedLayout(width int, pairs []info) (numGroups, numRows int) {
	if width <= 0 {
		width = 80
	}
	if len(pairs) == 0 {
		return 1, 0
	}
	// Target 2 rows; each group takes ~30 chars (key + value + separator).
	maxGroups := width / 30
	if maxGroups < 1 {
		maxGroups = 1
	}
	desiredGroups := (len(pairs) + 1) / 2 // ceil(n/2) → 2 rows
	numGroups = desiredGroups
	if numGroups > maxGroups {
		numGroups = maxGroups
	}
	numRows = (len(pairs) + numGroups - 1) / numGroups
	return numGroups, numRows
}

// ExpandedHeight returns the rows the footer needs when expanded.
func (f *Footer) ExpandedHeight() int {
	_, _, width, _ := f.Table.GetInnerRect()
	pairs := f.collectPairs()
	_, numRows := f.expandedLayout(width, pairs)
	return numRows + 1 // +1 for bottom padding from SetBorderPadding(0,1,1,1)
}

func (f *Footer) renderExpanded() {
	f.Table.Clear()
	pairs := f.collectPairs()
	if len(pairs) == 0 {
		return
	}

	_, _, width, _ := f.Table.GetInnerRect()
	numGroups, numRows := f.expandedLayout(width, pairs)

	for i, p := range pairs {
		row := i % numRows
		group := i / numRows
		col := group * 2
		f.Table.SetCell(row, col, f.keyCell(p.label))
		f.Table.SetCell(row, col+1, f.valueCell(p.value))
		if group < numGroups-1 {
			f.Table.SetCell(row, col+2, tview.NewTableCell(""))
		}
	}
}

// Render draws the footer. In collapsed mode it shows the focused element's
// keybindings in a single row. When expanded it uses a multi-column grid.
func (f *Footer) Render() {
	if f.expanded {
		f.renderExpanded()
		return
	}

	f.Table.Clear()
	k, err := f.UpdateKeys()
	if err != nil {
		log.Debug().Err(err).Msg("Footer: failed to update keys")
		return
	}

	if f.centered {
		f.Table.SetCell(0, 0, tview.NewTableCell("").SetExpansion(1))
		col := 1
		for _, key := range k {
			f.Table.SetCell(0, col, f.keyCell(formatKeyString(key)))
			f.Table.SetCell(0, col+1, f.valueCell(key.Description))
			col += 2
		}
		f.Table.SetCell(0, col, tview.NewTableCell("").SetExpansion(1))
		return
	}

	col := 0
	f.Table.SetCell(0, col, f.chordPendingCell(f.chordPending))
	col++

	for _, key := range f.pinnedKeys {
		f.Table.SetCell(0, col, f.keyCell(formatKeyString(key)))
		f.Table.SetCell(0, col+1, f.valueCell(key.Description))
		col += 2
	}

	for _, key := range k {
		f.Table.SetCell(0, col, f.keyCell(formatKeyString(key)))
		f.Table.SetCell(0, col+1, f.valueCell(key.Description))
		col += 2
	}

}

func (f *Footer) handleEvents() {
	go f.HandleEvents(FooterId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.FocusChanged:
			f.currentFocus = tview.Identifier(event.Message.Data.(tview.Identifier))
			go f.App.QueueUpdateDraw(func() {
				if f.expanded && f.onHeightChange != nil {
					f.onHeightChange()
				}
				f.Render()
			})
		case manager.StyleChanged:
			go f.App.QueueUpdateDraw(func() {
				f.setStyle()
				f.Render()
			})
		case manager.ChordPendingChanged:
			f.chordPending = event.Message.Data.(rune)
			go f.App.QueueUpdateDraw(f.Render)
		}
	})
}

func (f *Footer) keyCell(text string) *tview.TableCell {
	cell := tview.NewTableCell(text)
	cell.SetTextColor(f.App.GetStyles().Global.SecondaryTextColor.Color())
	return cell
}

func (f *Footer) valueCell(text string) *tview.TableCell {
	cell := tview.NewTableCell(text + " ")
	cell.SetTextColor(f.App.GetStyles().Global.TitleColor.Color())
	return cell
}

// chordPendingCell renders the pending chord prefix (e.g. "g")
func (f *Footer) chordPendingCell(prefix rune) *tview.TableCell {
	text := string(prefix)
	if prefix == 0 {
		text = " "
	}
	cell := tview.NewTableCell(text)
	cell.SetTextColor(f.App.GetStyles().Global.TitleColor.Color()).SetAttributes(tcell.AttrBold)
	return cell
}

// SetKeys sets static keybinding hints, bypassing the focus-based lookup.
// Static keys persist when focus moves to an element with no registered keys.
func (f *Footer) SetKeys(keys []config.Key) {
	f.keys = keys
	f.Table.Clear()
	if f.centered {
		f.Table.SetCell(0, 0, tview.NewTableCell("").SetExpansion(1))
		col := 1
		for _, key := range keys {
			f.Table.SetCell(0, col, f.keyCell(formatKeyString(key)))
			f.Table.SetCell(0, col+1, f.valueCell(key.Description))
			col += 2
		}
		f.Table.SetCell(0, col, tview.NewTableCell("").SetExpansion(1))
	} else {
		col := 0
		for _, key := range keys {
			f.Table.SetCell(0, col, f.keyCell(formatKeyString(key)))
			f.Table.SetCell(0, col+1, f.valueCell(key.Description))
			col += 2
		}
	}
}

// UpdateKeys returns the keybindings for the currently focused element.
func (f *Footer) UpdateKeys() ([]config.Key, error) {
	if f.currentFocus == "" {
		if len(f.keys) > 0 {
			return f.keys, nil
		}
		return nil, nil
	}

	focus := string(f.currentFocus)

	// QueryMode results table: show only read-only keys from DataKeys.
	if strings.HasSuffix(focus, ResultsSuffix) {
		keys := f.App.GetKeys().DataKeysForQueryMode()
		f.keys = keys
		return keys, nil
	}

	switch {
	case strings.HasSuffix(focus, FilterBarSuffix) || strings.HasSuffix(focus, SortBarSuffix):
		focus = "InputBar"
	case strings.HasSuffix(focus, EditorSuffix):
		focus = SQLQueryEditorId
	case strings.HasPrefix(focus, "QueryTab-"):
		focus = DataId
	case strings.HasPrefix(focus, PeekerId+"-"):
		focus = string(PeekerId)
	case strings.HasPrefix(focus, ExplainViewerId+"-"):
		focus = ExplainViewerId
	case strings.HasPrefix(focus, SQLEditModalId+"-"):
		focus = SQLEditModalId
	}

	orderedKeys, err := f.App.GetKeys().GetKeysForElement(string(focus))
	if err != nil {
		if len(f.keys) > 0 {
			return f.keys, nil
		}
		return nil, err
	}

	var keys []config.Key
	for _, ok := range orderedKeys {
		keys = append(keys, ok.Keys...)
	}

	if len(keys) > 0 {
		f.keys = keys
	} else {
		f.keys = nil
	}

	return keys, nil
}

func formatKeyString(key config.Key) string {
	var parts []string
	parts = append(parts, key.Keys...)
	parts = append(parts, key.Runes...)
	return strings.Join(parts, ", ")
}
