package page

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

const (
	ConnectionPageId = "Connection"

	// List width breakpoints (terminal columns).
	// Below connNarrowBreakpoint the list fills full width (no side padding).
	// Below connWideBreakpoint the list is capped at connWidthMedium.
	// At connWideBreakpoint and above the list is capped at connWidthWide.
	connNarrowBreakpoint = 80
	connWideBreakpoint   = 140
	connWidthMedium      = 70
	connWidthWide        = 90
)

type Connection struct {
	*core.BaseElement
	*core.Flex

	list     *core.List
	leftPad  *tview.Box
	rightPad *tview.Box

	style *config.ConnectionStyle

	onSubmit func()
}

func NewConnection() *Connection {
	c := &Connection{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		list:        core.NewList(),
	}

	c.SetIdentifier(ConnectionPageId)

	return c
}

func (c *Connection) Init(app *core.App) {
	c.App = app

	c.setLayout()
	c.setStyle()
	c.setKeybindings()

	c.handleEvents()
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
		Background(c.style.ListSecondaryBackgroundColor.Color()).
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
	form.Init(c.App)
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
	form.Init(c.App)
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

	c.leftPad = tview.NewBox()
	c.rightPad = tview.NewBox()
	c.AddItem(c.leftPad, 0, 1, false)

	if page, _ := c.App.Pages.GetFrontPage(); page == ConnectionPageId {
		c.renderList()
		if c.list.GetItemCount() > 1 {
			defer c.App.SetFocus(c.list)
		} else {
			// No saved connections — open add form immediately
			defer c.openAddForm()
		}
	}

	c.AddItem(c.rightPad, 0, 1, false)
}

// Draw adjusts the list width responsively before each render:
//   - narrow  (< connNarrowBreakpoint): list fills the full width
//   - medium  (connNarrowBreakpoint – connWideBreakpoint-1): capped at connWidthMedium, centered
//   - wide    (≥ connWideBreakpoint): capped at connWidthWide, centered
func (c *Connection) Draw(screen tcell.Screen) {
	if c.leftPad != nil && c.rightPad != nil {
		w, _ := screen.Size()
		switch {
		case w < connNarrowBreakpoint:
			c.ResizeItem(c.leftPad, 0, 0)
			c.ResizeItem(c.list, 0, 1)
			c.ResizeItem(c.rightPad, 0, 0)
		case w < connWideBreakpoint:
			c.ResizeItem(c.leftPad, 0, 1)
			c.ResizeItem(c.list, connWidthMedium, 0)
			c.ResizeItem(c.rightPad, 0, 1)
		default:
			c.ResizeItem(c.leftPad, 0, 1)
			c.ResizeItem(c.list, connWidthWide, 0)
			c.ResizeItem(c.rightPad, 0, 1)
		}
	}
	c.Flex.Draw(screen)
}

func (c *Connection) renderList() {
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

	addKey := c.App.GetKeys().Connection.ConnectionList.AddConnection.String()
	editKey := c.App.GetKeys().Connection.ConnectionList.EditConnection.String()
	deleteKey := c.App.GetKeys().Connection.ConnectionList.DeleteConnection.String()

	helpText := fmt.Sprintf("Add (%s) | Edit (%s) | Delete (%s)", addKey, editKey, deleteKey)
	c.list.AddItem("[Add new connection]", helpText, 0, func() {
		c.openAddForm()
	})

	c.AddItem(c.list, 0, 3, true)
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
