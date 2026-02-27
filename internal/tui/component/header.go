package component

import (
	"fmt"

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

		style        *config.HeaderStyle
		baseInfo     BaseInfo
		keys         []config.Key
		currentFocus tview.Identifier
		expanded     bool
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

func (h *Header) SetBaseInfo() BaseInfo {
	conn := h.App.GetConfig().GetCurrentConnection()
	if conn == nil {
		h.baseInfo = BaseInfo{
			0: {"Status", h.style.InactiveSymbol.String()},
		}
		return h.baseInfo
	}
	h.baseInfo = BaseInfo{
		0: {"Status", h.style.ActiveSymbol.String()},
		1: {"Host", conn.Host},
	}
	return h.baseInfo
}

// Toggle flips the expanded state and returns the new required height for the
// header so the caller can resize the layout accordingly.
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
	if h.currentFocus != "" {
		focus := h.currentFocus
		if focus == SchemaTreeId {
			focus = "Schema"
		}
		if orderedKeys, err := h.App.GetKeys().GetKeysForElement(string(focus)); err == nil && len(orderedKeys) > 0 {
			for _, key := range orderedKeys[0].Keys {
				var keyString string
				var iter []string
				if len(key.Keys) > 0 {
					iter = append(iter, key.Keys...)
				}
				if len(key.Runes) > 0 {
					iter = append(iter, key.Runes...)
				}
				for i, k := range iter {
					if i == 0 {
						keyString = k
					} else {
						keyString = fmt.Sprintf("%s, %s", keyString, k)
					}
				}
				pairs = append(pairs, info{keyString, key.Description})
			}
		}
	}
	return pairs
}

// expandedLayout computes the number of column groups and rows for the expanded
// view given the available inner width. Each group is estimated at 40 chars.
func (h *Header) expandedLayout(width int) (numGroups, numRows int) {
	if width <= 0 {
		width = 80
	}
	pairs := h.collectPairs()
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

// ExpandedHeight returns the number of rows the header needs when expanded,
// taking the current inner width into account to compute the column layout.
func (h *Header) ExpandedHeight() int {
	_, _, width, _ := h.Table.GetInnerRect()
	_, numRows := h.expandedLayout(width)
	return numRows + 2 // +2 for top/bottom borders
}

// renderExpanded draws all pairs in a column-major multi-column grid, filling
// as many column groups as the available width allows.
func (h *Header) renderExpanded() {
	h.Table.Clear()
	pairs := h.collectPairs()
	if len(pairs) == 0 {
		return
	}

	_, _, width, _ := h.Table.GetInnerRect()
	numGroups, numRows := h.expandedLayout(width)

	for i, p := range pairs {
		row := i % numRows
		group := i / numRows
		col := group * 2 // each group occupies 2 table columns (key + value)
		h.Table.SetCell(row, col, h.keyCell(p.label))
		h.Table.SetCell(row, col+1, h.valueCell(p.value))
		// Spacer between groups
		if group < numGroups-1 {
			h.Table.SetCell(row, col+2, tview.NewTableCell(" "))
		}
	}
}

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
		order := order(i)
		h.Table.SetCell(currRow, currCol, h.keyCell(base[order].label))
		h.Table.SetCell(currRow, currCol+1, h.valueCell(base[order].value))
		currRow++
	}

	h.Table.SetCell(0, 2, tview.NewTableCell(" "))
	h.Table.SetCell(1, 2, tview.NewTableCell(" "))
	currCol++

	k, err := h.UpdateKeys()
	if err != nil {
		currCol += 2
		h.Table.SetCell(0, currCol, h.keyCell("No special keys for this element"))
		h.Table.SetCell(1, currCol, h.valueCell("Press "+"<"+h.App.GetKeys().Global.ToggleFullScreenHelp.String()+">"+" to see available keybindings"))
		return
	}

	for _, key := range k {
		if currRow%maxInRow == 0 && currRow != 0 {
			currCol += 2
			currRow = 0
		}
		var keyString string
		var iter []string
		if len(key.Keys) > 0 {
			iter = append(iter, key.Keys...)
		}
		if len(key.Runes) > 0 {
			iter = append(iter, key.Runes...)
		}
		for i, k := range iter {
			if i == 0 {
				keyString = k
			} else {
				keyString = fmt.Sprintf("%s, %s", keyString, k)
			}
		}

		h.Table.SetCell(currRow, currCol, h.keyCell(keyString))
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

func (h *Header) UpdateKeys() ([]config.Key, error) {
	if h.currentFocus == "" {
		return nil, nil
	}

	// SchemaTree reports as "SchemaTree", but keys are under "Schema"
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
