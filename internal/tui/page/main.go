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
	header       *component.Header
	tabBar       *component.TabBar
	schemas      *component.SchemaTree
	content      *component.Content
	headerHeight int
}

func NewMain() *Main {
	m := &Main{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		innerFlex:   core.NewFlex(),
		header:      component.NewHeader(),
		tabBar:      component.NewTabBar(),
		schemas:     component.NewSchemaTree(),
		content:     component.NewContent(),
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
	if err := m.header.Init(m.App); err != nil {
		return err
	}
	if err := m.tabBar.Init(m.App); err != nil {
		return err
	}
	if err := m.schemas.Init(m.App); err != nil {
		return err
	}
	if err := m.content.Init(m.App); err != nil {
		return err
	}

	m.tabBar.AddTab("Content", m.content, true)

	return nil
}

func (m *Main) Render() {
	m.schemas.Render()
	m.header.Render()
	m.tabBar.Render()

	m.schemas.SetSelectFunc(func(ctx context.Context, schema, table string) error {
		err := m.content.HandleTableSelection(ctx, schema, table)
		if err != nil {
			return err
		}
		m.App.SetFocus(m.tabBar.GetActiveComponent())
		return nil
	})

	m.render()
}

func (m *Main) UpdateDriver(driver database.Driver) {
	m.BaseElement.UpdateDriver(driver)
	m.schemas.UpdateDriver(driver)
	m.header.UpdateDriver(driver)
	m.content.UpdateDriver(driver)
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
	m.innerFlex.Clear()

	schemaPanelWidth := m.App.GetConfig().UI.SchemaPanelWidth
	if schemaPanelWidth == 0 {
		schemaPanelWidth = 30
	}

	m.AddItem(m.schemas, schemaPanelWidth, 0, true)
	m.AddItem(m.innerFlex, 0, 7, false)
	if m.headerHeight == 0 {
		m.headerHeight = 4
	}
	m.innerFlex.AddItem(m.header, m.headerHeight, 0, false)
	m.innerFlex.AddItem(m.tabBar, 1, 0, false)
	m.innerFlex.AddItem(m.tabBar.GetActiveComponentAndRender(), 0, 7, true)

	m.App.Pages.AddPage(m.GetIdentifier(), m, true, true)
	m.App.SetFocus(m)
}

func (m *Main) ToggleHeader() {
	m.headerHeight = m.header.Toggle()
	m.innerFlex.Clear()
	m.innerFlex.AddItem(m.header, m.headerHeight, 0, false)
	m.innerFlex.AddItem(m.tabBar, 1, 0, false)
	m.innerFlex.AddItem(m.tabBar.GetActiveComponentAndRender(), 0, 7, true)
	m.header.Render()
	m.App.GetManager().Broadcast(manager.EventMsg{
		Sender:  m.GetIdentifier(),
		Message: manager.Message{Type: manager.HeaderHeightChanged, Data: m.headerHeight},
	})
}

func (m *Main) setKeybindings() {
	k := m.App.GetKeys()
	m.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Contains(k.Main.FocusNext, event.Name()):
			if m.schemas.IsFocused() {
				m.App.SetFocus(m.tabBar.GetActiveComponent())
			} else {
				m.innerFlex.RemoveItem(m.tabBar.GetActiveComponent())
				m.tabBar.NextTab()
				m.innerFlex.AddItem(m.tabBar.GetActiveComponentAndRender(), 0, 7, true)
				m.App.SetFocus(m.tabBar.GetActiveComponent())
			}
			return nil
		case k.Contains(k.Main.FocusPrevious, event.Name()):
			if m.tabBar.GetActiveTabIndex() == 0 {
				m.App.SetFocus(m.schemas)
			} else {
				m.innerFlex.RemoveItem(m.tabBar.GetActiveComponent())
				m.tabBar.PreviousTab()
				m.innerFlex.AddItem(m.tabBar.GetActiveComponentAndRender(), 0, 7, true)
				m.App.SetFocus(m.tabBar.GetActiveComponent())
			}
			return nil
		case k.Contains(k.Main.HideSchema, event.Name()):
			if _, ok := m.GetItem(0).(*component.SchemaTree); ok {
				m.RemoveItem(m.schemas)
				m.App.SetFocus(m.tabBar.GetActiveComponent())
			} else {
				m.Clear()
				m.render()
			}
			return nil
		case k.Contains(k.Main.ShowServerInfo, event.Name()):
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
