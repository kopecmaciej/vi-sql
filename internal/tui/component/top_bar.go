package component

import (
	"context"
	"fmt"
	"time"

	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/widget"
)

const (
	TopBarId = "TopBar"

	connInfoWidth         = 8
	defaultIndicatorWidth = 4
	pingInterval          = 30 * time.Second
	pingTimeout           = 5 * time.Second
)

// healthState holds the result of the most recent ping.
type healthState struct {
	checked   bool
	connected bool
	latency   time.Duration
}

// TopBar is a 1-row bar at the top of the main content area.
// It shows the active connection info on the left and the tab bar on the right.
type TopBar struct {
	*core.BaseElement
	*core.Flex

	tabBar      *widget.TabBar
	connText    *tview.TextView
	mcpText     *tview.TextView
	vimText     *tview.TextView
	updateText  *tview.TextView
	health      healthState
	stopMonitor context.CancelFunc
}

func NewTopBar() *TopBar {
	t := &TopBar{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		tabBar:      widget.NewTabBar(),
	}
	t.SetIdentifier(TopBarId)
	t.SetAfterInitFunc(t.init)
	return t
}

func (t *TopBar) init() error {
	t.connText = tview.NewTextView()
	t.connText.SetDynamicColors(true)
	t.connText.SetTextAlign(tview.AlignRight)

	t.mcpText = tview.NewTextView()
	t.mcpText.SetDynamicColors(true)
	t.mcpText.SetTextAlign(tview.AlignCenter)

	t.vimText = tview.NewTextView()
	t.vimText.SetDynamicColors(true)
	t.vimText.SetTextAlign(tview.AlignCenter)

	t.updateText = tview.NewTextView()
	t.updateText.SetDynamicColors(true)
	t.updateText.SetTextAlign(tview.AlignCenter)

	t.Flex.SetDirection(tview.FlexColumn)
	t.Flex.SetBorder(true)
	t.setStyle()

	t.Flex.AddItem(t.tabBar, 0, 1, false)
	t.Flex.AddItem(t.updateText, 0, 0, false)
	t.Flex.AddItem(t.vimText, defaultIndicatorWidth, 0, false)
	t.Flex.AddItem(t.mcpText, defaultIndicatorWidth, 0, false)
	t.Flex.AddItem(t.connText, connInfoWidth, 0, false)

	t.handleEvents()
	t.startHealthMonitor()
	return nil
}

// UpdateDriver overrides BaseElement.UpdateDriver to restart the health monitor
// when a new database connection is established.
func (t *TopBar) UpdateDriver(driver database.Driver) {
	t.BaseElement.UpdateDriver(driver)
	t.health = healthState{}
	t.startHealthMonitor()
}

func (t *TopBar) startHealthMonitor() {
	if t.stopMonitor != nil {
		t.stopMonitor()
	}
	if t.Driver == nil {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.stopMonitor = cancel

	go func() {
		t.doPing(ctx)

		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				t.doPing(ctx)
			}
		}
	}()
}

func (t *TopBar) doPing(ctx context.Context) {
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	start := time.Now()
	err := t.Driver.Ping(pingCtx)
	latency := time.Since(start)

	t.health = healthState{
		checked:   true,
		connected: err == nil,
		latency:   latency,
	}
	t.App.QueueUpdateDraw(func() {
		t.updateConnText()
	})
}

func (t *TopBar) setStyle() {
	styles := t.App.GetStyles()
	t.Flex.SetStyle(styles)
	t.tabBar.SetStyle(styles)
	bg := styles.Global.BackgroundColor.Color()
	t.connText.SetBackgroundColor(bg)
	t.mcpText.SetBackgroundColor(bg)
	t.vimText.SetBackgroundColor(bg)
	t.updateText.SetBackgroundColor(bg)
}

// SetUpdateAvailable shows the update icon in the top bar.
func (t *TopBar) SetUpdateAvailable() {
	g := t.App.GetStyles().Global
	sym := t.App.GetStyles().Icons
	t.updateText.SetText(fmt.Sprintf("[%s]%s[-]", g.WarningColor, string(sym.Update)))
	t.Flex.ResizeItem(t.updateText, defaultIndicatorWidth, 0)
}

func (t *TopBar) updateConnText() {
	g := t.App.GetStyles().Global
	sym := t.App.GetStyles().Icons

	if t.App.GetConfig().GetCurrentConnection() == nil {
		t.connText.SetText(fmt.Sprintf("[%s]%s ----[-] ", g.ErrorColor, string(sym.HealthDown)))
		return
	}

	indicator := fmt.Sprintf("[%s]%s[-]", g.DimColor, string(sym.HealthDown))
	extra := ""

	if t.health.checked {
		if t.health.connected {
			indicator = fmt.Sprintf("[%s]%s[-]", g.SuccessColor, string(sym.HealthUp))
			extra = fmt.Sprintf(" [%s]%s[-]", g.DimColor, formatPingLatency(t.health.latency))
		} else {
			indicator = fmt.Sprintf("[%s]%s[-]", g.ErrorColor, string(sym.HealthDown))
		}
	}

	t.connText.SetText(fmt.Sprintf("%s%s ", indicator, extra))
}

func (t *TopBar) updateMCPText() {
	g := t.App.GetStyles().Global
	sym := t.App.GetStyles().Icons
	if t.App.IsMCPEnabled() {
		t.mcpText.SetTextColor(g.SuccessColor.Color())
	} else {
		t.mcpText.SetTextColor(g.DimColor.Color())
	}
	t.mcpText.SetText(fmt.Sprintf(" %s ", string(sym.MCP)))
}

func (t *TopBar) updateVimText() {
	g := t.App.GetStyles().Global
	sym := t.App.GetStyles().Icons
	if !t.App.GetConfig().UI.VimMode {
		t.vimText.SetText("    ")
		return
	}
	t.vimText.SetTextColor(g.SuccessColor.Color())
	t.vimText.SetText(fmt.Sprintf(" %s ", string(sym.VimMode)))
}

// Render updates the connection info text and renders the tab bar.
func (t *TopBar) Render() {
	t.updateConnText()
	t.updateMCPText()
	t.updateVimText()
	t.tabBar.Render()
}

func (t *TopBar) handleEvents() {
	go t.HandleEvents(TopBarId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			t.App.QueueUpdateDraw(func() {
				t.setStyle()
				t.updateConnText()
				t.updateMCPText()
				t.updateVimText()
			})
		case manager.MCPStateChanged:
			t.App.QueueUpdateDraw(func() {
				t.updateMCPText()
			})
		case manager.ConfigChanged:
			t.App.QueueUpdateDraw(func() {
				t.updateVimText()
			})
		}
	})
}

// AddTab registers a new tab. Delegates to the internal TabBar.
func (t *TopBar) AddTab(name string, component widget.TabBarPrimitive, kind widget.TabKind, defaultTab bool) {
	t.tabBar.AddTab(name, component, kind, defaultTab)
}

// AddDynamicTab adds a tab at runtime and activates it. Returns its index.
func (t *TopBar) AddDynamicTab(name string, component widget.TabBarPrimitive, kind widget.TabKind) int {
	return t.tabBar.AddDynamicTab(name, component, kind)
}

// AddDynamicTabWithID is like AddDynamicTab but stamps the component with a stable tabID.
func (t *TopBar) AddDynamicTabWithID(name, tabID string, kind widget.TabKind, component widget.TabBarPrimitive) int {
	return t.tabBar.AddDynamicTabWithID(name, tabID, kind, component)
}

// GetActiveTabID returns the MCP-assigned tabID of the active tab, or empty string.
func (t *TopBar) GetActiveTabID() string {
	return t.tabBar.GetActiveTabID()
}

// RenameActiveTab updates the display name of the currently active tab.
func (t *TopBar) RenameActiveTab(newName string) {
	t.tabBar.RenameActiveTab(newName)
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
func (t *TopBar) GetActiveComponent() widget.TabBarPrimitive {
	return t.tabBar.GetActiveComponent()
}

// GetActiveComponentAndRender returns the active tab's primitive, rendering it
// if it hasn't been rendered yet.
func (t *TopBar) GetActiveComponentAndRender() widget.TabBarPrimitive {
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

// SwitchToTabByName activates the first tab matching both name and kind.
// Returns true if a matching tab was found.
func (t *TopBar) SwitchToTabByName(name string, kind widget.TabKind) bool {
	return t.tabBar.SwitchToTabByName(name, kind)
}

// GetTabCount returns the total number of registered tabs.
func (t *TopBar) GetTabCount() int {
	return t.tabBar.GetTabCount()
}

// ResetRendered clears the rendered flag on all tabs.
func (t *TopBar) ResetRendered() {
	t.tabBar.ResetRendered()
}

func formatPingLatency(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dμs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
