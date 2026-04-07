package component

import (
	"fmt"

	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

const (
	TopBarId = "TopBar"

	connInfoWidth = 20
)

// TopBar is a 1-row bar at the top of the main content area.
// It shows the active connection info on the left and the tab bar on the right.
type TopBar struct {
	*core.BaseElement
	*core.Flex

	tabBar   *TabBar
	connText *tview.TextView
}

func NewTopBar() *TopBar {
	t := &TopBar{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		tabBar:      NewTabBar(),
	}
	t.SetIdentifier(TopBarId)
	t.SetAfterInitFunc(t.init)
	return t
}

func (t *TopBar) init() error {
	if err := t.tabBar.Init(t.App); err != nil {
		return err
	}

	t.connText = tview.NewTextView()
	t.connText.SetDynamicColors(true)
	t.connText.SetTextAlign(tview.AlignRight)

	t.Flex.SetDirection(tview.FlexColumn)
	t.Flex.SetBorder(true)
	t.setStyle()

	t.Flex.AddItem(t.tabBar, 0, 1, false)
	t.Flex.AddItem(t.connText, connInfoWidth, 0, false)

	t.handleEvents()
	return nil
}

func (t *TopBar) setStyle() {
	styles := t.App.GetStyles()
	t.Flex.SetStyle(styles)
	t.connText.SetBackgroundColor(styles.Global.BackgroundColor.Color())
}

func (t *TopBar) updateConnText() {
	styles := t.App.GetStyles()
	conn := t.App.GetConfig().GetCurrentConnection()
	if conn == nil {
		t.connText.SetText(fmt.Sprintf("[%s]not connected[-] ", styles.Header.ValueColor.String()))
		return
	}

	host := conn.Host
	port := conn.Port

	if (host == "" || port == 0) && conn.DSN != "" {
		if parsed, err := util.ParsePostgresDSN(conn.GetDSN()); err == nil {
			host = parsed.Host
			var p int
			if n, _ := fmt.Sscanf(parsed.Port, "%d", &p); n == 1 {
				port = p
			}
		}
	}

	valColor := styles.Header.ValueColor.String()
	t.connText.SetText(fmt.Sprintf("[%s]%s:%d[-] ", valColor, host, port))
}

// Render updates the connection info text and renders the tab bar.
func (t *TopBar) Render() {
	t.updateConnText()
	t.tabBar.Render()
}

func (t *TopBar) handleEvents() {
	go t.HandleEvents(TopBarId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			t.App.QueueUpdateDraw(func() {
				t.setStyle()
				t.updateConnText()
			})
		}
	})
}

// AddTab registers a new tab. Delegates to the internal TabBar.
func (t *TopBar) AddTab(name string, component TabBarPrimitive, defaultTab bool) {
	t.tabBar.AddTab(name, component, defaultTab)
}

// AddDynamicTab adds a tab at runtime and activates it. Returns its index.
func (t *TopBar) AddDynamicTab(name string, component TabBarPrimitive) int {
	return t.tabBar.AddDynamicTab(name, component)
}

// CloseActiveTab removes the active tab. Does nothing if only one tab remains.
func (t *TopBar) CloseActiveTab() {
	t.tabBar.CloseActiveTab()
}

// HasTabs reports whether any tabs are registered.
func (t *TopBar) HasTabs() bool {
	return t.tabBar.HasTabs()
}

// ClearAllTabs removes all tabs from the tab bar.
func (t *TopBar) ClearAllTabs() {
	t.tabBar.ClearAllTabs()
}

// NextTab switches to the next tab.
func (t *TopBar) NextTab() {
	t.tabBar.NextTab()
}

// PreviousTab switches to the previous tab.
func (t *TopBar) PreviousTab() {
	t.tabBar.PreviousTab()
}

// GetActiveComponent returns the primitive of the currently active tab.
func (t *TopBar) GetActiveComponent() TabBarPrimitive {
	return t.tabBar.GetActiveComponent()
}

// GetActiveComponentAndRender returns the active tab's primitive, rendering it
// if it hasn't been rendered yet.
func (t *TopBar) GetActiveComponentAndRender() TabBarPrimitive {
	return t.tabBar.GetActiveComponentAndRender()
}

// GetActiveTabIndex returns the index of the currently active tab.
func (t *TopBar) GetActiveTabIndex() int {
	return t.tabBar.GetActiveTabIndex()
}

// GetActiveTabName returns the name of the currently active tab.
func (t *TopBar) GetActiveTabName() string {
	return t.tabBar.GetActiveTabName()
}

// SwitchToTabByName activates the first tab with the given name.
// Returns true if a matching tab was found.
func (t *TopBar) SwitchToTabByName(name string) bool {
	return t.tabBar.SwitchToTabByName(name)
}

// GetTabCount returns the total number of registered tabs.
func (t *TopBar) GetTabCount() int {
	return t.tabBar.GetTabCount()
}

// ResetRendered clears the rendered flag on all tabs.
func (t *TopBar) ResetRendered() {
	t.tabBar.ResetRendered()
}
