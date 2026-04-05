package page

import (
	"context"
	"fmt"
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
	content      *component.Content
	structure    *component.Structure
	indexes      *component.Indexes
	footerHeight int
}

func NewMain() *Main {
	m := &Main{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		innerFlex:   core.NewFlex(),
		footer:      component.NewFooter(),
		topBar:      component.NewTopBar(),
		schemas:     component.NewSchemaTree(),
		content:     component.NewContent(),
		structure:   component.NewStructure(),
		indexes:     component.NewIndexes(),
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
	if err := m.content.Init(m.App); err != nil {
		return err
	}
	if err := m.structure.Init(m.App); err != nil {
		return err
	}
	if err := m.indexes.Init(m.App); err != nil {
		return err
	}

	m.schemas.SetOnSchemasLoaded(m.content.SetEditorSchemas)

	m.topBar.AddTab("Content", m.content, true)
	m.topBar.AddTab("Indexes", m.indexes, false)
	m.topBar.AddTab("Structure", m.structure, false)

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
		if err := m.content.HandleTableSelection(ctx, schema, table); err != nil {
			return err
		}
		m.structure.HandleTableSelection(ctx, schema, table)
		m.indexes.HandleTableSelection(ctx, schema, table)
		m.App.SetFocus(m.topBar.GetActiveComponent())
		return nil
	})

	m.render()
}

func (m *Main) UpdateDriver(driver database.Driver) {
	m.BaseElement.UpdateDriver(driver)
	m.schemas.UpdateDriver(driver)
	m.footer.UpdateDriver(driver)
	m.content.UpdateDriver(driver)
	m.structure.UpdateDriver(driver)
	m.indexes.UpdateDriver(driver)

	m.content.Reset()
	m.topBar.ResetRendered()

	m.App.QueueUpdateDraw(func() { m.topBar.Render() })
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
		m.footerHeight = 1
	}
	m.rebuildInnerFlex()

	m.App.Pages.AddPage(m.GetIdentifier(), m, true, true)
	m.App.SetFocus(m.schemas)
}

func (m *Main) rebuildInnerFlex() {
	m.innerFlex.Clear()
	m.innerFlex.AddItem(m.topBar, 1, 0, false)
	m.innerFlex.AddItem(m.topBar.GetActiveComponentAndRender(), 0, 7, true)
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
		switch {
		case k.Contains(k.Navigation.FocusRight, event.Name()):
			if m.indexes.IsAddFormFocused() {
				return event
			}
			if m.schemas.IsFocused() {
				m.App.SetFocus(m.topBar.GetActiveComponent())
			} else {
				m.topBar.NextTab()
				m.rebuildInnerFlex()
				m.App.SetFocus(m.topBar.GetActiveComponent())
			}
			return nil
		case k.Contains(k.Navigation.FocusLeft, event.Name()):
			if m.indexes.IsAddFormFocused() {
				return event
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
				m.App.SetFocus(m.topBar.GetActiveComponent())
			} else {
				m.Clear()
				m.render()
			}
			return nil
		case k.Contains(k.Global.ServerInfo, event.Name()):
			m.showServerInfo()
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
