package widget

import (
	"slices"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

type TabBarPrimitive interface {
	tview.Primitive
	Render()
}

// TabKind distinguishes the visual category of a tab.
type TabKind int

const (
	KindQuery TabKind = iota
	KindTable
	KindStructure
	KindIndex
)

type tabBarComponent struct {
	id        string
	tabID     string
	kind      TabKind
	primitive TabBarPrimitive
	rendered  bool
}

type TabBar struct {
	*core.Table

	styles *config.Styles
	active int
	offset int
	tabs   []*tabBarComponent
}

func NewTabBar() *TabBar {
	t := &TabBar{
		Table: core.NewTable(),
		tabs:  []*tabBarComponent{},
	}
	t.SetBorderPadding(0, 0, 1, 0)
	return t
}

func (t *TabBar) SetStyle(styles *config.Styles) {
	t.styles = styles
	t.Table.SetStyle(styles)
	t.Render()
}

func (t *TabBar) AddTab(name string, component TabBarPrimitive, kind TabKind, defaultTab bool) {
	t.tabs = append(t.tabs, &tabBarComponent{
		id:        name,
		kind:      kind,
		primitive: component,
	})
	if defaultTab {
		t.active = len(t.tabs) - 1
	}
	t.Render()
}

// AddDynamicTab adds a tab at runtime and activates it immediately.
// Returns the index of the new tab.
func (t *TabBar) AddDynamicTab(name string, component TabBarPrimitive, kind TabKind) int {
	t.tabs = append(t.tabs, &tabBarComponent{
		id:        name,
		kind:      kind,
		primitive: component,
	})
	t.active = len(t.tabs) - 1
	t.Render()
	return t.active
}

// AddDynamicTabWithID is like AddDynamicTab but stamps the component with a stable tabID.
func (t *TabBar) AddDynamicTabWithID(name, tabID string, kind TabKind, component TabBarPrimitive) int {
	t.tabs = append(t.tabs, &tabBarComponent{
		id:        name,
		tabID:     tabID,
		kind:      kind,
		primitive: component,
	})
	t.active = len(t.tabs) - 1
	t.Render()
	return t.active
}

// GetActiveTabID returns the MCP-assigned tabID of the active tab, or empty string.
func (t *TabBar) GetActiveTabID() string {
	if t.active < len(t.tabs) {
		return t.tabs[t.active].tabID
	}
	return ""
}

func (t *TabBar) RenameActiveTab(newName string) {
	if t.active < len(t.tabs) {
		t.tabs[t.active].id = newName
		t.Render()
	}
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

func cellWidthPlusSeparator(content string) int {
	return len(content) + 1
}

func (t *TabBar) tabIcon(kind TabKind) string {
	if t.styles == nil {
		return ""
	}
	sym := &t.styles.Icons
	switch kind {
	case KindTable:
		return sym.Icon(sym.TabTable)
	case KindStructure:
		return sym.Icon(sym.TabStructure)
	case KindIndex:
		return sym.Icon(sym.TabIndex)
	default:
		return sym.Icon(sym.TabQuery)
	}
}

func (t *TabBar) tabLabel(tab *tabBarComponent) string {
	return " " + t.tabIcon(tab.kind) + tab.id
}

// calcVisibleEnd returns the exclusive end index of tabs visible from t.offset
// within availWidth columns, accounting for tview's per-column separator.
func (t *TabBar) calcVisibleEnd(availWidth int) int {
	reserved := 0
	if t.offset > 0 {
		reserved += cellWidthPlusSeparator("< ")
	}
	reserved += cellWidthPlusSeparator(" + ")

	visibleEnd := t.offset
	used := reserved
	for visibleEnd < len(t.tabs) {
		tw := cellWidthPlusSeparator(t.tabLabel(t.tabs[visibleEnd]))
		if used+tw > availWidth {
			break
		}
		used += tw
		visibleEnd++
	}

	if visibleEnd == len(t.tabs) {
		return visibleEnd
	}

	reserved += cellWidthPlusSeparator(" >")
	visibleEnd = t.offset
	used = reserved
	for visibleEnd < len(t.tabs) {
		tw := cellWidthPlusSeparator(t.tabLabel(t.tabs[visibleEnd]))
		if used+tw > availWidth {
			break
		}
		used += tw
		visibleEnd++
	}
	return visibleEnd
}

// scrollToActive adjusts t.offset so the active tab is always within the
// visible window given availWidth terminal columns.
func (t *TabBar) scrollToActive(availWidth int) {
	if t.active < t.offset {
		t.offset = t.active
		return
	}
	for t.offset <= t.active {
		if t.active < t.calcVisibleEnd(availWidth) {
			break
		}
		t.offset++
	}
	if t.offset > t.active {
		t.offset = t.active
	}
}

func (t *TabBar) Render() {
	if t.styles == nil {
		return
	}
	styles := t.styles
	t.Clear()

	bg := styles.Global.BackgroundColor.Color()
	navStyle := tcell.StyleDefault.Foreground(styles.Global.SecondaryTextColor.Color()).Background(bg)

	_, _, width, _ := t.GetInnerRect()
	if width <= 0 {
		for i, tab := range t.tabs {
			cell := tview.NewTableCell(t.tabLabel(tab))
			if i == t.active {
				cell.SetStyle(tcell.StyleDefault.
					Foreground(styles.TabBar.ActiveTextColor.Color()).
					Background(styles.Global.MoreContrastBackgroundColor.Color()).
					Attributes(tcell.AttrBold))
			} else {
				cell.SetStyle(tcell.StyleDefault.
					Foreground(styles.Global.TextColor.Color()).
					Background(bg))
			}
			t.SetCell(0, i, cell)
		}
		t.SetCell(0, len(t.tabs), tview.NewTableCell(" + ").SetStyle(navStyle).SetSelectable(false))
		return
	}

	t.scrollToActive(width)

	col := 0

	if t.offset > 0 {
		t.SetCell(0, col, tview.NewTableCell("< ").SetStyle(navStyle).SetSelectable(false))
		col++
	}

	visibleEnd := t.calcVisibleEnd(width)

	for i := t.offset; i < visibleEnd; i++ {
		tab := t.tabs[i]
		cell := tview.NewTableCell(t.tabLabel(tab))
		if i == t.active {
			cell.SetStyle(tcell.StyleDefault.
				Foreground(styles.TabBar.ActiveTextColor.Color()).
				Background(styles.Global.MoreContrastBackgroundColor.Color()).
				Attributes(tcell.AttrBold))
		} else {
			cell.SetStyle(tcell.StyleDefault.
				Foreground(styles.Global.TextColor.Color()).
				Background(bg))
		}
		t.SetCell(0, col, cell)
		col++
	}

	if visibleEnd < len(t.tabs) {
		t.SetCell(0, col, tview.NewTableCell(" >").SetStyle(navStyle).SetSelectable(false))
		col++
	}

	t.SetCell(0, col, tview.NewTableCell(" + ").SetStyle(navStyle).SetSelectable(false))
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
