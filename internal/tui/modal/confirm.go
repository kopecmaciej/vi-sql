package modal

import (
	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

type Confirm struct {
	*core.BaseElement
	*core.Modal

	confirmLabel string
	style        *config.OthersStyle
	onConfirm    func()
	onCancel     func()
}

func NewConfirm(id tview.Identifier) *Confirm {
	dm := &Confirm{
		BaseElement:  core.NewBaseElement(),
		Modal:        core.NewModal(),
		confirmLabel: "Confirm",
	}

	dm.SetIdentifier(id)
	dm.SetAfterInitFunc(dm.init)

	return dm
}

func (c *Confirm) init() error {
	c.setLayout()
	c.setStyle()
	c.setKeybindings()
	c.handleEvents()

	return nil
}

func (c *Confirm) setLayout() {
	c.AddButtons([]string{c.confirmLabel, "Cancel"})
	c.SetBorder(true)
	c.SetTitle(" " + c.confirmLabel + " ")
	c.SetBorderPadding(0, 0, 1, 1)
	c.Modal.SetDoneFunc(func(buttonIndex int, _ string) {
		if buttonIndex == 0 {
			if c.onConfirm != nil {
				c.onConfirm()
			}
		} else if c.onCancel != nil {
			c.onCancel()
		} else {
			c.App.Pages.RemovePage(c.GetIdentifier())
		}
	})
}

func (c *Confirm) setStyle() {
	c.SetStyle(c.App.GetStyles())
	c.style = &c.App.GetStyles().Others

	c.SetButtonActivatedStyle(tcell.StyleDefault.
		Background(c.style.DeleteButtonSelectedBackgroundColor.Color()))
}

func (c *Confirm) setKeybindings() {
	kb := c.App.GetKeys()
	c.SetInputCapture(kb.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case kb.Match(kb.Navigation.MoveLeft, event):
			return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
		case kb.Match(kb.Navigation.MoveRight, event):
			return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
		}
		return event
	}))
}

func (c *Confirm) handleEvents() {
	go c.HandleEvents(c.GetIdentifier(), func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			c.setStyle()
		}
	})
}

func (c *Confirm) SetConfirmButtonLabel(label string) {
	c.confirmLabel = label
	c.ClearButtons()
	c.AddButtons([]string{label, "Cancel"})
}

func (c *Confirm) SetOnConfirm(fn func()) {
	c.onConfirm = fn
}

func (c *Confirm) SetOnCancel(fn func()) {
	c.onCancel = fn
}
