package component

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
	"github.com/kopecmaciej/vi-sql/internal/tui/primitives"
	"github.com/rs/zerolog/log"
)

const (
	SchemaTreeId        = "SchemaTree"
	SchemaFilterBarId   = "FilterBar"
	SchemaInputModalId  = "SchemaInputModal"
	SchemaDeleteModalId = "SchemaDeleteModal"
)

// SchemaTree is a flex container holding a filter bar and a tree view
// of schemas → tables.
type SchemaTree struct {
	*core.BaseElement
	*core.Flex

	tree      *core.TreeView
	filterBar *InputBar
	style     *config.SchemasStyle

	inputModal  *primitives.InputModal
	deleteModal *modal.Confirm

	mutex            sync.Mutex
	schemasWithTables []database.SchemaWithTables
	nodeSelectFunc    func(ctx context.Context, schema, table string) error
}

func NewSchemaTree() *SchemaTree {
	s := &SchemaTree{
		BaseElement: core.NewBaseElement(),
		Flex:        core.NewFlex(),
		tree:        core.NewTreeView(),
		filterBar:   NewInputBar(SchemaFilterBarId, "Filter"),
		inputModal:  primitives.NewInputModal(),
		deleteModal: modal.NewConfirm(SchemaDeleteModalId),
	}

	s.SetIdentifier(SchemaTreeId)
	s.SetAfterInitFunc(s.init)

	return s
}

func (s *SchemaTree) init() error {
	s.setStyle()
	s.setLayout()
	s.setKeybindings()

	s.tree.SetSelectedFunc(func(node *tview.TreeNode) {
		s.tree.SetCurrentNode(node)
	})

	if err := s.filterBar.Init(s.App); err != nil {
		return err
	}
	s.filterBarHandler()

	if err := s.deleteModal.Init(s.App); err != nil {
		return err
	}

	s.handleEvents()

	return nil
}

func (s *SchemaTree) setLayout() {
	s.tree.SetBorder(true)
	s.tree.SetTitle(" Schemas ")
	s.tree.SetBorderPadding(0, 0, 1, 1)
	s.tree.SetGraphics(false)

	s.Flex.SetDirection(tview.FlexRow)

	s.inputModal.SetBorder(true)
	s.inputModal.SetTitle("Add table")
}

func (s *SchemaTree) setStyle() {
	globalStyle := s.App.GetStyles()
	s.Flex.SetStyle(globalStyle)
	s.tree.SetStyle(globalStyle)
	s.style = &globalStyle.Schemas

	s.inputModal.SetBorderColor(globalStyle.Global.BorderColor.Color())
	s.inputModal.SetBackgroundColor(globalStyle.Global.BackgroundColor.Color())
	s.inputModal.SetFieldTextColor(globalStyle.Others.ModalTextColor.Color())
	s.inputModal.SetFieldBackgroundColor(globalStyle.Global.ContrastBackgroundColor.Color())
}

func (s *SchemaTree) setKeybindings() {
	k := s.App.GetKeys()
	ctx := context.Background()

	closedNodeSymbol := config.SymbolWithColor(s.style.ClosedNodeSymbol, s.style.NodeSymbolColor)
	openNodeSymbol := config.SymbolWithColor(s.style.OpenNodeSymbol, s.style.NodeSymbolColor)

	s.tree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Contains(k.Schema.ExpandAll, event.Name()):
			s.expandAllNodes(closedNodeSymbol, openNodeSymbol)
			return nil
		case k.Contains(k.Schema.CollapseAll, event.Name()):
			s.collapseAllNodes(openNodeSymbol, closedNodeSymbol)
			return nil
		case k.Contains(k.Schema.AddTable, event.Name()):
			s.showAddTableModal(ctx)
			return nil
		case k.Contains(k.Schema.DeleteTable, event.Name()):
			s.showDeleteTableModal(ctx)
			return nil
		case k.Contains(k.Schema.RenameTable, event.Name()):
			s.showRenameTableModal(ctx)
			return nil
		case k.Contains(k.Schema.FilterBar, event.Name()):
			s.filterBar.Enable()
			s.renderLayout()
			return nil
		case k.Contains(k.Schema.ClearFilter, event.Name()):
			s.clearFilter()
			return nil
		}
		return event
	})
}

func (s *SchemaTree) handleEvents() {
	go s.HandleEvents(SchemaTreeId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			s.setStyle()
			s.refreshStyle()
		}
	})
}

func (s *SchemaTree) Render() {
	ctx := context.Background()

	if err := s.listSchemasWithTables(ctx); err != nil {
		modal.ShowError(s.App.Pages, "Failed to list schemas", err)
		s.schemasWithTables = []database.SchemaWithTables{}
	}

	s.renderTree(s.schemasWithTables, false)
	s.renderLayout()
}

func (s *SchemaTree) renderTree(schemas []database.SchemaWithTables, expand bool) {
	ctx := context.Background()
	rootNode := s.rootNode()
	s.tree.SetRoot(rootNode)

	if len(schemas) == 0 {
		emptyNode := tview.NewTreeNode("No schemas found")
		emptyNode.SetSelectable(false)
		rootNode.AddChild(emptyNode)
	}

	for _, schema := range schemas {
		parent := s.schemaNode(schema.Schema)
		rootNode.AddChild(parent)

		for _, table := range schema.Tables {
			s.addTableNode(ctx, parent, schema.Schema, table, false)
		}
	}

	children := rootNode.GetChildren()
	if len(children) > 0 {
		s.tree.SetCurrentNode(children[0])
	}
	if expand {
		rootNode.ExpandAll()
	}
}

func (s *SchemaTree) renderLayout() {
	s.Flex.Clear()

	var primitive tview.Primitive
	primitive = s.tree

	if s.filterBar.IsEnabled() {
		s.Flex.AddItem(s.filterBar, 3, 0, false)
		primitive = s.filterBar
	}
	defer s.App.SetFocus(primitive)

	s.Flex.AddItem(s.tree, 0, 1, true)
}

func (s *SchemaTree) IsFocused() bool {
	focus := s.App.GetFocus()
	if focus == nil {
		return false
	}
	id := focus.GetIdentifier()
	return id == s.GetIdentifier() || id == s.tree.GetIdentifier()
}

func (s *SchemaTree) SetSelectFunc(f func(ctx context.Context, schema, table string) error) {
	s.nodeSelectFunc = f
}

func (s *SchemaTree) listSchemasWithTables(ctx context.Context) error {
	schemas, err := s.Driver.ListSchemasWithTables(ctx, "")
	if err != nil {
		return err
	}
	s.schemasWithTables = schemas
	return nil
}

func (s *SchemaTree) rootNode() *tview.TreeNode {
	r := tview.NewTreeNode("")
	r.SetColor(s.style.NodeTextColor.Color())
	r.SetSelectable(false)
	r.SetExpanded(true)
	return r
}

func (s *SchemaTree) schemaNode(name string) *tview.TreeNode {
	openNodeSymbol := config.SymbolWithColor(s.style.OpenNodeSymbol, s.style.NodeSymbolColor)
	closedNodeSymbol := config.SymbolWithColor(s.style.ClosedNodeSymbol, s.style.NodeSymbolColor)
	r := tview.NewTreeNode(fmt.Sprintf("%s %s", closedNodeSymbol, name))
	r.SetColor(s.style.NodeTextColor.Color())
	r.SetSelectable(true)
	r.SetExpanded(false)

	r.SetSelectedFunc(func() {
		if r.IsExpanded() {
			r.SetText(fmt.Sprintf("%s %s", closedNodeSymbol, name))
		} else {
			r.SetText(fmt.Sprintf("%s %s", openNodeSymbol, name))
		}
		r.SetExpanded(!r.IsExpanded())
	})

	return r
}

func (s *SchemaTree) tableNode(name string) *tview.TreeNode {
	leafSymbol := config.SymbolWithColor(s.style.LeafSymbol, s.style.LeafSymbolColor)
	ch := tview.NewTreeNode(fmt.Sprintf("%s %s", leafSymbol, name))
	ch.SetColor(s.style.LeafTextColor.Color())
	ch.SetSelectable(true)
	ch.SetExpanded(false)
	return ch
}

func (s *SchemaTree) addTableNode(ctx context.Context, parent *tview.TreeNode, schemaName, tableName string, expand bool) {
	node := s.tableNode(tableName)
	parent.AddChild(node).SetExpanded(expand)
	node.SetReference(parent)
	node.SetSelectedFunc(func() {
		_, cleanTable := s.removeSymbols(parent.GetText(), node.GetText())
		cleanSchema, _ := s.removeSymbols(parent.GetText(), "")
		err := s.nodeSelectFunc(ctx, cleanSchema, cleanTable)
		if err != nil {
			log.Error().Err(err).Msg("Error selecting table")
			modal.ShowError(s.App.Pages, "Error selecting table", err)
		}
	})
}

func (s *SchemaTree) expandAllNodes(closedSymbol, openSymbol string) {
	s.tree.GetRoot().ExpandAll()
	s.tree.GetRoot().Walk(func(node, parent *tview.TreeNode) bool {
		s.setNodeSymbol(node, closedSymbol, openSymbol)
		return true
	})
}

func (s *SchemaTree) collapseAllNodes(openSymbol, closedSymbol string) {
	s.tree.GetRoot().CollapseAll()
	s.tree.GetRoot().SetExpanded(true)
	s.tree.GetRoot().Walk(func(node, parent *tview.TreeNode) bool {
		s.setNodeSymbol(node, openSymbol, closedSymbol)
		return true
	})
}

func (s *SchemaTree) setNodeSymbol(node *tview.TreeNode, oldSymbol, newSymbol string) {
	text := node.GetText()
	node.SetText(strings.Replace(text, oldSymbol, newSymbol, 1))
}

func (s *SchemaTree) removeSymbols(schema, table string) (string, string) {
	openNodeSymbol := config.SymbolWithColor(s.style.OpenNodeSymbol, s.style.NodeSymbolColor)
	closedNodeSymbol := config.SymbolWithColor(s.style.ClosedNodeSymbol, s.style.NodeSymbolColor)
	leafSymbol := config.SymbolWithColor(s.style.LeafSymbol, s.style.LeafSymbolColor)
	symbolsToRemove := []string{openNodeSymbol, closedNodeSymbol, leafSymbol}

	for _, symbol := range symbolsToRemove {
		schema = strings.ReplaceAll(schema, symbol, "")
		table = strings.ReplaceAll(table, symbol, "")
	}

	return strings.TrimSpace(schema), strings.TrimSpace(table)
}

func (s *SchemaTree) refreshStyle() {
	root := s.tree.GetRoot()
	if root == nil {
		return
	}
	root.Walk(func(node, parent *tview.TreeNode) bool {
		if parent != nil {
			s.updateNodeSymbol(parent)
		}
		s.updateLeafSymbol(node)
		return true
	})
}

func (s *SchemaTree) updateNodeSymbol(node *tview.TreeNode) {
	node.SetColor(s.style.NodeTextColor.Color())
	openNodeSymbol := config.SymbolWithColor(s.style.OpenNodeSymbol, s.style.NodeSymbolColor)
	closedNodeSymbol := config.SymbolWithColor(s.style.ClosedNodeSymbol, s.style.NodeSymbolColor)
	currText := strings.Split(node.GetText(), " ")
	if len(currText) < 2 {
		return
	}
	name := currText[1]
	if node.IsExpanded() {
		node.SetText(fmt.Sprintf("%s %s", openNodeSymbol, name))
	} else {
		node.SetText(fmt.Sprintf("%s %s", closedNodeSymbol, name))
	}

	node.SetSelectedFunc(func() {
		if node.IsExpanded() {
			node.SetText(fmt.Sprintf("%s %s", closedNodeSymbol, name))
		} else {
			node.SetText(fmt.Sprintf("%s %s", openNodeSymbol, name))
		}
		node.SetExpanded(!node.IsExpanded())
	})
}

func (s *SchemaTree) updateLeafSymbol(node *tview.TreeNode) {
	node.SetColor(s.style.LeafTextColor.Color())
	leafSymbol := config.SymbolWithColor(s.style.LeafSymbol, s.style.LeafSymbolColor)
	currText := strings.Split(node.GetText(), " ")
	if len(currText) < 2 {
		return
	}
	node.SetText(fmt.Sprintf("%s %s", leafSymbol, currText[1]))
}

// --- Filter ---

func (s *SchemaTree) filterBarHandler() {
	acceptFunc := func(text string) {
		s.filter(text)
	}
	rejectFunc := func() {
		s.renderLayout()
	}
	s.filterBar.DoneFuncHandler(acceptFunc, rejectFunc)
}

func (s *SchemaTree) clearFilter() {
	s.filterBar.SetText("")
	s.renderTree(s.schemasWithTables, false)
	s.renderLayout()
}

func (s *SchemaTree) filter(text string) {
	expand := true
	filtered := []database.SchemaWithTables{}
	if text == "" {
		filtered = s.schemasWithTables
		expand = false
	} else {
		re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(text))
		for _, st := range s.schemasWithTables {
			matchedSchema := re.MatchString(st.Schema)
			matchedTables := []string{}

			for _, t := range st.Tables {
				if re.MatchString(t) {
					matchedTables = append(matchedTables, t)
				}
			}

			if matchedSchema || len(matchedTables) > 0 {
				filteredST := database.SchemaWithTables{
					Schema: st.Schema,
					Tables: matchedTables,
				}
				if matchedSchema {
					filteredST.Tables = st.Tables
				}
				filtered = append(filtered, filteredST)
				expand = expand || len(matchedTables) > 0
			}
		}
	}
	s.renderTree(filtered, expand)
	s.renderLayout()
}

// --- DDL Modals ---

func (s *SchemaTree) getParentNode() *tview.TreeNode {
	current := s.tree.GetCurrentNode()
	if current == nil {
		return nil
	}
	level := current.GetLevel()
	if level == 0 {
		return nil
	}
	if level == 1 {
		return current
	}
	return current.GetReference().(*tview.TreeNode)
}

func (s *SchemaTree) showAddTableModal(ctx context.Context) {
	parent := s.getParentNode()
	if parent == nil {
		return
	}
	schemaName, _ := s.removeSymbols(parent.GetText(), "")

	s.inputModal.SetLabel(fmt.Sprintf("Add table to [%s][::b]%s", s.style.NodeTextColor.Color(), schemaName))
	s.inputModal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			tableName := s.inputModal.GetText()
			if tableName == "" {
				return event
			}
			ddl := fmt.Sprintf("CREATE TABLE %s (id serial PRIMARY KEY)", tableName)
			err := s.Driver.CreateTable(ctx, schemaName, ddl)
			if err != nil {
				modal.ShowError(s.App.Pages, "Error creating table", err)
				return event
			}
			s.addTableNode(ctx, parent, schemaName, tableName, true)
			s.closeInputModal()
		case tcell.KeyEscape:
			s.closeInputModal()
		}
		return event
	})
	s.App.Pages.AddPage(SchemaInputModalId, s.inputModal, true, true)
}

func (s *SchemaTree) closeInputModal() {
	s.inputModal.SetText("")
	s.App.Pages.RemovePage(SchemaInputModalId)
}

func (s *SchemaTree) showDeleteTableModal(ctx context.Context) {
	current := s.tree.GetCurrentNode()
	if current == nil || current.GetLevel() < 2 {
		return
	}
	parent := current.GetReference().(*tview.TreeNode)
	schemaName, tableName := s.removeSymbols(parent.GetText(), current.GetText())

	s.deleteModal.SetText(fmt.Sprintf("Are you sure you want to drop [%s]%s[-:-:-] [white]from [%s]%s[-:-:-]?",
		s.style.LeafTextColor.Color(), tableName, s.style.NodeTextColor.Color(), schemaName))
	s.deleteModal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		defer s.App.Pages.RemovePage(SchemaDeleteModalId)
		if buttonIndex == 0 {
			err := s.Driver.DropTable(ctx, schemaName, tableName)
			if err != nil {
				modal.ShowError(s.App.Pages, "Error dropping table", err)
				return
			}
			s.removeTableNode(parent, current)
		}
	})
	s.App.Pages.AddPage(SchemaDeleteModalId, s.deleteModal, true, true)
}

func (s *SchemaTree) removeTableNode(parent, current *tview.TreeNode) {
	children := parent.GetChildren()
	index := -1
	for i, ch := range children {
		if ch.GetText() == current.GetText() {
			index = i
			break
		}
	}
	parent.RemoveChild(current)
	if index == 0 && len(children) > 1 {
		s.tree.SetCurrentNode(parent.GetChildren()[0])
	} else if index > 0 {
		s.tree.SetCurrentNode(parent.GetChildren()[index-1])
	}
}

func (s *SchemaTree) showRenameTableModal(ctx context.Context) {
	current := s.tree.GetCurrentNode()
	if current == nil || current.GetLevel() < 2 {
		return
	}
	parent := current.GetReference().(*tview.TreeNode)
	schemaName, oldName := s.removeSymbols(parent.GetText(), current.GetText())

	s.inputModal.SetLabel(fmt.Sprintf("Rename table [%s][::b]%s.%s", s.style.NodeTextColor.Color(), schemaName, oldName))
	s.inputModal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			newName := s.inputModal.GetText()
			if newName == "" {
				return event
			}
			err := s.Driver.RenameTable(ctx, schemaName, oldName, newName)
			if err != nil {
				modal.ShowError(s.App.Pages, "Error renaming table", err)
				return event
			}
			leafSymbol := config.SymbolWithColor(s.style.LeafSymbol, s.style.LeafSymbolColor)
			current.SetText(fmt.Sprintf("%s %s", leafSymbol, newName))
			s.closeInputModal()
		case tcell.KeyEscape:
			s.closeInputModal()
		}
		return event
	})
	s.App.Pages.AddPage(SchemaInputModalId, s.inputModal, true, true)
}

// JumpToTable expands the given schema and selects the given table.
func (s *SchemaTree) JumpToTable(ctx context.Context, targetSchema, targetTable string) error {
	root := s.tree.GetRoot()
	if root == nil {
		return fmt.Errorf("tree not initialized")
	}

	for _, schemaNode := range root.GetChildren() {
		cleanSchema, _ := s.removeSymbols(schemaNode.GetText(), "")

		if cleanSchema == targetSchema {
			schemaNode.SetExpanded(true)
			openNodeSymbol := config.SymbolWithColor(s.style.OpenNodeSymbol, s.style.NodeSymbolColor)
			schemaNode.SetText(fmt.Sprintf("%s %s", openNodeSymbol, cleanSchema))

			for _, tableNode := range schemaNode.GetChildren() {
				_, cleanTable := s.removeSymbols("", tableNode.GetText())
				if cleanTable == targetTable {
					s.tree.SetCurrentNode(tableNode)
					if s.nodeSelectFunc != nil {
						return s.nodeSelectFunc(ctx, targetSchema, targetTable)
					}
					return nil
				}
			}
			return fmt.Errorf("table %q not found in schema %q", targetTable, targetSchema)
		}
	}

	return fmt.Errorf("schema %q not found", targetSchema)
}
