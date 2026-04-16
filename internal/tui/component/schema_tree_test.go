package component

import (
	"context"
	"testing"

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

// TestSchemaTree_Render_EmptySchema is a regression test for the bug where
// an empty schema (no tables) didn't highlight correctly on focus.
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

// TestSchemaTree_Filter_MatchesTableName verifies that filtering by a table name
// narrows the displayed tables to only those containing the search text.
func TestSchemaTree_Filter_MatchesTableName(t *testing.T) {
	app, sim := testutil.NewTestApp(t)
	app.SetDriver(newSchemaTreeMock([]database.Schema{
		{Schema: "public", Tables: []string{"users", "user_roles", "orders"}},
	}))

	tree := NewSchemaTree()
	require.NoError(t, tree.Init(app))
	app.SetRoot(tree, true)
	tree.Render()

	// Apply filter — only tables containing "user" should remain.
	tree.filter("user")
	tree.expandAllNodes("", "")
	testutil.DrawAndSync(app, sim)

	assert.True(t, testutil.ScreenContains(sim, "users"), "filtered tree should show 'users'")
	assert.True(t, testutil.ScreenContains(sim, "user_roles"), "filtered tree should show 'user_roles'")
	assert.False(t, testutil.ScreenContains(sim, "orders"), "filtered tree should hide 'orders'")
}

// TestSchemaTree_Filter_SchemaNameMatch verifies that matching the schema name
// returns all tables under that schema.
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

// TestSchemaTree_Filter_NoMatch verifies that a filter with no matches shows
// nothing (no tables rendered).
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

// TestSchemaTree_Filter_ClearRestoresAll verifies that clearFilter() brings
// back all tables after a narrowing filter.
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

// TestSchemaTree_JumpToTable_Success verifies that JumpToTable navigates to the
// correct node and fires the select callback.
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

// TestSchemaTree_JumpToTable_SchemaNotFound verifies an error when the schema
// does not exist.
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

// TestSchemaTree_JumpToTable_TableNotFound verifies an error when the schema
// exists but the table does not.
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
