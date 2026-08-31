package component

import (
	"context"
	"testing"

	"github.com/kopecmaciej/tview"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newSchemaTreeMock returns a MockDriver configured for SchemaTree tests.
// SchemaTree.init() initializes a CreateTableModal which calls CommonDataTypes().
func newSchemaTreeMock(schemas []database.Schema) *testutil.MockDriver {
	m := &testutil.MockDriver{}
	m.On("ListSchemas", context.Background(), "").Return(schemas, nil)
	m.On("CommonDataTypes").Return([]string{"TEXT", "INTEGER", "REAL", "BLOB"})
	return m
}

func TestSchemaTree_Render_ShowsSchemaName(t *testing.T) {
	app, sim := testutil.NewTestApp(t)
	app.SetDriver(newSchemaTreeMock([]database.Schema{
		{Schema: "public", Tables: []string{"users", "orders"}},
	}))

	tree := NewSchemaTree()
	require.NoError(t, tree.Init(app))
	app.SetRoot(tree, true)
	tree.Render()
	testutil.DrawAndSync(app, sim)

	assert.True(t, testutil.ScreenContains(sim, "public"),
		"screen should show schema name\nscreen:\n%v", testutil.ScreenFull(sim))
}

func TestSchemaTree_Render_ShowsTableNodes(t *testing.T) {
	app, sim := testutil.NewTestApp(t)
	app.SetDriver(newSchemaTreeMock([]database.Schema{
		{Schema: "main", Tables: []string{"users", "products"}},
	}))

	tree := NewSchemaTree()
	require.NoError(t, tree.Init(app))
	app.SetRoot(tree, true)
	tree.Render()

	// Expand all nodes to make table names visible.
	tree.expandAllNodes("", "")
	testutil.DrawAndSync(app, sim)

	assert.True(t, testutil.ScreenContains(sim, "users"),
		"screen should show table name after expand\nscreen:\n%v", testutil.ScreenFull(sim))
}

func TestSchemaTree_Render_EmptySchema(t *testing.T) {
	app, sim := testutil.NewTestApp(t)
	app.SetDriver(newSchemaTreeMock([]database.Schema{
		{Schema: "public", Tables: []string{}},
	}))

	tree := NewSchemaTree()
	require.NoError(t, tree.Init(app))
	app.SetRoot(tree, true)
	tree.Render()
	testutil.DrawAndSync(app, sim)

	assert.True(t, testutil.ScreenContains(sim, "public"),
		"empty schema should still appear in the tree\nscreen:\n%v", testutil.ScreenFull(sim))
}

func TestSchemaTree_SetSelectFunc_IsRegistered(t *testing.T) {
	app, _ := testutil.NewTestApp(t)
	app.SetDriver(newSchemaTreeMock([]database.Schema{
		{Schema: "main", Tables: []string{"users"}},
	}))

	tree := NewSchemaTree()
	require.NoError(t, tree.Init(app))
	app.SetRoot(tree, true)
	tree.Render()

	// Verify the root → schema → table node hierarchy was built.
	rootNode := tree.tree.GetRoot()
	require.NotNil(t, rootNode)
	schemaChildren := rootNode.GetChildren()
	require.NotEmpty(t, schemaChildren, "schema node should exist under root")
	tableChildren := schemaChildren[0].GetChildren()
	require.NotEmpty(t, tableChildren, "table node should exist under schema node")

	// Register a select callback and verify it can be called directly.
	called := false
	tree.SetSelectFunc(func(ctx context.Context, schema, table string) error {
		called = true
		return nil
	})
	require.NotNil(t, tree.nodeSelectFunc)

	_ = tree.nodeSelectFunc(context.Background(), "main", "users")
	assert.True(t, called, "nodeSelectFunc should be callable")
}

func TestSchemaTree_Filter_MatchesTableName(t *testing.T) {
	app, sim := testutil.NewTestApp(t)
	app.SetDriver(newSchemaTreeMock([]database.Schema{
		{Schema: "public", Tables: []string{"users", "user_roles", "orders"}},
	}))

	tree := NewSchemaTree()
	require.NoError(t, tree.Init(app))
	app.SetRoot(tree, true)
	tree.Render()

	tree.filter("user")
	tree.expandAllNodes("", "")
	testutil.DrawAndSync(app, sim)

	assert.True(t, testutil.ScreenContains(sim, "users"), "filtered tree should show 'users'")
	assert.True(t, testutil.ScreenContains(sim, "user_roles"), "filtered tree should show 'user_roles'")
	assert.False(t, testutil.ScreenContains(sim, "orders"), "filtered tree should hide 'orders'")
}

func TestSchemaTree_Filter_SchemaNameMatch(t *testing.T) {
	app, sim := testutil.NewTestApp(t)
	app.SetDriver(newSchemaTreeMock([]database.Schema{
		{Schema: "public", Tables: []string{"alpha", "beta"}},
		{Schema: "hidden", Tables: []string{"gamma"}},
	}))

	tree := NewSchemaTree()
	require.NoError(t, tree.Init(app))
	app.SetRoot(tree, true)
	tree.Render()

	// Filtering by schema name "public" should show all tables under it.
	tree.filter("public")
	tree.expandAllNodes("", "")
	testutil.DrawAndSync(app, sim)

	assert.True(t, testutil.ScreenContains(sim, "alpha"), "schema match should include all its tables")
	assert.True(t, testutil.ScreenContains(sim, "beta"), "schema match should include all its tables")
	assert.False(t, testutil.ScreenContains(sim, "gamma"), "non-matching schema should be hidden")
}

func TestSchemaTree_Filter_NoMatch(t *testing.T) {
	app, sim := testutil.NewTestApp(t)
	app.SetDriver(newSchemaTreeMock([]database.Schema{
		{Schema: "public", Tables: []string{"users", "orders"}},
	}))

	tree := NewSchemaTree()
	require.NoError(t, tree.Init(app))
	app.SetRoot(tree, true)
	tree.Render()

	tree.filter("zzz_no_match")
	tree.expandAllNodes("", "")
	testutil.DrawAndSync(app, sim)

	assert.False(t, testutil.ScreenContains(sim, "users"), "non-matching filter should hide all tables")
	assert.False(t, testutil.ScreenContains(sim, "orders"))
}

func TestSchemaTree_Filter_ClearRestoresAll(t *testing.T) {
	app, sim := testutil.NewTestApp(t)
	app.SetDriver(newSchemaTreeMock([]database.Schema{
		{Schema: "public", Tables: []string{"users", "orders"}},
	}))

	tree := NewSchemaTree()
	require.NoError(t, tree.Init(app))
	app.SetRoot(tree, true)
	tree.Render()

	tree.filter("orders")
	tree.clearFilter()
	tree.expandAllNodes("", "")
	testutil.DrawAndSync(app, sim)

	assert.True(t, testutil.ScreenContains(sim, "users"), "clearFilter should restore all tables")
	assert.True(t, testutil.ScreenContains(sim, "orders"))
}

// Regression for github.com/kopecmaciej/vi-sql discussion #77
func TestSchemaTree_Refresh_KeepsActiveFilter(t *testing.T) {
	app, sim := testutil.NewTestApp(t)
	app.SetDriver(newSchemaTreeMock([]database.Schema{
		{Schema: "public", Tables: []string{"users", "orders"}},
	}))

	tree := NewSchemaTree()
	require.NoError(t, tree.Init(app))
	app.SetRoot(tree, true)
	tree.Render()

	tree.filterBar.SetText("order")
	tree.filter("order")
	filteredNode := tree.tree.GetRoot().GetChildren()[0].GetText()

	tree.restoreAfterReload(tree.expandedSchemas(), tree.currentSelection())
	testutil.DrawAndSync(app, sim)

	assert.True(t, testutil.ScreenContains(sim, "orders"), "refresh should keep the filter applied")
	assert.False(t, testutil.ScreenContains(sim, "users"), "refresh must not fall back to the full list")
	assert.Equal(t, filteredNode, tree.tree.GetRoot().GetChildren()[0].GetText(),
		"refresh must render the schema node (expand icon included) the same as the initial filter")
}

func TestSchemaTree_JumpToTable_Success(t *testing.T) {
	app, _ := testutil.NewTestApp(t)
	app.SetDriver(newSchemaTreeMock([]database.Schema{
		{Schema: "public", Tables: []string{"users", "orders"}},
	}))

	tree := NewSchemaTree()
	require.NoError(t, tree.Init(app))
	app.SetRoot(tree, true)
	tree.Render()

	var gotSchema, gotTable string
	tree.SetSelectFunc(func(ctx context.Context, schema, table string) error {
		gotSchema = schema
		gotTable = table
		return nil
	})

	err := tree.JumpToTable(context.Background(), "public", "orders")
	require.NoError(t, err)
	assert.Equal(t, "public", gotSchema)
	assert.Equal(t, "orders", gotTable)
}

func TestSchemaTree_JumpToTable_SchemaNotFound(t *testing.T) {
	app, _ := testutil.NewTestApp(t)
	app.SetDriver(newSchemaTreeMock([]database.Schema{
		{Schema: "public", Tables: []string{"users"}},
	}))

	tree := NewSchemaTree()
	require.NoError(t, tree.Init(app))
	app.SetRoot(tree, true)
	tree.Render()

	err := tree.JumpToTable(context.Background(), "nonexistent", "users")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
}

func TestSchemaTree_JumpToTable_TableNotFound(t *testing.T) {
	app, _ := testutil.NewTestApp(t)
	app.SetDriver(newSchemaTreeMock([]database.Schema{
		{Schema: "public", Tables: []string{"users"}},
	}))

	tree := NewSchemaTree()
	require.NoError(t, tree.Init(app))
	app.SetRoot(tree, true)
	tree.Render()

	err := tree.JumpToTable(context.Background(), "public", "nonexistent_table")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent_table")
}

func TestSchemaTree_Render_MultipleSchemas(t *testing.T) {
	app, sim := testutil.NewTestApp(t)
	app.SetDriver(newSchemaTreeMock([]database.Schema{
		{Schema: "auth", Tables: []string{"users"}},
		{Schema: "store", Tables: []string{"products", "orders"}},
	}))

	tree := NewSchemaTree()
	require.NoError(t, tree.Init(app))
	app.SetRoot(tree, true)
	tree.Render()
	testutil.DrawAndSync(app, sim)

	assert.True(t, testutil.ScreenContains(sim, "auth"),
		"screen should show first schema\nscreen:\n%v", testutil.ScreenFull(sim))
	assert.True(t, testutil.ScreenContains(sim, "store"),
		"screen should show second schema")
}

func TestExtractName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"users", "users"},
		{"", ""},
		{"  spaces  ", "spaces"},
		{"[#387D44]\xef\x82\x94[-:-:-]  v_active_users", "v_active_users"},
		{"[color]icon[-:-:-] name[-:-:-]  actual", "actual"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, extractName(tt.input))
		})
	}
}

func TestSchemaTree_JumpToView(t *testing.T) {
	schemas := []database.Schema{
		{Schema: "public", Tables: []string{"sessions"}, Views: []string{"v_sessions", "v_users"}},
	}

	tests := []struct {
		name        string
		schema      string
		view        string
		wantErr     bool
		errContains string
		wantCalled  bool
	}{
		{"success", "public", "v_sessions", false, "", true},
		{"schema not found", "nonexistent", "v_sessions", true, "nonexistent", false},
		{"view not found", "public", "v_nonexistent", true, "v_nonexistent", false},
		{"skips table node with same name", "public", "sessions", true, "sessions", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, _ := testutil.NewTestApp(t)
			app.SetDriver(newSchemaTreeMock(schemas))
			tree := NewSchemaTree()
			require.NoError(t, tree.Init(app))
			app.SetRoot(tree, true)
			tree.Render()

			called := false
			tree.SetViewSelectFunc(func(ctx context.Context, schema, view string) error {
				called = true
				return nil
			})

			err := tree.JumpToView(context.Background(), tt.schema, tt.view)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.wantCalled, called)
		})
	}
}

func TestLastVisibleTreeNode_Leaf(t *testing.T) {
	node := tview.NewTreeNode("leaf")
	got := lastVisibleTreeNode(node)
	assert.Equal(t, node, got, "leaf node should return itself")
}

func TestLastVisibleTreeNode_Collapsed(t *testing.T) {
	parent := tview.NewTreeNode("parent")
	child := tview.NewTreeNode("child")
	parent.AddChild(child)
	parent.SetExpanded(false)

	got := lastVisibleTreeNode(parent)
	assert.Equal(t, parent, got, "collapsed node must return itself, not its children")
}

func TestLastVisibleTreeNode_SingleLevel(t *testing.T) {
	root := tview.NewTreeNode("root")
	root.SetExpanded(true)
	a := tview.NewTreeNode("a")
	b := tview.NewTreeNode("b")
	c := tview.NewTreeNode("c")
	root.AddChild(a).AddChild(b).AddChild(c)

	got := lastVisibleTreeNode(root)
	assert.Equal(t, c, got, "should return the last direct child when expanded one level")
}

func TestLastVisibleTreeNode_DeepNested(t *testing.T) {
	root := tview.NewTreeNode("root")
	root.SetExpanded(true)
	mid := tview.NewTreeNode("mid")
	mid.SetExpanded(true)
	leaf := tview.NewTreeNode("leaf")
	mid.AddChild(leaf)
	root.AddChild(mid)

	got := lastVisibleTreeNode(root)
	assert.Equal(t, leaf, got, "should descend into expanded children recursively")
}

func TestLastVisibleTreeNode_PartiallyCollapsed(t *testing.T) {
	root := tview.NewTreeNode("root")
	root.SetExpanded(true)
	expanded := tview.NewTreeNode("expanded")
	expanded.SetExpanded(true)
	collapsed := tview.NewTreeNode("collapsed")
	collapsed.SetExpanded(false)

	deepChild := tview.NewTreeNode("deep")
	expanded.AddChild(deepChild)
	hiddenChild := tview.NewTreeNode("hidden")
	collapsed.AddChild(hiddenChild)

	root.AddChild(expanded).AddChild(collapsed)

	// root is expanded; last child is "collapsed" (not expanded) → return "collapsed"
	got := lastVisibleTreeNode(root)
	assert.Equal(t, collapsed, got,
		"last child is collapsed — should return it, not descend into its hidden children")
}
