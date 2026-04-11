package component

import (
	"slices"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

const (
	TabBarId = "TabBar"
)

type TabBarPrimitive interface {
	tview.Primitive
	Render()
}

type TabBarComponent struct {
	id        string
	primitive TabBarPrimitive
	rendered  bool
}

type TabBar struct {
	*core.BaseElement
	*core.Table

	active int
	tabs   []*TabBarComponent
}

func NewTabBar() *TabBar {
	t := &TabBar{
		BaseElement: core.NewBaseElement(),
		Table:       core.NewTable(),
		tabs:        []*TabBarComponent{},
	}

	t.SetIdentifier(TabBarId)
	t.SetAfterInitFunc(t.init)

	return t
}

func (t *TabBar) init() error {
	t.setLayout()
	t.setStyle()

	t.handleEvents()
	return nil
}

func (t *TabBar) setStyle() {
	styles := t.App.GetStyles()
	t.SetStyle(styles)
}

func (t *TabBar) setLayout() {
	t.SetBorderPadding(0, 0, 1, 0)
}

func (t *TabBar) handleEvents() {
	go t.HandleEvents(TabBarId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			t.setStyle()
			t.Render()
		}
	})
}

func (t *TabBar) AddTab(name string, component TabBarPrimitive, defaultTab bool) {
	t.tabs = append(t.tabs, &TabBarComponent{
		id:        name,
		primitive: component,
	})
	if defaultTab {
		t.active = len(t.tabs) - 1
	}
	t.Render()
}

// AddDynamicTab adds a tab at runtime and activates it immediately.
// Returns the index of the new tab.
func (t *TabBar) AddDynamicTab(name string, component TabBarPrimitive) int {
	t.tabs = append(t.tabs, &TabBarComponent{
		id:        name,
		primitive: component,
	})
	t.active = len(t.tabs) - 1
	t.Render()
	return t.active
}

// CloseActiveTab removes the currently active tab. Does nothing if only one tab remains.
func (t *TabBar) CloseActiveTab() {
	if len(t.tabs) <= 1 {
		return
	}
	t.tabs = slices.Delete(t.tabs, t.active, t.active+1)
	if t.active >= len(t.tabs) {
		t.active = len(t.tabs) - 1
	}
	t.Render()
}

// HasTabs reports whether any tabs are registered.
func (t *TabBar) HasTabs() bool {
	return len(t.tabs) > 0
}

// ClearAllTabs removes every tab and resets the active index.
func (t *TabBar) ClearAllTabs() {
	t.tabs = t.tabs[:0]
	t.active = 0
	t.Render()
}

func (t *TabBar) NextTab() {
	if t.active < len(t.tabs)-1 {
		t.active++
	}
	t.Render()
}

func (t *TabBar) PreviousTab() {
	if t.active > 0 {
		t.active--
	}
	t.Render()
}

func (t *TabBar) Render() {
	styles := t.App.GetStyles()
	t.Clear()
	for i, tab := range t.tabs {
		cell := tview.NewTableCell(" " + tab.id + " ")
		if i == t.active {
			cell.SetTextColor(styles.TabBar.ActiveTextColor.Color())
			cell.SetAttributes(tcell.AttrBold)
			cell.SetBackgroundColor(styles.Global.MoreContrastBackgroundColor.Color())
		} else {
			cell.SetTextColor(styles.Global.TextColor.Color())
		}
		t.SetCell(0, i, cell)
	}
	plusCell := tview.NewTableCell(" + ").
		SetTextColor(styles.Global.SecondaryTextColor.Color()).
		SetSelectable(false)
	t.SetCell(0, len(t.tabs), plusCell)
}

func (t *TabBar) GetActiveComponent() TabBarPrimitive {
	return t.tabs[t.active].primitive
}

func (t *TabBar) GetActiveComponentAndRender() TabBarPrimitive {
	component := t.tabs[t.active]
	if !component.rendered {
		component.primitive.Render()
		component.rendered = true
	}
	return component.primitive
}

func (t *TabBar) GetActiveTabIndex() int {
	return t.active
}

func (t *TabBar) GetActiveTabName() string {
	if t.active < len(t.tabs) {
		return t.tabs[t.active].id
	}
	return ""
}

// SwitchToTabByName activates the first tab with the given name.
// Returns true if a matching tab was found.
func (t *TabBar) SwitchToTabByName(name string) bool {
	for i, tab := range t.tabs {
		if tab.id == name {
			t.active = i
			t.Render()
			return true
		}
	}
	return false
}

// GetTabCount returns the total number of registered tabs.
func (t *TabBar) GetTabCount() int {
	return len(t.tabs)
}

// ResetRendered clears the rendered flag on all tabs so each tab's Render()
// is called again on the next GetActiveComponentAndRender invocation.
func (t *TabBar) ResetRendered() {
	for _, tab := range t.tabs {
		tab.rendered = false
	}
}
