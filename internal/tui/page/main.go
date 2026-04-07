package page

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/component"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
)

const (
	MainPageId = "Main"
)

type Main struct {
	*core.BaseElement
	*core.Flex

	innerFlex    *core.Flex
	footer       *component.Footer
	topBar       *component.TopBar
	schemas      *component.SchemaTree
	footerHeight int

	// structurePanel and indexPanel are shown in-place of the active tab when
	// the user selects the Columns or Indexes child node in the schema tree.
	structurePanel *component.Structure
	indexPanel     *component.Indexes

	// queryTabs mirrors the tab bar: queryTabs[i] == tab bar tab i.
	queryTabs []*component.Content

	// lastSchemas caches the most recent schema list for new-tab autocomplete.
	lastSchemas []database.SchemaWithTables

	// activePanel is non-nil when the structure or index panel is currently
	// swapped in as the main content area.
	activePanel tview.Primitive

	// queryTabNums tracks which "Query N" numbers are currently in use,
	// so closed tabs release their number for reuse.
	queryTabNums map[int]bool
}

func NewMain() *Main {
	m := &Main{
		BaseElement:    core.NewBaseElement(),
		Flex:           core.NewFlex(),
		innerFlex:      core.NewFlex(),
		footer:         component.NewFooter(),
		topBar:         component.NewTopBar(),
		schemas:        component.NewSchemaTree(),
		structurePanel: component.NewStructure(),
		indexPanel:     component.NewIndexes(),
		queryTabNums:   make(map[int]bool),
	}

	m.SetIdentifier(MainPageId)
	m.SetAfterInitFunc(m.init)

	return m
}

func (m *Main) init() error {
	m.setStyles()
	m.setKeybindings()
	m.handleEvents()
	return m.initComponents()
}

func (m *Main) setStyles() {
	m.SetStyle(m.App.GetStyles())
	m.innerFlex.SetStyle(m.App.GetStyles())
	m.innerFlex.SetDirection(tview.FlexRow)
}

func (m *Main) handleEvents() {
	go m.HandleEvents(MainPageId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			m.setStyles()
		}
	})
}

func (m *Main) initComponents() error {
	if err := m.footer.Init(m.App); err != nil {
		return err
	}
	if err := m.topBar.Init(m.App); err != nil {
		return err
	}
	if err := m.schemas.Init(m.App); err != nil {
		return err
	}
	if err := m.structurePanel.Init(m.App); err != nil {
		return err
	}
	if err := m.indexPanel.Init(m.App); err != nil {
		return err
	}

	m.schemas.SetOnSchemasLoaded(func(schemas []database.SchemaWithTables) {
		m.lastSchemas = schemas
		for _, tab := range m.queryTabs {
			tab.SetEditorSchemas(schemas)
		}
	})

	return nil
}

func (m *Main) Render() {
	m.schemas.Render()
	m.footer.Render()
	m.topBar.Render()

	m.footer.SetOnHeightChange(func() {
		newHeight := m.footer.ExpandedHeight()
		if newHeight == m.footerHeight {
			return
		}
		m.footerHeight = newHeight
		m.innerFlex.ResizeItem(m.footer, newHeight, 0)
	})

	m.schemas.SetSelectFunc(func(ctx context.Context, schema, table string) error {
		return m.openNewTableTab(ctx, schema, table)
	})

	m.schemas.SetColumnsFunc(func(ctx context.Context, schema, table string) {
		m.structurePanel.Render()
		m.structurePanel.HandleTableSelection(ctx, schema, table)
		m.activePanel = m.structurePanel
		m.rebuildInnerFlex()
		m.App.SetFocus(m.structurePanel)
	})

	m.schemas.SetIndexesFunc(func(ctx context.Context, schema, table string) {
		m.indexPanel.Render()
		m.indexPanel.HandleTableSelection(ctx, schema, table)
		m.activePanel = m.indexPanel
		m.rebuildInnerFlex()
		m.App.SetFocus(m.indexPanel)
	})

	m.render()
}

// openNewTableTab creates a full CRUD table tab and adds it to the tab bar.
func (m *Main) openNewTableTab(ctx context.Context, schema, table string) error {
	tab := component.NewTableTab()
	if err := tab.Init(m.App); err != nil {
		return err
	}
	tab.SetEditorSchemas(m.lastSchemas)
	m.queryTabs = append(m.queryTabs, tab)
	m.topBar.AddDynamicTab(table, tab)
	m.activePanel = nil
	m.rebuildInnerFlex()
	if err := tab.HandleTableSelection(ctx, schema, table); err != nil {
		return err
	}
	return nil
}

// nextQueryTabNum returns the lowest positive integer not currently in use.
func (m *Main) nextQueryTabNum() int {
	for n := 1; ; n++ {
		if !m.queryTabNums[n] {
			return n
		}
	}
}

// openNewQueryTab creates a blank read-only query tab.
func (m *Main) openNewQueryTab() {
	n := m.nextQueryTabNum()
	m.queryTabNums[n] = true
	tab := component.NewContent()
	if err := tab.Init(m.App); err != nil {
		modal.ShowError(m.App.Pages, "Failed to create tab", err)
		m.queryTabNums[n] = false
		return
	}
	tab.SetEditorSchemas(m.lastSchemas)
	tab.Render()
	m.queryTabs = append(m.queryTabs, tab)
	m.topBar.AddDynamicTab(fmt.Sprintf("Query %d", n), tab)
	m.activePanel = nil
	m.rebuildInnerFlex()
	m.App.SetFocus(tab)
}

// closeActiveTab removes the active tab. The last tab cannot be closed.
func (m *Main) closeActiveTab() {
	if len(m.queryTabs) <= 1 {
		return
	}
	idx := m.topBar.GetActiveTabIndex()
	if idx < 0 || idx >= len(m.queryTabs) {
		return
	}
	// Release the query number if this was a "Query N" tab.
	name := m.topBar.GetActiveTabName()
	var n int
	if _, err := fmt.Sscanf(name, "Query %d", &n); err == nil {
		delete(m.queryTabNums, n)
	}
	m.queryTabs = slices.Delete(m.queryTabs, idx, idx+1)
	m.topBar.CloseActiveTab()
	m.activePanel = nil
	m.rebuildInnerFlex()
	if m.topBar.HasTabs() {
		m.App.SetFocus(m.topBar.GetActiveComponent())
	}
}

// hidePanel dismisses the structure or index panel and restores the active tab.
func (m *Main) hidePanel() {
	m.activePanel = nil
	m.rebuildInnerFlex()
	if m.topBar.HasTabs() {
		m.App.SetFocus(m.topBar.GetActiveComponent())
	} else {
		m.App.SetFocus(m.schemas)
	}
}

func (m *Main) UpdateDriver(driver database.Driver) {
	m.BaseElement.UpdateDriver(driver)
	m.schemas.UpdateDriver(driver)
	m.footer.UpdateDriver(driver)
	m.structurePanel.UpdateDriver(driver)
	m.indexPanel.UpdateDriver(driver)

	for _, tab := range m.queryTabs {
		tab.UpdateDriver(driver)
		tab.Reset()
	}
	// Remove all existing tabs; the user starts fresh with the new connection.
	m.queryTabs = m.queryTabs[:0]
	m.queryTabNums = make(map[int]bool)
	m.topBar.ClearAllTabs()
	m.activePanel = nil

	m.topBar.ResetRendered()
	m.App.QueueUpdateDraw(func() {
		m.topBar.Render()
		m.rebuildInnerFlex()
	})
}

func (m *Main) JumpToTable(schema, table string) error {
	if m.Driver == nil {
		return fmt.Errorf("not connected to a database")
	}

	ctx := context.Background()
	return m.schemas.JumpToTable(ctx, schema, table)
}

func (m *Main) render() {
	m.Clear()

	schemaPanelWidth := m.App.GetConfig().UI.SchemaPanelWidth
	if schemaPanelWidth == 0 {
		schemaPanelWidth = 30
	}

	m.AddItem(m.schemas, schemaPanelWidth, 0, true)
	m.AddItem(m.innerFlex, 0, 7, false)
	if m.footerHeight == 0 {
		m.footerHeight = 2
	}

	// Start with a single blank query tab.
	m.openNewQueryTab()

	m.App.Pages.AddPage(m.GetIdentifier(), m, true, true)
	m.App.SetFocus(m.schemas)
}

func (m *Main) rebuildInnerFlex() {
	m.innerFlex.Clear()
	m.innerFlex.AddItem(m.topBar, 3, 0, false)
	if m.activePanel != nil {
		m.innerFlex.AddItem(m.activePanel, 0, 7, true)
	} else if m.topBar.HasTabs() {
		m.innerFlex.AddItem(m.topBar.GetActiveComponentAndRender(), 0, 7, true)
	}
	m.innerFlex.AddItem(m.footer, m.footerHeight, 0, false)
}

func (m *Main) ToggleHeader() {
	m.footerHeight = m.footer.Toggle()
	m.rebuildInnerFlex()
	m.footer.Render()
	m.App.GetManager().Broadcast(manager.EventMsg{
		Sender:  m.GetIdentifier(),
		Message: manager.Message{Type: manager.HeaderHeightChanged, Data: m.footerHeight},
	})
}

func (m *Main) setKeybindings() {
	k := m.App.GetKeys()
	m.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if _, isInputBar := m.App.GetFocus().(*component.InputBar); isInputBar {
			return event
		}

		// Dismiss structure/index panel with Escape.
		if m.activePanel != nil && event.Key() == tcell.KeyEscape {
			m.hidePanel()
			return nil
		}

		switch {
		case k.Contains(k.Navigation.FocusRight, event.Name()):
			if m.indexPanel.IsAddFormFocused() {
				return event
			}
			if m.schemas.IsFocused() {
				if m.topBar.HasTabs() {
					m.App.SetFocus(m.topBar.GetActiveComponent())
				}
			} else if m.activePanel == nil {
				m.topBar.NextTab()
				m.rebuildInnerFlex()
				m.App.SetFocus(m.topBar.GetActiveComponent())
			}
			return nil
		case k.Contains(k.Navigation.FocusLeft, event.Name()):
			if m.indexPanel.IsAddFormFocused() {
				return event
			}
			if m.activePanel != nil {
				m.hidePanel()
				m.App.SetFocus(m.schemas)
				return nil
			}
			if m.topBar.GetActiveTabIndex() == 0 {
				m.App.SetFocus(m.schemas)
			} else {
				m.topBar.PreviousTab()
				m.rebuildInnerFlex()
				m.App.SetFocus(m.topBar.GetActiveComponent())
			}
			return nil
		case k.Contains(k.Global.HideSchema, event.Name()):
			if _, ok := m.GetItem(0).(*component.SchemaTree); ok {
				m.RemoveItem(m.schemas)
				if m.topBar.HasTabs() {
					m.App.SetFocus(m.topBar.GetActiveComponent())
				}
			} else {
				m.Clear()
				m.render()
			}
			return nil
		case k.Contains(k.Global.ServerInfo, event.Name()):
			m.showServerInfo()
			return nil
		case k.Contains(k.Global.NewTab, event.Name()):
			m.openNewQueryTab()
			return nil
		case k.Contains(k.Global.CloseTab, event.Name()):
			m.closeActiveTab()
			return nil
		}
		return event
	})
}

func (m *Main) showServerInfo() {
	if m.Driver == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := m.Driver.GetServerInfo(ctx)
	if err != nil {
		modal.ShowError(m.App.Pages, "Failed to get server info", err)
		return
	}

	infoText := fmt.Sprintf(
		"Version: %s\nUptime: %s\nActive Sessions: %d\nDatabase: %s\nHost: %s:%d",
		info.Version, info.Uptime, info.ActiveSessions, info.CurrentDB, info.Host, info.Port,
	)

	infoModal := core.NewModal()
	infoModal.SetStyle(m.App.GetStyles())
	infoModal.SetText(infoText)
	infoModal.AddButtons([]string{"Close"})
	infoModal.SetDoneFunc(func(_ int, _ string) {
		m.App.Pages.RemovePage("ServerInfo")
	})

	m.App.Pages.AddPage("ServerInfo", infoModal, true, true)
}
