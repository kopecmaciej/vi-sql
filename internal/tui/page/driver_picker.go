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
)

type DriverPicker struct {
	*core.BaseElement
	*core.Flex

	buttons *tview.ButtonGroup
	footer  *component.Footer

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

func (dp *DriverPicker) SetOnSelectFunc(fn func(driver string)) {
	dp.onSelect = fn
}

func (dp *DriverPicker) SetOnCancelFunc(fn func()) {
	dp.onCancel = fn
}

func (dp *DriverPicker) Init(app *core.App) error {
	dp.App = app
	return dp.footer.Init(app)
}

func (dp *DriverPicker) Render() {
	dp.Clear()
	dp.SetDirection(tview.FlexRow)
	dp.SetStyle(dp.App.GetStyles())

	drivers := database.ListConnectors()
	icons := dp.App.GetStyles().Icons

	driverIcons := make([]string, len(drivers))
	for i, d := range drivers {
		driverIcons[i] = icons.DriverIcon(d)
	}

	styles := dp.App.GetStyles()
	g := styles.Global

	dp.buttons = tview.NewButtonGroup("", drivers, 0, func(option string, _ int) {
		if dp.onSelect != nil {
			dp.onSelect(option)
		}
	})
	dp.buttons.SetIcons(driverIcons)
	dp.buttons.SetUnselectedStyle(
		tcell.StyleDefault.
			Foreground(g.TextColor.Color()),
	)
	dp.buttons.SetCursorStyle(
		tcell.StyleDefault.
			Background(g.FocusColor.Color()).
			Foreground(g.BackgroundColor.Color()),
	)
	dp.buttons.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape && dp.onCancel != nil {
			dp.onCancel()
		}
	})

	k := dp.App.GetKeys()
	dp.buttons.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		return event
	}))

	dp.AddItem(tview.NewBox(), 0, 1, false)
	dp.AddItem(dp.buttons, 6, 0, true)
	dp.AddItem(tview.NewBox(), 0, 1, false)

	dp.renderFooter()
	dp.AddItem(dp.footer, 2, 0, false)

	dp.App.SetFocus(dp.buttons)
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
