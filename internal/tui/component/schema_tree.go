package component

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/manager"
	sqlpkg "github.com/kopecmaciej/vi-sql/internal/sql"
	"github.com/kopecmaciej/vi-sql/internal/tui/core"
	"github.com/kopecmaciej/vi-sql/internal/tui/modal"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog/log"
)

const (
	SchemaTreeId        = "Schema"
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
	style     *config.IconStyle

	inputModal       *core.InputField
	deleteModal      *modal.Confirm
	createTableModal *modal.CreateTableModal

	schemas            []database.Schema
	viewNodes          map[*tview.TreeNode]bool
	nodeSelectFunc     func(ctx context.Context, schema, table string) error
	nodeViewSelectFunc func(ctx context.Context, schema, view string) error
	nodeColumnsFunc    func(ctx context.Context, schema, table string)
	nodeIndexesFunc    func(ctx context.Context, schema, table string)
	nodeViewDDLFunc    func(ctx context.Context, schema, view string)
	onSchemasLoaded    func([]database.Schema)
	onImport           func(schema, table string)
}

// UpdateDriver propagates the driver change to child components that use it.
func (s *SchemaTree) UpdateDriver(driver database.Driver) {
	s.BaseElement.UpdateDriver(driver)
	s.createTableModal.UpdateDriver(driver)
}

func NewSchemaTree() *SchemaTree {
	s := &SchemaTree{
		BaseElement:      core.NewBaseElement(),
		Flex:             core.NewFlex(),
		tree:             core.NewTreeView(),
		filterBar:        NewInputBar(SchemaFilterBarId, "Filter"),
		inputModal:       core.NewInputField(),
		deleteModal:      modal.NewConfirm(SchemaDeleteModalId),
		createTableModal: modal.NewCreateTableModal(),
	}

	s.SetIdentifier(SchemaTreeId)
	s.tree.SetIdentifier(SchemaTreeId)
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

	if err := s.createTableModal.Init(s.App); err != nil {
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
	s.inputModal.SetTitle(" Rename table ")
}

func (s *SchemaTree) setStyle() {
	styles := s.App.GetStyles()
	s.Flex.SetStyle(styles)
	s.tree.SetStyle(styles)
	s.style = &styles.Icons

	s.inputModal.SetStyle(styles)
	s.inputModal.SetBorderPadding(1, 1, 2, 2)
}

func (s *SchemaTree) setKeybindings() {
	k := s.App.GetKeys()
	ctx := context.Background()

	closedNodeIcon := s.style.IconWithColor(s.style.ClosedNode, s.App.GetStyles().Global.SecondaryTextColor)
	openNodeIcon := s.style.IconWithColor(s.style.OpenNode, s.App.GetStyles().Global.SecondaryTextColor)

	s.tree.SetInputCapture(k.WrapInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case k.Match(k.Navigation.GoTop, event):
			root := s.tree.GetRoot()
			if root == nil {
				return nil
			}
			if children := root.GetChildren(); len(children) > 0 {
				s.tree.SetCurrentNode(children[0])
			}
			return nil
		case k.Match(k.Navigation.GoBottom, event):
			root := s.tree.GetRoot()
			if root != nil {
				if last := lastVisibleTreeNode(root); last != nil {
					s.tree.SetCurrentNode(last)
				}
			}
			return nil
		case k.Match(k.Schema.ExpandAll, event):
			s.expandAllNodes(closedNodeIcon, openNodeIcon)
			return nil
		case k.Match(k.Schema.CollapseAll, event):
			s.collapseAllNodes(openNodeIcon, closedNodeIcon)
			return nil
		case k.Match(k.Common.Add, event):
			s.showAddTableModal(ctx)
			return nil
		case k.Match(k.Common.Delete, event):
			current := s.tree.GetCurrentNode()
			if current != nil && !s.isViewNode(current) {
				s.showDeleteTableModal(ctx)
			}
			return nil
		case k.Match(k.Schema.RenameTable, event):
			current := s.tree.GetCurrentNode()
			if current != nil && !s.isViewNode(current) {
				s.showRenameTableModal(ctx)
			}
			return nil
		case k.Match(k.Schema.ExpandTable, event):
			current := s.tree.GetCurrentNode()
			if current != nil && current.GetLevel() >= 2 {
				current.SetExpanded(!current.IsExpanded())
			}
			return nil
		case k.Match(k.Schema.OpenStructure, event):
			current := s.tree.GetCurrentNode()
			if current != nil && current.GetLevel() >= 2 {
				schema, name := s.SelectedTable()
				if s.isViewNode(current) {
					if s.nodeViewDDLFunc != nil {
						s.nodeViewDDLFunc(ctx, schema, name)
					}
				} else if s.nodeColumnsFunc != nil {
					s.nodeColumnsFunc(ctx, schema, name)
				}
			}
			return nil
		case k.Match(k.Schema.OpenIndexes, event):
			current := s.tree.GetCurrentNode()
			if s.nodeIndexesFunc != nil && current != nil && current.GetLevel() >= 2 && !s.isViewNode(current) {
				schema, table := s.SelectedTable()
				s.nodeIndexesFunc(ctx, schema, table)
			}
			return nil
		case k.Match(k.Common.Copy, event):
			s.copyCurrentNode()
			return nil
		case k.Match(k.Common.Filter, event):
			s.filterBar.Enable()
			s.renderLayout()
			return nil
		case k.Match(k.Common.Clear, event):
			s.clearFilter()
			return nil
		case k.Match(k.Common.Refresh, event):
			s.reloadTree(ctx)
			return nil
		}
		return event
	}))
}

func (s *SchemaTree) handleEvents() {
	go s.HandleEvents(SchemaTreeId, func(event manager.EventMsg) {
		switch event.Message.Type {
		case manager.StyleChanged:
			s.setStyle()
			s.refreshStyle()
		case manager.QueryExecuted:
			result, ok := event.Message.Data.(manager.QueryResult)
			if !ok || !isDDLQuery(result.Query) {
				return
			}
			go s.App.Application.QueueUpdateDraw(func() {
				s.reloadTree(context.Background())
			})
		}
	})
}

func (s *SchemaTree) expandedSchemas() map[string]bool {
	expanded := map[string]bool{}
	root := s.tree.GetRoot()
	if root == nil {
		return expanded
	}
	for _, node := range root.GetChildren() {
		if node.IsExpanded() {
			expanded[extractName(node.GetText())] = true
		}
	}
	return expanded
}

func (s *SchemaTree) restoreExpanded(expanded map[string]bool) {
	if len(expanded) == 0 {
		return
	}
	openIcon := s.style.IconWithColor(s.style.OpenNode, s.App.GetStyles().Global.SecondaryTextColor)
	root := s.tree.GetRoot()
	if root == nil {
		return
	}
	for _, node := range root.GetChildren() {
		name := extractName(node.GetText())
		if expanded[name] {
			node.SetExpanded(true)
			node.SetText(fmt.Sprintf("%s%s", openIcon, name))
		}
	}
}

type treeSelection struct {
	schema string
	table  string
}

func (s *SchemaTree) currentSelection() treeSelection {
	current := s.tree.GetCurrentNode()
	if current == nil {
		return treeSelection{}
	}
	if current.GetLevel() == 1 {
		schema, _ := s.removeIcons(current.GetText(), "")
		return treeSelection{schema: schema}
	}
	if current.GetLevel() >= 2 {
		schema, table := s.SelectedTable()
		return treeSelection{schema: schema, table: table}
	}
	return treeSelection{}
}

func (s *SchemaTree) restoreSelection(sel treeSelection) {
	if sel.schema == "" {
		return
	}
	root := s.tree.GetRoot()
	if root == nil {
		return
	}
	for _, schemaNode := range root.GetChildren() {
		if extractName(schemaNode.GetText()) != sel.schema {
			continue
		}
		if sel.table == "" {
			s.tree.SetCurrentNode(schemaNode)
			return
		}
		for _, tableNode := range schemaNode.GetChildren() {
			if extractName(tableNode.GetText()) == sel.table {
				s.tree.SetCurrentNode(tableNode)
				return
			}
		}
		// table was deleted — fall back to the schema node
		s.tree.SetCurrentNode(schemaNode)
		return
	}
}

// lastVisibleTreeNode returns the last visible node in a tree, descending into
// expanded nodes recursively. Call with the invisible root; it returns the last
// selectable node the user can see.
func lastVisibleTreeNode(node *tview.TreeNode) *tview.TreeNode {
	children := node.GetChildren()
	if !node.IsExpanded() || len(children) == 0 {
		return node
	}
	return lastVisibleTreeNode(children[len(children)-1])
}

// Must be called from the tview goroutine — expandedSchemas/currentSelection are not goroutine-safe.
func (s *SchemaTree) reloadTree(ctx context.Context) {
	expanded := s.expandedSchemas()
	sel := s.currentSelection()
	go func() {
		if err := s.ListSchemas(ctx); err != nil {
			return
		}
		go s.App.Application.QueueUpdateDraw(func() {
			s.renderTree(s.schemas, false)
			s.restoreExpanded(expanded)
			s.restoreSelection(sel)
		})
	}()
}

func isDDLQuery(sql string) bool {
	tokens := sqlpkg.Tokenize(sql)
	for _, t := range tokens {
		if t.Type == sqlpkg.TokenWhitespace || t.Type == sqlpkg.TokenComment {
			continue
		}
		switch strings.ToUpper(t.Value) {
		case "CREATE", "DROP", "ALTER", "RENAME":
			return true
		}
		return false
	}
	return false
}

func (s *SchemaTree) Render() {
	ctx := context.Background()

	if err := s.ListSchemas(ctx); err != nil {
		modal.ShowError(s.App.Pages, "Failed to list schemas", err)
		s.schemas = []database.Schema{}
	}

	s.renderTree(s.schemas, false)
	s.renderLayout()
}

func (s *SchemaTree) renderTree(schemas []database.Schema, expand bool) {
	ctx := context.Background()
	rootNode := s.rootNode()
	s.tree.SetRoot(rootNode)
	s.viewNodes = make(map[*tview.TreeNode]bool)

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
		for _, view := range schema.Views {
			s.addViewNode(ctx, parent, schema.Schema, view)
		}
	}

	children := rootNode.GetChildren()
	if len(children) > 0 {
		s.tree.SetCurrentNode(children[0])
	}
	if expand {
		for _, schemaNode := range rootNode.GetChildren() {
			schemaNode.SetExpanded(true)
		}
	}
}

func (s *SchemaTree) renderLayout() {
	s.Flex.Clear()

	var focusTarget tview.Primitive = s.tree
	if s.filterBar.IsEnabled() {
		s.Flex.AddItem(s.filterBar, 3, 0, false)
		focusTarget = s.filterBar
	}
	s.Flex.AddItem(s.tree, 0, 1, true)
	s.App.SetFocusOnly(focusTarget)
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

func (s *SchemaTree) SetColumnsFunc(f func(ctx context.Context, schema, table string)) {
	s.nodeColumnsFunc = f
}

func (s *SchemaTree) SetIndexesFunc(f func(ctx context.Context, schema, table string)) {
	s.nodeIndexesFunc = f
}

func (s *SchemaTree) SetViewSelectFunc(f func(ctx context.Context, schema, view string) error) {
	s.nodeViewSelectFunc = f
}

func (s *SchemaTree) SetViewDDLFunc(f func(ctx context.Context, schema, view string)) {
	s.nodeViewDDLFunc = f
}

func (s *SchemaTree) IsViewSelected() bool {
	current := s.tree.GetCurrentNode()
	return current != nil && s.isViewNode(current)
}

func (s *SchemaTree) SetOnSchemasLoaded(fn func([]database.Schema)) {
	s.onSchemasLoaded = fn
}

func (s *SchemaTree) ListSchemas(ctx context.Context) error {
	schemas, err := s.Driver.ListSchemas(ctx, "")
	if err != nil {
		return err
	}
	s.schemas = schemas
	if s.onSchemasLoaded != nil {
		s.onSchemasLoaded(schemas)
	}
	return nil
}

func (s *SchemaTree) rootNode() *tview.TreeNode {
	r := tview.NewTreeNode("")
	r.SetColor(s.App.GetStyles().Global.MoreContrastBackgroundColor.Color())
	r.SetSelectable(false)
	r.SetExpanded(true)
	return r
}

func (s *SchemaTree) schemaNode(name string) *tview.TreeNode {
	openNodeIcon := s.style.IconWithColor(s.style.OpenNode, s.App.GetStyles().Global.SecondaryTextColor)
	closedNodeIcon := s.style.IconWithColor(s.style.ClosedNode, s.App.GetStyles().Global.SecondaryTextColor)
	r := tview.NewTreeNode(fmt.Sprintf("%s%s", closedNodeIcon, name))
	r.SetColor(s.App.GetStyles().Global.MoreContrastBackgroundColor.Color())
	r.SetSelectable(true)
	r.SetExpanded(false)

	r.SetSelectedFunc(func() {
		if r.IsExpanded() {
			r.SetText(fmt.Sprintf("%s%s", closedNodeIcon, name))
		} else {
			r.SetText(fmt.Sprintf("%s%s", openNodeIcon, name))
		}
		r.SetExpanded(!r.IsExpanded())
	})

	return r
}

func (s *SchemaTree) tableNode(name string) *tview.TreeNode {
	leafIcon := s.style.IconWithColor(s.style.Leaf, s.App.GetStyles().Others.LeafIconColor)
	ch := tview.NewTreeNode(fmt.Sprintf("%s%s", leafIcon, name))
	ch.SetColor(s.App.GetStyles().Global.TextColor.Color())
	ch.SetSelectable(true)
	ch.SetExpanded(false)
	return ch
}

func (s *SchemaTree) addTableNode(ctx context.Context, parent *tview.TreeNode, schemaName, tableName string, expand bool) {
	node := s.tableNode(tableName)
	parent.AddChild(node).SetExpanded(expand)
	node.SetReference(parent)
	node.SetSelectedFunc(func() {
		if s.nodeSelectFunc != nil {
			if err := s.nodeSelectFunc(ctx, schemaName, tableName); err != nil {
				log.Error().Err(err).Msg("Error selecting table")
				modal.ShowError(s.App.Pages, "Error selecting table", err)
			}
		}
	})
}

func (s *SchemaTree) viewNode(name string) *tview.TreeNode {
	viewIcon := s.style.IconWithColor(s.style.View, s.App.GetStyles().Others.LeafIconColor)
	ch := tview.NewTreeNode(fmt.Sprintf("%s%s", viewIcon, name))
	ch.SetColor(s.App.GetStyles().Global.TextColor.Color())
	ch.SetSelectable(true)
	ch.SetExpanded(false)
	return ch
}

func (s *SchemaTree) addViewNode(ctx context.Context, parent *tview.TreeNode, schemaName, viewName string) {
	node := s.viewNode(viewName)
	parent.AddChild(node)
	s.viewNodes[node] = true
	node.SetReference(parent)
	node.SetSelectedFunc(func() {
		if s.nodeViewSelectFunc != nil {
			if err := s.nodeViewSelectFunc(ctx, schemaName, viewName); err != nil {
				log.Error().Err(err).Msg("Error selecting view")
				modal.ShowError(s.App.Pages, "Error selecting view", err)
			}
		}
	})
}

func (s *SchemaTree) isViewNode(node *tview.TreeNode) bool {
	return s.viewNodes != nil && s.viewNodes[node]
}

func (s *SchemaTree) expandAllNodes(closedIcon, openIcon string) {
	s.tree.GetRoot().ExpandAll()
	s.tree.GetRoot().Walk(func(node, parent *tview.TreeNode) bool {
		s.setNodeIcon(node, closedIcon, openIcon)
		return true
	})
}

func (s *SchemaTree) collapseAllNodes(openIcon, closedIcon string) {
	s.tree.GetRoot().CollapseAll()
	s.tree.GetRoot().SetExpanded(true)
	s.tree.GetRoot().Walk(func(node, parent *tview.TreeNode) bool {
		s.setNodeIcon(node, openIcon, closedIcon)
		return true
	})
}

func (s *SchemaTree) setNodeIcon(node *tview.TreeNode, oldIcon, newIcon string) {
	text := node.GetText()
	node.SetText(strings.Replace(text, oldIcon, newIcon, 1))
}

func (s *SchemaTree) removeIcons(schema, table string) (string, string) {
	return extractName(schema), extractName(table)
}

func (s *SchemaTree) refreshStyle() {
	root := s.tree.GetRoot()
	if root == nil {
		return
	}
	root.Walk(func(node, parent *tview.TreeNode) bool {
		if parent == nil {
			return true
		}
		// Table and view nodes have a *tview.TreeNode reference pointing to their schema node.
		if _, isLeafRef := node.GetReference().(*tview.TreeNode); isLeafRef {
			if s.isViewNode(node) {
				s.updateViewIcon(node)
			} else {
				s.updateLeafIcon(node)
			}
			return true
		}
		s.updateNodeIcon(node)
		return true
	})
}

func extractName(text string) string {
	const resetTag = "[-:-:-]"
	idx := strings.LastIndex(text, resetTag)
	if idx == -1 {
		return strings.TrimSpace(text)
	}
	return strings.TrimSpace(text[idx+len(resetTag):])
}

func (s *SchemaTree) updateNodeIcon(node *tview.TreeNode) {
	node.SetColor(s.App.GetStyles().Global.MoreContrastBackgroundColor.Color())
	openNodeIcon := s.style.IconWithColor(s.style.OpenNode, s.App.GetStyles().Global.SecondaryTextColor)
	closedNodeIcon := s.style.IconWithColor(s.style.ClosedNode, s.App.GetStyles().Global.SecondaryTextColor)
	name := extractName(node.GetText())
	if name == "" {
		return
	}
	if node.IsExpanded() {
		node.SetText(fmt.Sprintf("%s%s", openNodeIcon, name))
	} else {
		node.SetText(fmt.Sprintf("%s%s", closedNodeIcon, name))
	}

	node.SetSelectedFunc(func() {
		if node.IsExpanded() {
			node.SetText(fmt.Sprintf("%s%s", closedNodeIcon, name))
		} else {
			node.SetText(fmt.Sprintf("%s%s", openNodeIcon, name))
		}
		node.SetExpanded(!node.IsExpanded())
	})
}

func (s *SchemaTree) updateLeafIcon(node *tview.TreeNode) {
	node.SetColor(s.App.GetStyles().Global.TextColor.Color())
	leafIcon := s.style.IconWithColor(s.style.Leaf, s.App.GetStyles().Others.LeafIconColor)
	name := extractName(node.GetText())
	if name == "" {
		return
	}
	node.SetText(fmt.Sprintf("%s%s", leafIcon, name))
}

func (s *SchemaTree) updateViewIcon(node *tview.TreeNode) {
	node.SetColor(s.App.GetStyles().Global.TextColor.Color())
	viewIcon := s.style.IconWithColor(s.style.View, s.App.GetStyles().Others.LeafIconColor)
	name := extractName(node.GetText())
	if name == "" {
		return
	}
	node.SetText(fmt.Sprintf("%s%s", viewIcon, name))
}

func (s *SchemaTree) copyCurrentNode() {
	current := s.tree.GetCurrentNode()
	if current == nil {
		return
	}
	level := current.GetLevel()
	if level == 1 {
		schema, _ := s.removeIcons(current.GetText(), "")
		util.Copy(schema)
	} else if level >= 2 {
		parent := current.GetReference().(*tview.TreeNode)
		schema, table := s.removeIcons(parent.GetText(), current.GetText())
		util.Copy(schema + "." + table)
	}
}

func (s *SchemaTree) filterBarHandler() {
	acceptFunc := func(_ string) {
		s.filterBar.Disable()
		s.renderLayout()
	}
	rejectFunc := func() {
		s.clearFilter()
	}
	s.filterBar.DoneFuncHandler(acceptFunc, rejectFunc)
	s.filterBar.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			acceptFunc(s.filterBar.GetText())
		}
	})
	s.filterBar.SetChangedFunc(func(text string) {
		s.filter(text)
	})
}

func (s *SchemaTree) clearFilter() {
	s.filterBar.SetText("")
	s.renderTree(s.schemas, false)
	s.renderLayout()
}

func (s *SchemaTree) filter(text string) {
	expand := false
	filtered := []database.Schema{}
	if text == "" {
		filtered = s.schemas
	} else {
		re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(text))
		for _, st := range s.schemas {
			matchedSchema := re.MatchString(st.Schema)
			matchedTables := []string{}
			for _, t := range st.Tables {
				if re.MatchString(t) {
					matchedTables = append(matchedTables, t)
				}
			}
			matchedViews := []string{}
			for _, v := range st.Views {
				if re.MatchString(v) {
					matchedViews = append(matchedViews, v)
				}
			}

			if matchedSchema || len(matchedTables) > 0 || len(matchedViews) > 0 {
				filteredST := database.Schema{
					Schema: st.Schema,
					Tables: matchedTables,
					Views:  matchedViews,
				}
				if matchedSchema {
					filteredST.Tables = st.Tables
					filteredST.Views = st.Views
				}
				filtered = append(filtered, filteredST)
				expand = expand || len(matchedTables) > 0 || len(matchedViews) > 0
			}
		}
	}
	s.renderTree(filtered, expand)
	s.renderLayout()
}

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
	schemaName, _ := s.removeIcons(parent.GetText(), "")

	s.createTableModal.SetSchema(schemaName)
	s.createTableModal.SetApplyCallback(func(ddl string) error {
		tableName := s.createTableModal.GetTableName()
		if err := s.Driver.CreateTable(ctx, schemaName, ddl); err != nil {
			return err
		}
		s.closeCreateTableModal()
		// Add the new table node directly without collapsing the whole tree.
		s.addTableNode(ctx, parent, schemaName, tableName, false)
		parent.SetExpanded(true)
		// Keep the in-memory cache consistent.
		for i, st := range s.schemas {
			if st.Schema == schemaName {
				s.schemas[i].Tables = append(s.schemas[i].Tables, tableName)
				break
			}
		}
		return nil
	})
	s.createTableModal.SetCancelCallback(s.closeCreateTableModal)
	s.createTableModal.Render("")
}

func (s *SchemaTree) closeCreateTableModal() {
	s.createTableModal.Hide()
}

func (s *SchemaTree) OpenCreateTable(ctx context.Context) {
	s.showAddTableModal(ctx)
}

func (s *SchemaTree) SetImportFunc(fn func(schema, table string)) {
	s.onImport = fn
}

func (s *SchemaTree) closeInputModal() {
	s.inputModal.SetText("")
	s.App.Pages.RemoveModalPage(SchemaInputModalId)
}

func (s *SchemaTree) showDeleteTableModal(ctx context.Context) {
	current := s.tree.GetCurrentNode()
	if current == nil || current.GetLevel() < 2 {
		return
	}
	parent := current.GetReference().(*tview.TreeNode)
	schemaName, tableName := s.removeIcons(parent.GetText(), current.GetText())

	s.deleteModal.SetText(fmt.Sprintf("Are you sure you want to drop [%s]%s[-:-:-] [white]from [%s]%s[-:-:-]?",
		s.App.GetStyles().Global.TextColor.Color(), tableName, s.App.GetStyles().Global.MoreContrastBackgroundColor.Color(), schemaName))
	s.deleteModal.SetOnConfirm(func() {
		// Remove the delete modal first so GiveBackFocus restores focus to the
		// tree before we potentially show an error modal on top.
		s.App.Pages.RemoveModalPage(SchemaDeleteModalId)
		err := s.Driver.DropTable(ctx, schemaName, tableName)
		if err != nil {
			modal.ShowError(s.App.Pages, "Error dropping table", err)
			return
		}
		s.removeTableNode(parent, current)
	})
	s.App.Pages.AddModalPage(SchemaDeleteModalId, s.deleteModal, true, true)
	s.App.SetFocusOnly(s.deleteModal)
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
	schemaName, oldName := s.removeIcons(parent.GetText(), current.GetText())

	s.inputModal.SetLabel(schemaName + ".")
	s.inputModal.SetText(oldName)
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
			leafIcon := s.style.IconWithColor(s.style.Leaf, s.App.GetStyles().Others.LeafIconColor)
			current.SetText(fmt.Sprintf("%s%s", leafIcon, newName))
			s.closeInputModal()
		case tcell.KeyEscape:
			s.closeInputModal()
		}
		return event
	})
	s.App.Pages.AddModalPage(SchemaInputModalId, core.CenteredFlex(s.inputModal, 2, 1), true, true)
	s.App.SetFocusOnly(s.inputModal)
}

func (s *SchemaTree) SelectedTable() (schema, table string) {
	current := s.tree.GetCurrentNode()
	if current == nil || current.GetLevel() < 2 {
		return "", ""
	}
	parent := current.GetReference().(*tview.TreeNode)
	schema, table = s.removeIcons(parent.GetText(), current.GetText())
	return schema, table
}

func (s *SchemaTree) JumpToTable(ctx context.Context, targetSchema, targetTable string) error {
	return s.jumpToLeaf(ctx, targetSchema, targetTable, false, "table", s.nodeSelectFunc)
}

func (s *SchemaTree) JumpToView(ctx context.Context, targetSchema, targetView string) error {
	return s.jumpToLeaf(ctx, targetSchema, targetView, true, "view", s.nodeViewSelectFunc)
}

func (s *SchemaTree) jumpToLeaf(ctx context.Context, targetSchema, targetName string, wantView bool, kind string, selectFunc func(context.Context, string, string) error) error {
	root := s.tree.GetRoot()
	if root == nil {
		return fmt.Errorf("tree not initialized")
	}

	for _, schemaNode := range root.GetChildren() {
		cleanSchema, _ := s.removeIcons(schemaNode.GetText(), "")
		if cleanSchema != targetSchema {
			continue
		}

		schemaNode.SetExpanded(true)
		openNodeIcon := s.style.IconWithColor(s.style.OpenNode, s.App.GetStyles().Global.SecondaryTextColor)
		schemaNode.SetText(fmt.Sprintf("%s%s", openNodeIcon, cleanSchema))

		for _, leaf := range schemaNode.GetChildren() {
			if s.isViewNode(leaf) != wantView {
				continue
			}
			_, cleanName := s.removeIcons("", leaf.GetText())
			if cleanName == targetName {
				s.tree.SetCurrentNode(leaf)
				if selectFunc != nil {
					return selectFunc(ctx, targetSchema, targetName)
				}
				return nil
			}
		}
		return fmt.Errorf("%s %q not found in schema %q", kind, targetName, targetSchema)
	}

	return fmt.Errorf("schema %q not found", targetSchema)
}
