package page

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/component"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

const (
	ConnectionPageId = "Connection"
)

type Connection struct {
	*core.BaseElement
	*core.Flex

	list   *core.List
	footer *component.Footer

	style *config.ConnectionStyle

	onSubmit func()
}

func NewConnection() *Connection {
	c := &Connection{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		list:        core.NewList(),
		footer:      component.NewFooter(),
	}
	c.footer.SetCentered(true)

	c.SetIdentifier(ConnectionPageId)

	return c
}

func (c *Connection) Init(app *core.App) error {
	c.App = app

	if err := c.footer.Init(app); err != nil {
		return err
	}
	c.setLayout()
	c.setStyle()
	c.setKeybindings()
	c.handleEvents()
	return nil
}

func (c *Connection) handleEvents() {
	go c.HandleEvents(ConnectionPageId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			c.setStyle()
			go c.App.QueueUpdateDraw(func() {
				c.Render()
			})
		}
	})
}

func (c *Connection) setLayout() {
	c.list.SetTitle(" Saved connections ")
	c.list.SetBorder(true)
	c.list.ShowSecondaryText(true)
	c.list.SetWrapText(true)
	c.list.SetBorderPadding(1, 1, 1, 1)
	c.list.SetItemGap(1)

}

func (c *Connection) setStyle() {
	c.SetStyle(c.App.GetStyles())
	c.list.SetStyle(c.App.GetStyles())

	c.style = &c.App.GetStyles().Connection

	globalBackground := c.App.GetStyles().Global.BackgroundColor.Color()
	mainStyle := tcell.StyleDefault.
		Foreground(c.style.ListTextColor.Color()).
		Background(globalBackground)
	c.list.SetMainTextStyle(mainStyle)

	secondaryStyle := tcell.StyleDefault.
		Foreground(c.style.ListSecondaryTextColor.Color()).
		Background(c.App.GetStyles().Global.BackgroundColor.Color()).
		Italic(true)
	c.list.SetSecondaryTextStyle(secondaryStyle)
	c.list.SetSelectedStyle(tcell.StyleDefault.
		Foreground(c.style.ListSelectedTextColor.Color()).
		Background(c.style.ListSelectedBackgroundColor.Color()))
}

func (c *Connection) setKeybindings() {
	k := c.App.GetKeys()
	c.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Contains(k.Connection.ConnectionList.AddConnection, event.Name()):
			c.openAddForm()
			return nil
		case k.Contains(k.Connection.ConnectionList.DeleteConnection, event.Name()):
			c.deleteCurrConnection()
			return nil
		case k.Contains(k.Connection.ConnectionList.EditConnection, event.Name()):
			c.openEditForm()
			return nil
		}
		return event
	})
}

func (c *Connection) openAddForm() {
	form := NewConnectionForm(nil)
	if err := form.Init(c.App); err != nil {
		showError(c.App.Pages, "Failed to init connection form", err)
		return
	}
	form.SetOnSaveFunc(func() {
		c.App.Pages.RemovePage(ConnectionFormPageId)
		c.Render()
		c.list.SetCurrentItem(c.list.GetItemCount() - 2)
		c.App.SetFocus(c.list)
	})
	form.SetOnCancelFunc(func() {
		c.App.Pages.RemovePage(ConnectionFormPageId)
		c.App.SetFocus(c.list)
	})
	form.Render()
	c.App.Pages.AddPage(ConnectionFormPageId, form, true, true)
}

func (c *Connection) openEditForm() {
	currItem := c.list.GetCurrentItem()
	if currItem >= c.list.GetItemCount()-1 {
		return
	}
	connName, _ := c.list.GetItemText(currItem)
	conn, err := c.App.GetConfig().GetConnectionByName(connName)
	if err != nil {
		showError(c.App.Pages, "Failed to load connection for editing", err)
		return
	}

	form := NewConnectionForm(conn)
	if err := form.Init(c.App); err != nil {
		showError(c.App.Pages, "Failed to init connection form", err)
		return
	}
	form.SetOnSaveFunc(func() {
		c.App.Pages.RemovePage(ConnectionFormPageId)
		c.Render()
		c.list.SetCurrentItem(currItem)
		c.App.SetFocus(c.list)
	})
	form.SetOnCancelFunc(func() {
		c.App.Pages.RemovePage(ConnectionFormPageId)
		c.App.SetFocus(c.list)
	})
	form.Render()
	c.App.Pages.AddPage(ConnectionFormPageId, form, true, true)
}

func (c *Connection) Render() {
	c.Clear()
	c.SetDirection(tview.FlexRow)

	centerFlex := tview.NewFlex()
	centerFlex.AddItem(tview.NewBox(), 0, 1, false)
	c.renderList(centerFlex)
	centerFlex.AddItem(tview.NewBox(), 0, 1, false)

	c.AddItem(centerFlex, 0, 1, true)
	c.renderFooter()
	c.AddItem(c.footer, 2, 0, false)

	if page, _ := c.App.Pages.GetFrontPage(); page == ConnectionPageId {
		if c.list.GetItemCount() > 1 {
			defer c.App.SetFocus(c.list)
		} else {
			// No saved connections — open add form immediately
			defer c.openAddForm()
		}
	}
}

func (c *Connection) renderFooter() {
	k := c.App.GetKeys()
	c.footer.SetKeys([]config.Key{
		k.Connection.ConnectionList.SetConnection,
		k.Connection.ConnectionList.AddConnection,
		k.Connection.ConnectionList.EditConnection,
		k.Connection.ConnectionList.DeleteConnection,
		k.Global.FullScreenHelp,
	})
}

func (c *Connection) renderList(centerFlex *tview.Flex) {
	c.list.Clear()

	for _, conn := range c.App.GetConfig().Connections {
		var dsnDisplay string
		trimmed := strings.TrimSpace(conn.DSN)
		if strings.HasPrefix(trimmed, "$") {
			dsnDisplay = "dsn: " + trimmed
		} else {
			dsnDisplay = "dsn: " + conn.GetSafeDSN()
		}
		c.list.AddItem(conn.Name, dsnDisplay, 0, func() {
			c.setConnection()
		})
	}

	c.list.AddItem("[Add new connection]", "", 0, func() {
		c.openAddForm()
	})

	centerFlex.AddItem(c.list, 0, 3, true)
}

func (c *Connection) setConnection() {
	if c.list.GetCurrentItem() == c.list.GetItemCount()-1 {
		return
	}
	connName, _ := c.list.GetItemText(c.list.GetCurrentItem())
	err := c.App.GetConfig().SetCurrentConnection(connName)
	if err != nil {
		showError(c.App.Pages, "Failed to set current connection", err)
		return
	}
	c.App.GetConfig().CurrentConnection = connName
	if c.onSubmit != nil {
		c.onSubmit()
	}
}

func (c *Connection) deleteCurrConnection() {
	currItem := c.list.GetCurrentItem()
	if currItem >= c.list.GetItemCount()-1 {
		return
	}
	currConn, _ := c.list.GetItemText(currItem)
	err := c.App.GetConfig().DeleteConnection(currConn)
	if err != nil {
		showError(c.App.Pages, "Failed to delete connection", err)
		return
	}

	c.Render()
	if currItem > 0 {
		c.list.SetCurrentItem(currItem - 1)
	}
}

func (c *Connection) SetOnSubmitFunc(onSubmit func()) {
	c.onSubmit = onSubmit
}
