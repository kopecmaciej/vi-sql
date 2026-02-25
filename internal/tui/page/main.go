package page

import (
	"context"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
)

const (
	MainPageId = "Main"
)

type Main struct {
	*core.BaseElement
	*core.Flex

	innerFlex *core.Flex

	// TODO: wire in components once component package is built
	// header    *component.Header
	// tabBar    *component.TabBar
	// schemas   *component.SchemaTree
	// content   *component.Content

	// placeholder widgets until components exist
	schemaTree *core.TreeView
	contentTv  *core.TextView
	headerTv   *core.TextView
	statusTv   *core.TextView
}

func NewMain() *Main {
	m := &Main{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		innerFlex:   core.NewFlex(),
		schemaTree:  core.NewTreeView(),
		contentTv:   core.NewTextView(),
		headerTv:    core.NewTextView(),
		statusTv:    core.NewTextView(),
	}

	m.SetIdentifier(MainPageId)
	m.SetAfterInitFunc(m.init)

	return m
}

func (m *Main) init() error {
	m.setStyles()
	m.setKeybindings()

	m.handleEvents()

	return nil
}

func (m *Main) setStyles() {
	m.SetStyle(m.App.GetStyles())
	m.innerFlex.SetStyle(m.App.GetStyles())
	m.innerFlex.SetDirection(tview.FlexRow)
	m.schemaTree.SetStyle(m.App.GetStyles())
	m.contentTv.SetStyle(m.App.GetStyles())
	m.headerTv.SetStyle(m.App.GetStyles())
	m.statusTv.SetStyle(m.App.GetStyles())
}

func (m *Main) handleEvents() {
	go m.HandleEvents(MainPageId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			m.setStyles()
		}
	})
}

func (m *Main) Render() {
	m.Clear()
	m.innerFlex.Clear()

	// Header
	m.headerTv.SetBorder(false)
	m.headerTv.SetDynamicColors(true)
	m.headerTv.SetTextAlign(tview.AlignLeft)
	m.renderHeader()

	// Schema tree (left panel)
	m.schemaTree.SetBorder(true)
	m.schemaTree.SetTitle(" Schemas ")

	root := tview.NewTreeNode("schemas")
	m.schemaTree.SetRoot(root)
	m.schemaTree.SetCurrentNode(root)

	if m.Driver != nil {
		m.loadSchemas(root)
	}

	// Content area (right panel)
	m.contentTv.SetBorder(true)
	m.contentTv.SetTitle(" Content ")
	m.contentTv.SetText("Select a table from the schema tree")

	// Status bar
	m.statusTv.SetBorder(false)
	m.statusTv.SetDynamicColors(true)
	m.statusTv.SetTextAlign(tview.AlignLeft)
	m.statusTv.SetText("[yellow]Ready")

	schemaPanelWidth := m.App.GetConfig().UI.SchemaPanelWidth
	if schemaPanelWidth == 0 {
		schemaPanelWidth = 30
	}

	m.AddItem(m.schemaTree, schemaPanelWidth, 0, true)
	m.AddItem(m.innerFlex, 0, 7, false)
	m.innerFlex.AddItem(m.headerTv, 3, 0, false)
	m.innerFlex.AddItem(m.contentTv, 0, 7, true)
	m.innerFlex.AddItem(m.statusTv, 1, 0, false)
}

func (m *Main) renderHeader() {
	if m.Driver == nil {
		m.headerTv.SetText("[yellow]Not connected")
		return
	}

	conn := m.App.GetConfig().GetCurrentConnection()
	if conn == nil {
		m.headerTv.SetText("[yellow]No active connection")
		return
	}

	headerText := fmt.Sprintf(
		" [yellow]Connection:[-] %s  [yellow]Host:[-] %s  [yellow]Database:[-] %s",
		conn.Name, conn.Host, conn.Database,
	)
	m.headerTv.SetText(headerText)
}

func (m *Main) loadSchemas(root *tview.TreeNode) {
	ctx := context.Background()
	schemas, err := m.Driver.ListSchemasWithTables(ctx, "")
	if err != nil {
		root.AddChild(tview.NewTreeNode(fmt.Sprintf("Error: %v", err)))
		return
	}

	for _, s := range schemas {
		schemaNode := tview.NewTreeNode(s.Schema).
			SetSelectable(true).
			SetExpanded(false)

		for _, t := range s.Tables {
			tableName := t
			tableNode := tview.NewTreeNode(tableName).
				SetSelectable(true).
				SetReference(schemaTable{Schema: s.Schema, Table: tableName})

			tableNode.SetSelectedFunc(func() {
				m.onTableSelected(s.Schema, tableName)
			})

			schemaNode.AddChild(tableNode)
		}

		root.AddChild(schemaNode)
	}
}

type schemaTable struct {
	Schema string
	Table  string
}

func (m *Main) onTableSelected(schema, table string) {
	m.contentTv.SetTitle(fmt.Sprintf(" %s.%s ", schema, table))
	m.statusTv.SetText(fmt.Sprintf("[yellow]Loading[-] %s.%s...", schema, table))

	ctx := context.Background()
	state := database.NewTableState(schema, table)
	state.Limit = 50

	rows, err := m.Driver.ListRows(ctx, state, "", "", nil, func(count int64) {
		state.Count = count
		go m.App.QueueUpdateDraw(func() {
			m.statusTv.SetText(fmt.Sprintf("[yellow]%s.%s[-] | Rows: %d | Page: %d/%d",
				schema, table, state.Count, state.GetCurrentPage(), state.GetTotalPages()))
		})
	})
	if err != nil {
		m.contentTv.SetText(fmt.Sprintf("Error loading rows: %v", err))
		return
	}

	if len(rows) == 0 {
		m.contentTv.SetText("(empty table)")
		return
	}

	cols := database.GetSortedColumnNames(rows[0])
	var buf fmt.Stringer = &rowFormatter{cols: cols, rows: rows}
	m.contentTv.SetText(buf.String())
}

type rowFormatter struct {
	cols []string
	rows []database.Row
}

func (r *rowFormatter) String() string {
	var b []byte

	// Header
	for i, col := range r.cols {
		if i > 0 {
			b = append(b, " | "...)
		}
		b = append(b, col...)
	}
	b = append(b, '\n')
	for i := range r.cols {
		if i > 0 {
			b = append(b, "-+-"...)
		}
		for j := 0; j < len(r.cols[i]); j++ {
			b = append(b, '-')
		}
	}
	b = append(b, '\n')

	// Rows
	for _, row := range r.rows {
		for i, col := range r.cols {
			if i > 0 {
				b = append(b, " | "...)
			}
			b = append(b, database.StringifyValue(row[col])...)
		}
		b = append(b, '\n')
	}

	return string(b)
}

func (m *Main) UpdateDriver(driver database.Driver) {
	m.BaseElement.UpdateDriver(driver)
}

func (m *Main) JumpToTable(schema, table string) error {
	if m.Driver == nil {
		return fmt.Errorf("not connected to a database")
	}

	// Expand the tree to the target table
	root := m.schemaTree.GetRoot()
	if root == nil {
		return fmt.Errorf("schema tree not loaded")
	}

	for _, schemaNode := range root.GetChildren() {
		if schemaNode.GetText() == schema {
			schemaNode.SetExpanded(true)
			for _, tableNode := range schemaNode.GetChildren() {
				if tableNode.GetText() == table {
					m.schemaTree.SetCurrentNode(tableNode)
					m.onTableSelected(schema, table)
					return nil
				}
			}
			return fmt.Errorf("table %q not found in schema %q", table, schema)
		}
	}

	return fmt.Errorf("schema %q not found", schema)
}

func (m *Main) setKeybindings() {
	k := m.App.GetKeys()
	m.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Contains(k.Main.FocusNext, event.Name()):
			if m.schemaTree.HasFocus() {
				m.App.SetFocus(m.contentTv)
			} else {
				m.App.SetFocus(m.schemaTree)
			}
			return nil
		case k.Contains(k.Main.FocusPrevious, event.Name()):
			if m.contentTv.HasFocus() {
				m.App.SetFocus(m.schemaTree)
			} else {
				m.App.SetFocus(m.contentTv)
			}
			return nil
		case k.Contains(k.Main.HideSchema, event.Name()):
			if _, ok := m.GetItem(0).(*core.TreeView); ok {
				m.RemoveItem(m.schemaTree)
				m.App.SetFocus(m.contentTv)
			} else {
				m.Clear()
				m.Render()
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

	ctx := context.Background()
	info, err := m.Driver.GetServerInfo(ctx)
	if err != nil {
		showError(m.App.Pages, "Failed to get server info", err)
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
