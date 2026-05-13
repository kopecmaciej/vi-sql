package page

import (
	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/tui/component"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

const (
	DriverPickerPageId = "DriverPicker"

	tileWidth  = 22
	tileHeight = 7
	tileGap    = 2
)

type DriverPicker struct {
	*core.BaseElement
	*core.Flex

	footer *component.Footer

	drivers     []string
	tiles       []*core.Flex
	cursor      int
	tilesPerRow int
	lastWidth   int

	onSelect func(driver string)
	onCancel func()
}

func NewDriverPicker() *DriverPicker {
	dp := &DriverPicker{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		footer:      component.NewFooter(),
	}
	dp.SetIdentifier(DriverPickerPageId)
	dp.footer.SetIdentifier("DriverPickerFooter")
	dp.SetTitle(" Pick Driver ")
	dp.SetBorder(true)
	dp.footer.SetCentered(true)
	return dp
}

func (dp *DriverPicker) SetOnSelectFunc(fn func(driver string)) { dp.onSelect = fn }
func (dp *DriverPicker) SetOnCancelFunc(fn func())              { dp.onCancel = fn }

func (dp *DriverPicker) Init(app *core.App) error {
	dp.App = app
	return dp.footer.Init(app)
}

func (dp *DriverPicker) Draw(screen tcell.Screen) {
	screenWidth, _ := screen.Size()
	if screenWidth != dp.lastWidth {
		dp.lastWidth = screenWidth
		newTilesPerRow := max(1, screenWidth/(tileWidth+tileGap))
		if newTilesPerRow != dp.tilesPerRow {
			dp.tilesPerRow = newTilesPerRow
			dp.Render()
		}
	}
	dp.Flex.Draw(screen)
}

func (dp *DriverPicker) Render() {
	dp.Clear()
	dp.SetDirection(tview.FlexRow)
	dp.SetStyle(dp.App.GetStyles())

	dp.drivers = database.ListConnectors()
	if dp.cursor >= len(dp.drivers) {
		dp.cursor = 0
	}
	if dp.tilesPerRow == 0 {
		dp.tilesPerRow = len(dp.drivers)
	}

	icons := dp.App.GetStyles().Icons
	dp.tiles = make([]*core.Flex, len(dp.drivers))

	dp.AddItem(tview.NewBox(), 0, 1, false)
	for rowStart := 0; rowStart < len(dp.drivers); rowStart += dp.tilesPerRow {
		rowEnd := min(rowStart+dp.tilesPerRow, len(dp.drivers))
		row := core.NewFlex()
		row.SetDirection(tview.FlexColumn)
		row.AddItem(tview.NewBox(), 0, 1, false)
		for i := rowStart; i < rowEnd; i++ {
			tile := dp.buildTile(icons.DriverIcon(dp.drivers[i]), dp.drivers[i])
			dp.tiles[i] = tile
			row.AddItem(tile, tileWidth, 0, false)
			if i < rowEnd-1 {
				row.AddItem(tview.NewBox(), tileGap, 0, false)
			}
		}
		row.AddItem(tview.NewBox(), 0, 1, false)
		dp.AddItem(row, tileHeight, 0, false)
		if rowEnd < len(dp.drivers) {
			dp.AddItem(tview.NewBox(), 1, 0, false)
		}
	}
	dp.AddItem(tview.NewBox(), 0, 1, false)

	dp.renderFooter()
	dp.AddItem(dp.footer, 2, 0, false)

	dp.applyTileStyles()

	k := dp.App.GetKeys()
	dp.SetInputCapture(k.WrapInputCapture(dp.handleKey))
}

func (dp *DriverPicker) buildTile(icon, name string) *core.Flex {
	tile := core.NewFlex()
	tile.SetDirection(tview.FlexRow)
	tile.SetBorder(true)

	iconView := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetDynamicColors(true).
		SetText(icon)
	nameView := tview.NewTextView().
		SetTextAlign(tview.AlignCenter).
		SetText(name)

	tile.AddItem(tview.NewBox(), 0, 1, false)
	tile.AddItem(iconView, 1, 0, false)
	tile.AddItem(tview.NewBox(), 0, 1, false)
	tile.AddItem(nameView, 1, 0, false)
	tile.AddItem(tview.NewBox(), 0, 1, false)
	return tile
}

func (dp *DriverPicker) applyTileStyles() {
	g := dp.App.GetStyles().Global
	for i, tile := range dp.tiles {
		if i == dp.cursor {
			tile.SetBorderColor(g.FocusColor.Color())
		} else {
			tile.SetBorderColor(g.BorderColor.Color())
		}
	}
}

// lastRowInfo returns the start index, count, and visual-column offset of the last row.
// The last row is centered: its tiles occupy visual columns [offset, offset+count-1]
// within the tilesPerRow-wide grid. offset = (tilesPerRow - count) / 2.
func (dp *DriverPicker) lastRowInfo() (start, count, offset int) {
	count = len(dp.drivers) % dp.tilesPerRow
	if count == 0 {
		count = dp.tilesPerRow
	}
	start = len(dp.drivers) - count
	offset = (dp.tilesPerRow - count) / 2
	return
}

func (dp *DriverPicker) moveCursorDown() {
	lastStart, lastCount, offset := dp.lastRowInfo()
	currentRow := dp.cursor / dp.tilesPerRow
	lastRow := lastStart / dp.tilesPerRow
	if currentRow >= lastRow {
		return
	}
	if currentRow+1 == lastRow {
		// Next row is the partial, centered last row.
		col := dp.cursor % dp.tilesPerRow
		pos := col - offset
		if pos < 0 {
			pos = 0
		} else if pos >= lastCount {
			pos = lastCount - 1
		}
		dp.cursor = lastStart + pos
	} else {
		dp.cursor += dp.tilesPerRow
	}
	dp.applyTileStyles()
}

func (dp *DriverPicker) moveCursorUp() {
	lastStart, _, offset := dp.lastRowInfo()
	currentRow := dp.cursor / dp.tilesPerRow
	lastRow := lastStart / dp.tilesPerRow
	if currentRow == 0 {
		return
	}
	if currentRow == lastRow {
		// Moving from the partial, centered last row back to the full row above.
		posInLastRow := dp.cursor - lastStart
		visualCol := offset + posInLastRow
		dp.cursor = lastStart - dp.tilesPerRow + visualCol
	} else {
		dp.cursor -= dp.tilesPerRow
	}
	dp.applyTileStyles()
}

func (dp *DriverPicker) handleKey(event *tcell.EventKey) *tcell.EventKey {
	k := dp.App.GetKeys()
	switch {
	case k.Match(k.Navigation.MoveLeft, event):
		if dp.cursor > 0 {
			dp.cursor--
		} else {
			dp.cursor = len(dp.drivers) - 1
		}
		dp.applyTileStyles()
		return nil
	case k.Match(k.Navigation.MoveRight, event):
		if dp.cursor < len(dp.drivers)-1 {
			dp.cursor++
		} else {
			dp.cursor = 0
		}
		dp.applyTileStyles()
		return nil
	case k.Match(k.Navigation.MoveUp, event):
		dp.moveCursorUp()
		return nil
	case k.Match(k.Navigation.MoveDown, event):
		dp.moveCursorDown()
		return nil
	case k.Match(k.Common.Select, event):
		if dp.onSelect != nil {
			dp.onSelect(dp.drivers[dp.cursor])
		}
		return nil
	}
	if event.Key() == tcell.KeyEscape {
		if dp.onCancel != nil {
			dp.onCancel()
		}
		return nil
	}
	return event
}

func (dp *DriverPicker) renderFooter() {
	k := dp.App.GetKeys()
	keys := []config.Key{k.Navigation.MoveLeft, k.Navigation.MoveRight}
	if len(dp.drivers) > dp.tilesPerRow {
		keys = append(keys, k.Navigation.MoveUp, k.Navigation.MoveDown)
	}
	keys = append(keys, k.Common.Close, k.Common.Select)
	dp.footer.SetKeys(keys)
}
