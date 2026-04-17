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

	// queryTabs holds only Content (query/table) tabs.
	queryTabs []*component.Data

	// structureTabs and indexTabs cache open structure/index tabs by "schema.table" key.
	structureTabs map[string]*component.Structure
	indexTabs     map[string]*component.Indexes

	// lastSchemas caches the most recent schema list for new-tab autocomplete.
	lastSchemas []database.Schema

	// queryTabNums tracks which "Query N" numbers are currently in use,
	// so closed tabs release their number for reuse.
	queryTabNums map[int]bool

	actionsModal *modal.ActionsModal
	importModal  *modal.ImportModal
}

func NewMain() *Main {
	m := &Main{
		BaseElement:   core.NewBaseElement(),
		Flex:          core.NewFlex(),
		innerFlex:     core.NewFlex(),
		footer:        component.NewFooter(),
		topBar:        component.NewTopBar(),
		schemas:       component.NewSchemaTree(),
		structureTabs: make(map[string]*component.Structure),
		indexTabs:     make(map[string]*component.Indexes),
		queryTabNums:  make(map[int]bool),
		actionsModal:  modal.NewActionsModal(),
		importModal:   modal.NewImportModal(),
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
	if err := m.actionsModal.Init(m.App); err != nil {
		return err
	}
	if err := m.importModal.Init(m.App); err != nil {
		return err
	}

	m.schemas.SetImportFunc(func(schema, table string) {
		m.importModal.Render(schema, table)
	})

	m.schemas.SetOnSchemasLoaded(func(schemas []database.Schema) {
		m.lastSchemas = schemas
		for _, tab := range m.queryTabs {
			tab.SetSchemasForAutocomplete(schemas)
		}
		m.importModal.SetSchemas(schemas)
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
		key := schema + "." + table
		tabName := table + ": Structure"
		tab, exists := m.structureTabs[key]
		if !exists {
			tab = component.NewStructure()
			if err := tab.Init(m.App); err != nil {
				modal.ShowError(m.App.Pages, "Failed to init structure tab", err)
				return
			}
			m.structureTabs[key] = tab
			m.topBar.AddDynamicTab(tabName, tab)
			m.rebuildInnerFlex()
			tab.HandleTableSelection(ctx, schema, table)
		} else {
			m.topBar.SwitchToTabByName(tabName)
			m.rebuildInnerFlex()
		}
		m.App.SetFocus(tab)
	})

	m.schemas.SetIndexesFunc(func(ctx context.Context, schema, table string) {
		key := schema + "." + table
		tabName := table + ": Indexes"
		tab, exists := m.indexTabs[key]
		if !exists {
			tab = component.NewIndexes()
			if err := tab.Init(m.App); err != nil {
				modal.ShowError(m.App.Pages, "Failed to init indexes tab", err)
				return
			}
			m.indexTabs[key] = tab
			m.topBar.AddDynamicTab(tabName, tab)
			m.rebuildInnerFlex()
			tab.HandleTableSelection(ctx, schema, table)
		} else {
			m.topBar.SwitchToTabByName(tabName)
			m.rebuildInnerFlex()
		}
		m.App.SetFocus(tab)
	})

	m.render()
}

// openNewTableTab creates a full CRUD table tab and adds it to the tab bar.
// If the only existing tab is a clean blank query tab, it is silently replaced.
func (m *Main) openNewTableTab(ctx context.Context, schema, table string) error {
	// Replace the initial blank query tab if it hasn't been used yet.
	if len(m.queryTabs) == 1 && m.topBar.GetTabCount() == 1 {
		if m.queryTabs[0].IsCleanQueryTab() {
			name := m.topBar.GetActiveTabName()
			var n int
			if _, err := fmt.Sscanf(name, "Query %d", &n); err == nil {
				delete(m.queryTabNums, n)
			}
			m.queryTabs = m.queryTabs[:0]
			m.topBar.ClearAllTabs()
		}
	}

	tab := component.NewTableTab()
	if err := tab.Init(m.App); err != nil {
		return err
	}
	tab.SetSchemasForAutocomplete(m.lastSchemas)
	tab.SetOnOpenQueryWithSQL(m.openNewQueryTabWithQuery)
	m.queryTabs = append(m.queryTabs, tab)
	m.topBar.AddDynamicTab(table, tab)
	m.rebuildInnerFlex()
	// Defer data loading so the first draw properly get height from c.table.GetInnerRect()
	go m.App.Application.QueueUpdateDraw(func() {
		if err := tab.HandleTableSelection(ctx, schema, table); err != nil {
			modal.ShowError(m.App.Pages, "Failed to load table data", err)
		}
	})
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

// openNewQueryTabWithQuery creates a new query tab, pre-fills it with query, and executes it.
func (m *Main) openNewQueryTabWithQuery(query string) {
	m.openNewQueryTab()
	if len(m.queryTabs) > 0 {
		m.queryTabs[len(m.queryTabs)-1].SetEditorTextAndExecute(query)
	}
}

// openNewQueryTab creates a blank read-only query tab.
func (m *Main) openNewQueryTab() {
	n := m.nextQueryTabNum()
	m.queryTabNums[n] = true
	tab := component.NewData()
	if err := tab.Init(m.App); err != nil {
		modal.ShowError(m.App.Pages, "Failed to create tab", err)
		m.queryTabNums[n] = false
		return
	}
	tab.SetSchemasForAutocomplete(m.lastSchemas)
	tab.Render()
	m.queryTabs = append(m.queryTabs, tab)
	m.topBar.AddDynamicTab(fmt.Sprintf("Query %d", n), tab)
	m.rebuildInnerFlex()
}

// closeActiveTab removes the active tab. The last query tab cannot be closed,
// but the last table/structure/index tab can — a fresh blank query tab replaces it.
func (m *Main) closeActiveTab() {
	isLastTab := m.topBar.GetTabCount() <= 1
	name := m.topBar.GetActiveTabName()
	active := m.topBar.GetActiveComponent()

	switch tab := active.(type) {
	case *component.Data:
		if isLastTab && tab.IsQueryTab() {
			return
		}
		for i, t := range m.queryTabs {
			if t == tab {
				m.queryTabs = slices.Delete(m.queryTabs, i, i+1)
				break
			}
		}
		var n int
		if _, err := fmt.Sscanf(name, "Query %d", &n); err == nil {
			delete(m.queryTabNums, n)
		}
	case *component.Structure:
		for key, s := range m.structureTabs {
			if s == tab {
				delete(m.structureTabs, key)
				break
			}
		}
	case *component.Indexes:
		for key, idx := range m.indexTabs {
			if idx == tab {
				delete(m.indexTabs, key)
				break
			}
		}
	}

	if isLastTab {
		m.topBar.ClearAllTabs()
		m.openNewQueryTab()
		return
	}
	m.topBar.CloseActiveTab()
	m.rebuildInnerFlex()
	m.setFocusToActiveTab()
}

func (m *Main) UpdateDriver(driver database.Driver) {
	m.BaseElement.UpdateDriver(driver)
	m.schemas.UpdateDriver(driver)
	m.footer.UpdateDriver(driver)
	m.topBar.UpdateDriver(driver)

	for _, tab := range m.queryTabs {
		tab.UpdateDriver(driver)
		tab.Reset()
	}
	// Remove all existing tabs; the user starts fresh with the new connection.
	m.queryTabs = m.queryTabs[:0]
	m.queryTabNums = make(map[int]bool)
	m.structureTabs = make(map[string]*component.Structure)
	m.indexTabs = make(map[string]*component.Indexes)
	m.topBar.ClearAllTabs()

	m.topBar.ResetRendered()
	m.topBar.Render()
	m.rebuildInnerFlex()
}

func (m *Main) JumpToTable(schema, table string) error {
	if m.Driver == nil {
		return fmt.Errorf("not connected to a database")
	}

	ctx := context.Background()
	return m.schemas.JumpToTable(ctx, schema, table)
}

// showSchemas re-inserts the schema panel into the layout without recreating tabs.
func (m *Main) showSchemas() {
	schemaPanelWidth := m.App.GetConfig().UI.SchemaPanelWidth
	if schemaPanelWidth == 0 {
		schemaPanelWidth = 30
	}
	m.Clear()
	m.AddItem(m.schemas, schemaPanelWidth, 0, true)
	m.AddItem(m.innerFlex, 0, 7, false)
	m.App.SetFocus(m.schemas)
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
	if m.topBar.HasTabs() {
		m.innerFlex.AddItem(m.topBar.GetActiveComponentAndRender(), 0, 7, true)
	}
	m.innerFlex.AddItem(m.footer, m.footerHeight, 0, false)
}

func (m *Main) ToggleFooter() {
	m.footerHeight = m.footer.Toggle()
	m.rebuildInnerFlex()
	m.footer.Render()
	m.App.GetManager().Broadcast(manager.EventMsg{
		Sender:  m.GetIdentifier(),
		Message: manager.Message{Type: manager.FooterHeightChanged, Data: m.footerHeight},
	})
}

func (m *Main) setKeybindings() {
	k := m.App.GetKeys()
	m.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if _, isInputBar := m.App.GetFocus().(*component.InputBar); isInputBar {
			return event
		}

		switch {
		case k.Contains(k.Navigation.FocusRight, event.Name()):
			if !m.topBar.HasTabs() {
				return nil
			}
			if active, ok := m.topBar.GetActiveComponent().(*component.Indexes); ok && active.IsAddFormFocused() {
				return event
			}
			if m.schemas.IsFocused() {
				m.setFocusToActiveTab()
			} else if m.topBar.GetActiveTabIndex() < m.topBar.GetTabCount()-1 {
				m.topBar.NextTab()
				m.rebuildInnerFlex()
				m.setFocusToActiveTab()
			}
			// On last tab: do nothing; focus and footer keys stay intact
			return nil
		case k.Contains(k.Navigation.FocusLeft, event.Name()):
			if m.topBar.HasTabs() {
				if active, ok := m.topBar.GetActiveComponent().(*component.Indexes); ok && active.IsAddFormFocused() {
					return event
				}
			}
			if m.topBar.GetActiveTabIndex() == 0 {
				m.App.SetFocus(m.schemas)
			} else {
				m.topBar.PreviousTab()
				m.rebuildInnerFlex()
				m.setFocusToActiveTab()
			}
			return nil
		case k.Contains(k.Main.FocusSchemaTree, event.Name()):
			if _, ok := m.GetItem(0).(*component.SchemaTree); !ok {
				m.showSchemas()
			} else {
				m.App.SetFocus(m.schemas)
			}
			return nil
		case k.Contains(k.Main.HideSchema, event.Name()):
			if _, ok := m.GetItem(0).(*component.SchemaTree); ok {
				m.RemoveItem(m.schemas)
				if m.topBar.HasTabs() {
					m.setFocusToActiveTab()
				}
			} else {
				m.showSchemas()
			}
			return nil
		case k.Contains(k.Main.ServerInfo, event.Name()):
			m.showServerInfo()
			return nil
		case k.Contains(k.Main.OpenActions, event.Name()):
			m.openActionsModal()
			return nil
		case k.Contains(k.Main.NewTab, event.Name()):
			m.openNewQueryTab()
			return nil
		case k.Contains(k.Main.CloseTab, event.Name()):
			m.closeActiveTab()
			return nil
		case k.Contains(k.Main.ImportData, event.Name()):
			m.importModal.Render(m.schemas.SelectedTable())
			return nil
		}
		return event
	})
}

// setFocusToActiveTab sets focus on the right inner primitive of the currently active tab.
// For Content tabs it re-focuses the SQL editor if it was open, otherwise the table.
func (m *Main) setFocusToActiveTab() {
	active := m.topBar.GetActiveComponent()
	if content, ok := active.(*component.Data); ok {
		m.App.SetFocus(content.GetFocusPrimitive())
	} else {
		m.App.SetFocus(active)
	}
}

func (m *Main) openActionsModal() {
	k := m.App.GetKeys()
	ctx := context.Background()

	entries := []modal.ActionEntry{
		{
			Label:   "Server info",
			KeyHint: k.Main.ServerInfo.String(),
			Handler: m.showServerInfo,
		},
		{
			Label:   "Change style",
			KeyHint: k.Global.ChangeStyle.String(),
			Handler: m.App.OpenStyleModal,
		},
		{
			Label:   "Connection page",
			KeyHint: k.Global.OpenConnection.String(),
			Handler: m.App.OpenConnectionPage,
		},
		{
			Label:   "New tab",
			KeyHint: k.Main.NewTab.String(),
			Handler: m.openNewQueryTab,
		},
		{
			Label:   "Close tab",
			KeyHint: k.Main.CloseTab.String(),
			Handler: m.closeActiveTab,
		},
		{
			Label:   "Import CSV",
			KeyHint: k.Main.ImportData.String(),
			Handler: func() { m.importModal.Render(m.schemas.SelectedTable()) },
		},
		{
			Label:   "Create table",
			KeyHint: k.Common.Add.String(),
			Handler: func() { m.schemas.OpenCreateTable(ctx) },
		},
	}

	if data, ok := m.topBar.GetActiveComponent().(*component.Data); ok {
		hasResults := data.HasResults()

		var historyHandler func()
		if data.IsQueryTab() {
			historyHandler = data.OpenHistory
		} else {
			historyHandler = func() { data.OpenHistoryWithCallback(m.openNewQueryTabWithQuery) }
		}

		entries = append(entries,
			modal.ActionEntry{
				Label:    "Export data",
				KeyHint:  k.Data.ExportData.String(),
				Handler:  func() { data.OpenExport(ctx) },
				Disabled: !hasResults,
			},
			modal.ActionEntry{
				Label:    "Explain viewer",
				KeyHint:  k.Data.ExplainQuery.String(),
				Handler:  func() { data.OpenExplain(ctx) },
				Disabled: !hasResults,
			},
			modal.ActionEntry{
				Label:   "History",
				KeyHint: k.SQLQueryEditor.OpenHistory.String(),
				Handler: historyHandler,
			},
		)
	}

	m.actionsModal.Open(entries)
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
