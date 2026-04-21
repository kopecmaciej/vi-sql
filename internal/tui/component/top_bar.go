package component

import (
	"context"
	"fmt"
	"time"

	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/gdamore/tcell/v2"
)

const (
	TopBarId = "TopBar"

	connInfoWidth    = 26
	mcpIndicatorWidth = 7
	pingInterval     = 30 * time.Second
	pingTimeout      = 5 * time.Second
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

	tabBar      *TabBar
	connText    *tview.TextView
	mcpText     *tview.TextView
	health      healthState
	stopMonitor context.CancelFunc
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

	t.mcpText = tview.NewTextView()
	t.mcpText.SetDynamicColors(true)
	t.mcpText.SetTextAlign(tview.AlignRight)

	t.Flex.SetDirection(tview.FlexColumn)
	t.Flex.SetBorder(true)
	t.setStyle()

	t.Flex.AddItem(t.tabBar, 0, 1, false)
	t.Flex.AddItem(t.mcpText, mcpIndicatorWidth, 0, false)
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
	bg := styles.Global.BackgroundColor.Color()
	t.connText.SetBackgroundColor(bg)
	t.mcpText.SetBackgroundColor(bg)
}

func (t *TopBar) updateConnText() {
	styles := t.App.GetStyles()
	conn := t.App.GetConfig().GetCurrentConnection()
	if conn == nil {
		t.connText.SetText(fmt.Sprintf("[%s]not connected[-] ", styles.Global.TitleColor.String()))
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

	const (
		colorGreen = "#4ADE80"
		colorRed   = "#F87171"
		colorGray  = "#64748B"
	)

	dot := fmt.Sprintf("[%s]●[-]", colorGray)
	extra := ""

	if t.health.checked {
		if t.health.connected {
			dot = fmt.Sprintf("[%s]●[-]", colorGreen)
			extra = fmt.Sprintf(" [%s]%s[-]", colorGray, formatPingLatency(t.health.latency))
		} else {
			dot = fmt.Sprintf("[%s]●[-]", colorRed)
			extra = fmt.Sprintf(" [%s]Ctrl+O[-]", colorRed)
		}
	}

	valColor := styles.Global.TitleColor.String()
	t.connText.SetText(fmt.Sprintf("%s [%s]%s:%d[-]%s ", dot, valColor, host, port, extra))
}

func (t *TopBar) updateMCPText() {
	const (
		colorGreen = "#4ADE80"
		colorGray  = "#64748B"
	)
	if t.App.IsMCPEnabled() {
		t.mcpText.SetTextColor(tcell.GetColor(colorGreen))
		t.mcpText.SetText(" MCP ")
	} else {
		t.mcpText.SetTextColor(tcell.GetColor(colorGray))
		t.mcpText.SetText("       ")
	}
}

// Render updates the connection info text and renders the tab bar.
func (t *TopBar) Render() {
	t.updateConnText()
	t.updateMCPText()
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
			})
		case manager.MCPStateChanged:
			t.App.QueueUpdateDraw(func() {
				t.updateMCPText()
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

// AddDynamicTabWithID is like AddDynamicTab but stamps the component with a stable tabID.
func (t *TopBar) AddDynamicTabWithID(name, tabID string, component TabBarPrimitive) int {
	return t.tabBar.AddDynamicTabWithID(name, tabID, component)
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

func formatPingLatency(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dμs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
