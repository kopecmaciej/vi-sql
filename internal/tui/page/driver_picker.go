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

	drivers []string
	tiles   []*core.Flex
	cursor  int

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
	dp.footer.SetCentered(true)
	return dp
}

func (dp *DriverPicker) SetOnSelectFunc(fn func(driver string)) { dp.onSelect = fn }
func (dp *DriverPicker) SetOnCancelFunc(fn func())              { dp.onCancel = fn }

func (dp *DriverPicker) Init(app *core.App) error {
	dp.App = app
	return dp.footer.Init(app)
}

func (dp *DriverPicker) Render() {
	dp.Clear()
	dp.SetDirection(tview.FlexRow)
	dp.SetStyle(dp.App.GetStyles())

	dp.drivers = database.ListConnectors()
	if dp.cursor >= len(dp.drivers) {
		dp.cursor = 0
	}

	icons := dp.App.GetStyles().Icons
	dp.tiles = make([]*core.Flex, len(dp.drivers))

	row := core.NewFlex()
	row.SetDirection(tview.FlexColumn)
	row.AddItem(tview.NewBox(), 0, 1, false)
	for i, d := range dp.drivers {
		tile := dp.buildTile(icons.DriverIcon(d), d)
		dp.tiles[i] = tile
		row.AddItem(tile, tileWidth, 0, false)
		if i < len(dp.drivers)-1 {
			row.AddItem(tview.NewBox(), tileGap, 0, false)
		}
	}
	row.AddItem(tview.NewBox(), 0, 1, false)

	dp.AddItem(tview.NewBox(), 0, 1, false)
	dp.AddItem(row, tileHeight, 0, true)
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

func (dp *DriverPicker) handleKey(event *tcell.EventKey) *tcell.EventKey {
	k := dp.App.GetKeys()
	switch {
	case k.Match(k.Navigation.MoveLeft, event):
		if dp.cursor > 0 {
			dp.cursor--
			dp.applyTileStyles()
		}
		return nil
	case k.Match(k.Navigation.MoveRight, event):
		if dp.cursor < len(dp.drivers)-1 {
			dp.cursor++
			dp.applyTileStyles()
		}
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
	dp.footer.SetKeys([]config.Key{
		k.Navigation.MoveLeft,
		k.Navigation.MoveRight,
		k.Common.Close,
		k.Common.Select,
	})
}
