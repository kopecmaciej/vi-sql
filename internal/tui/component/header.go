package component

import (
	"fmt"
	"strings"

	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

const (
	HeaderId = "Header"
)

type (
	order int

	info struct {
		label string
		value string
	}

	BaseInfo map[order]info

	Header struct {
		*core.BaseElement
		*core.Table

		style          *config.HeaderStyle
		baseInfo       BaseInfo
		keys           []config.Key
		currentFocus   tview.Identifier
		expanded       bool
		onHeightChange func()
	}
)

func NewHeader() *Header {
	h := Header{
		BaseElement: core.NewBaseElement(),
		Table:       core.NewTable(),
		baseInfo:    make(BaseInfo),
	}

	h.SetIdentifier(HeaderId)
	h.SetAfterInitFunc(h.init)

	return &h
}

func (h *Header) init() error {
	h.setStyle()
	h.setLayout()
	h.handleEvents()
	return nil
}

func (h *Header) setLayout() {
	h.Table.SetBorder(true)
	h.Table.SetTitle(" Basic Info ")
	h.Table.SetBorderPadding(0, 0, 1, 1)
}

func (h *Header) setStyle() {
	h.style = &h.App.GetStyles().Header
	h.SetStyle(h.App.GetStyles())
}

// SetOnHeightChange registers a callback invoked when the header height needs
// to change due to a focus switch while expanded.
func (h *Header) SetOnHeightChange(f func()) {
	h.onHeightChange = f
}

// SetBaseInfo populates base info with connection host and port.
func (h *Header) SetBaseInfo() BaseInfo {
	conn := h.App.GetConfig().GetCurrentConnection()
	if conn == nil {
		h.baseInfo = BaseInfo{
			0: {h.style.InactiveSymbol.String(), "not connected"},
		}
		return h.baseInfo
	}
	h.baseInfo = BaseInfo{
		0: {"host", conn.Host},
		1: {"port", fmt.Sprintf("%d", conn.Port)},
	}
	return h.baseInfo
}

func (h *Header) setInactiveBaseInfo(err error) {
	conn := h.App.GetConfig().GetCurrentConnection()
	h.baseInfo = make(BaseInfo)
	host := ""
	if conn != nil {
		host = conn.Host
	}
	h.baseInfo[0] = info{"host", host}
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unauthorized") {
			h.baseInfo[1] = info{"error", "unauthorized — check credentials"}
		} else {
			h.baseInfo[1] = info{"error", err.Error()}
		}
	}
}

// Toggle flips the expanded state and returns the new required height.
func (h *Header) Toggle() int {
	h.expanded = !h.expanded
	if h.expanded {
		return h.ExpandedHeight()
	}
	return 4
}

// collectPairs returns all label-value pairs: base info followed by the keys
// of the currently focused element.
func (h *Header) collectPairs() []info {
	base := h.SetBaseInfo()
	pairs := make([]info, 0, len(base)+16)
	for i := 0; i < len(base); i++ {
		b := base[order(i)]
		pairs = append(pairs, info{b.label, b.value})
	}

	keys, _ := h.UpdateKeys()
	for _, key := range keys {
		pairs = append(pairs, info{formatKeyString(key), key.Description})
	}
	return pairs
}

// expandedLayout computes the number of column groups and rows for the expanded
// view given the available inner width and the set of pairs to display.
// Each group is estimated at 40 chars wide.
func (h *Header) expandedLayout(width int, pairs []info) (numGroups, numRows int) {
	if width <= 0 {
		width = 80
	}
	if len(pairs) == 0 {
		return 1, 0
	}
	numGroups = width / 40
	if numGroups < 1 {
		numGroups = 1
	}
	numRows = (len(pairs) + numGroups - 1) / numGroups
	return numGroups, numRows
}

// ExpandedHeight returns the rows the header needs when expanded.
func (h *Header) ExpandedHeight() int {
	_, _, width, _ := h.Table.GetInnerRect()
	pairs := h.collectPairs()
	_, numRows := h.expandedLayout(width, pairs)
	return numRows + 2 // +2 for borders
}

// renderExpanded draws all pairs in a column-major multi-column grid.
func (h *Header) renderExpanded() {
	h.Table.Clear()
	pairs := h.collectPairs()
	if len(pairs) == 0 {
		return
	}

	_, _, width, _ := h.Table.GetInnerRect()
	numGroups, numRows := h.expandedLayout(width, pairs)

	for i, p := range pairs {
		row := i % numRows
		group := i / numRows
		col := group * 2
		h.Table.SetCell(row, col, h.keyCell(p.label))
		h.Table.SetCell(row, col+1, h.valueCell(p.value))
		if group < numGroups-1 {
			h.Table.SetCell(row, col+2, tview.NewTableCell(""))
		}
	}
}

// Render draws the header. In collapsed mode it shows the connection base info
// on the left, a spacer column, then the focused element's keybindings.
func (h *Header) Render() {
	if h.expanded {
		h.renderExpanded()
		return
	}

	h.Table.Clear()
	base := h.SetBaseInfo()

	maxInRow := 2
	currCol := 0
	currRow := 0

	for i := 0; i < len(base); i++ {
		if i%maxInRow == 0 && i != 0 {
			currCol += 2
			currRow = 0
		}
		o := order(i)
		h.Table.SetCell(currRow, currCol, h.keyCell(base[o].label))
		h.Table.SetCell(currRow, currCol+1, h.valueCell(base[o].value))
		currRow++
	}

	// Spacer between base info and keys
	h.Table.SetCell(0, 2, tview.NewTableCell(""))
	h.Table.SetCell(1, 2, tview.NewTableCell(""))
	currCol++

	k, err := h.UpdateKeys()
	if err != nil {
		currCol += 2
		h.Table.SetCell(0, currCol, h.keyCell("no keys for this element"))
		h.Table.SetCell(1, currCol, h.valueCell("press <"+h.App.GetKeys().Global.ToggleFullScreenHelp.String()+"> for all keybindings"))
		return
	}

	for _, key := range k {
		if currRow%maxInRow == 0 && currRow != 0 {
			currCol += 2
			currRow = 0
		}
		h.Table.SetCell(currRow, currCol, h.keyCell(formatKeyString(key)))
		h.Table.SetCell(currRow, currCol+1, h.valueCell(key.Description))
		currRow++
	}
}

func (h *Header) handleEvents() {
	go h.HandleEvents(HeaderId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.FocusChanged:
			h.currentFocus = tview.Identifier(event.Message.Data.(tview.Identifier))
			go h.App.QueueUpdateDraw(func() {
				if h.expanded && h.onHeightChange != nil {
					h.onHeightChange()
				}
				h.Render()
			})
		case manager.StyleChanged:
			h.setStyle()
			go h.App.QueueUpdateDraw(func() {
				h.Render()
			})
		}
	})
}

func (h *Header) keyCell(text string) *tview.TableCell {
	cell := tview.NewTableCell(text + " ")
	cell.SetTextColor(h.style.KeyColor.Color())
	return cell
}

func (h *Header) valueCell(text string) *tview.TableCell {
	cell := tview.NewTableCell(text)
	cell.SetTextColor(h.style.ValueColor.Color())
	return cell
}

// UpdateKeys returns the keybindings for the currently focused element.
func (h *Header) UpdateKeys() ([]config.Key, error) {
	if h.currentFocus == "" {
		return nil, nil
	}

	focus := h.currentFocus
	if focus == SchemaTreeId {
		focus = "Schema"
	}

	orderedKeys, err := h.App.GetKeys().GetKeysForElement(string(focus))
	if err != nil {
		return nil, err
	}
	keys := orderedKeys[0].Keys

	if len(keys) > 0 {
		h.keys = keys
	} else {
		h.keys = nil
	}

	return keys, nil
}

func formatKeyString(key config.Key) string {
	var parts []string
	parts = append(parts, key.Keys...)
	parts = append(parts, key.Runes...)
	return strings.Join(parts, ", ")
}
