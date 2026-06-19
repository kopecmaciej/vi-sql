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

		keys            []config.Key
		currentFocus    tview.Identifier
		expanded        bool
		centered        bool
		pinnedKeys      []config.Key
		sequencePending string
		onHeightChange  func()
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

func (f *Footer) Toggle() int {
	const collapsedHeight = 2 // 1 row + bottom padding
	_, _, width, _ := f.Table.GetInnerRect()
	pairs := f.collectPairs()
	_, numRows := f.expandedLayout(width, pairs)
	if numRows <= 1 {
		return collapsedHeight
	}
	f.expanded = !f.expanded
	if f.expanded {
		return numRows + 1
	}
	return collapsedHeight
}

func (f *Footer) collectPairs() []info {
	keys, _ := f.UpdateKeys()
	pairs := make([]info, 0, len(keys))
	for _, key := range keys {
		if label := formatKeyString(key); label != "" {
			pairs = append(pairs, info{label, key.Description})
		}
	}
	return pairs
}

// expandedLayout lays pairs column-major and finds the maximum number of columns that fit.
// tview adds +1 char between every table column, so each pair column costs (maxKeyW+1)+(maxValW+1).
func (f *Footer) expandedLayout(width int, pairs []info) (numGroups, numRows int) {
	if width <= 0 {
		width = 80
	}
	n := len(pairs)
	if n == 0 {
		return 1, 0
	}

	keyW := make([]int, n)
	valW := make([]int, n)
	for i, p := range pairs {
		keyW[i] = tview.TaggedStringWidth(p.label)
		valW[i] = tview.TaggedStringWidth(p.value) + 1 // +1 for trailing space in valueCell
	}

	for G := n; G >= 1; G-- {
		R := (n + G - 1) / G // rows = ceil(n/G)
		used := 0
		for g := 0; g < G; g++ {
			start := g * R
			if start >= n {
				break
			}
			end := start + R
			if end > n {
				end = n
			}
			maxKey, maxVal := 0, 0
			for i := start; i < end; i++ {
				if keyW[i] > maxKey {
					maxKey = keyW[i]
				}
				if valW[i] > maxVal {
					maxVal = valW[i]
				}
			}
			used += (maxKey + 1) + (maxVal + 1)
		}
		if used <= width {
			return G, R
		}
	}
	return 1, n
}

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
	_, numRows := f.expandedLayout(width, pairs)

	for i, p := range pairs {
		row := i % numRows
		col := (i / numRows) * 2
		f.Table.SetCell(row, col, f.keyCell(p.label))
		f.Table.SetCell(row, col+1, f.valueCell(p.value))
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
			if label := formatKeyString(key); label != "" {
				f.Table.SetCell(0, col, f.keyCell(label))
				f.Table.SetCell(0, col+1, f.valueCell(key.Description))
				col += 2
			}
		}
		f.Table.SetCell(0, col, tview.NewTableCell("").SetExpansion(1))
		return
	}

	col := 0
	if f.App.GetConfig().UI.VimMode {
		f.Table.SetCell(0, col, f.sequencePendingCell(f.sequencePending))
		col++
	}

	for _, key := range f.pinnedKeys {
		f.Table.SetCell(0, col, f.keyCell(formatKeyString(key)))
		f.Table.SetCell(0, col+1, f.valueCell(key.Description))
		col += 2
	}

	for _, key := range k {
		if label := formatKeyString(key); label != "" {
			f.Table.SetCell(0, col, f.keyCell(label))
			f.Table.SetCell(0, col+1, f.valueCell(key.Description))
			col += 2
		}
	}

}

func (f *Footer) handleEvents() {
	go f.HandleEvents(f.GetIdentifier(), func(event manager.EventMsg) {
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
		case manager.SequencePendingChanged:
			f.sequencePending = event.Message.Data.(string)
			go f.App.QueueUpdateDraw(f.Render)
		case manager.ConfigChanged:
			go f.App.QueueUpdateDraw(f.Render)
		}
	})
}

func (f *Footer) keyCell(text string) *tview.TableCell {
	styles := f.App.GetStyles()
	return tview.NewTableCell(text).SetStyle(tcell.StyleDefault.
		Foreground(styles.Global.SecondaryTextColor.Color()).
		Background(styles.Global.BackgroundColor.Color()))
}

func (f *Footer) valueCell(text string) *tview.TableCell {
	styles := f.App.GetStyles()
	return tview.NewTableCell(text + " ").SetStyle(tcell.StyleDefault.
		Foreground(styles.Global.TitleColor.Color()).
		Background(styles.Global.BackgroundColor.Color()))
}

// sequencePendingCell renders the pending sequence prefix (e.g. "yr")
func (f *Footer) sequencePendingCell(prefix string) *tview.TableCell {
	text := prefix
	if prefix == "" {
		text = " "
	}
	styles := f.App.GetStyles()
	return tview.NewTableCell(text).SetStyle(tcell.StyleDefault.
		Foreground(styles.Global.TitleColor.Color()).
		Background(styles.Global.BackgroundColor.Color()).
		Attributes(tcell.AttrBold))
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
	case strings.HasSuffix(focus, FilterBarSuffix) || strings.HasSuffix(focus, OrderBarSuffix):
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

	k := f.App.GetKeys()
	editorEnabled := f.App.GetConfig().Editor.Enabled
	var keys []config.Key
	for _, ok := range orderedKeys {
		for _, key := range ok.Keys {
			if !editorEnabled && key.Description == k.SQLQueryEditor.TermEditor.Description {
				continue
			}
			keys = append(keys, key)
		}
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
	parts = append(parts, key.Sequences...)
	return strings.Join(parts, ", ")
}
