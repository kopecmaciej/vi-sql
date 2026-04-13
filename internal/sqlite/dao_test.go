package sqlite

import (
	"context"
	"os"
	"testing"

	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestDao creates a fresh in-memory SQLite Dao and loads the sample fixture.
// Each call returns an independent database — safe for parallel and mutating tests.
func newTestDao(t *testing.T) *Dao {
	t.Helper()
	cfg := &config.SQLConfig{Driver: "sqlite", DSN: ":memory:"}
	client := NewClient(cfg)
	ctx := context.Background()
	require.NoError(t, client.Connect(ctx))
	t.Cleanup(func() { client.Close() })

	sql, err := os.ReadFile(testutil.SampleSQLitePath())
	require.NoError(t, err, "read SQLite fixture")
	_, err = client.DB.ExecContext(ctx, string(sql))
	require.NoError(t, err, "load SQLite fixture")

	return NewDao(client)
}

// --- Schema browsing ---

func TestListSchemasWithTables_ReturnsMainSchema(t *testing.T) {
	t.Parallel()
	dao := newTestDao(t)
	ctx := context.Background()

	schemas, err := dao.ListSchemasWithTables(ctx, "")
	require.NoError(t, err)
	require.Len(t, schemas, 1)
	assert.Equal(t, "main", schemas[0].Schema)
	assert.Contains(t, schemas[0].Tables, "users")
	assert.Contains(t, schemas[0].Tables, "orders")
	assert.Contains(t, schemas[0].Tables, "products")
}

func TestListSchemasWithTables_WithFilter(t *testing.T) {
	t.Parallel()
	dao := newTestDao(t)
	ctx := context.Background()

	schemas, err := dao.ListSchemasWithTables(ctx, "user")
	require.NoError(t, err)
	require.Len(t, schemas, 1)
	for _, table := range schemas[0].Tables {
		assert.Contains(t, table, "user")
	}
}

func TestListSchemasWithTables_FilterNoMatch(t *testing.T) {
	t.Parallel()
	dao := newTestDao(t)
	ctx := context.Background()

	schemas, err := dao.ListSchemasWithTables(ctx, "nonexistent_xyz")
	require.NoError(t, err)
	require.Len(t, schemas, 1)
	assert.Empty(t, schemas[0].Tables)
}

// --- Table structure ---

func TestGetTableColumns_Users(t *testing.T) {
	t.Parallel()
	dao := newTestDao(t)
	ctx := context.Background()

	cols, err := dao.GetTableColumns(ctx, "main", "users")
	require.NoError(t, err)
	require.NotEmpty(t, cols)

	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Name
	}
	assert.Contains(t, names, "id")
	assert.Contains(t, names, "email")

	var idCol database.ColumnInfo
	for _, c := range cols {
		if c.Name == "id" {
			idCol = c
			break
		}
	}
	assert.True(t, idCol.IsPK)
}

func TestGetTableConstraints_UsersPrimaryKey(t *testing.T) {
	t.Parallel()
	dao := newTestDao(t)
	ctx := context.Background()

	constraints, err := dao.GetTableConstraints(ctx, "main", "users")
	require.NoError(t, err)

	var found bool
	for _, c := range constraints {
		if c.Type == "PRIMARY KEY" {
			found = true
			assert.Contains(t, c.Columns, "id")
		}
	}
	assert.True(t, found, "expected a PRIMARY KEY constraint on users")
}

func TestGetTableForeignKeys_OrderItems(t *testing.T) {
	t.Parallel()
	dao := newTestDao(t)
	ctx := context.Background()

	fks, err := dao.GetTableForeignKeys(ctx, "main", "order_items")
	require.NoError(t, err)
	require.NotEmpty(t, fks, "order_items should have foreign keys")

	tables := make([]string, len(fks))
	for i, fk := range fks {
		tables[i] = fk.ReferencedTable
	}
	assert.Contains(t, tables, "orders")
	assert.Contains(t, tables, "products")
}

// --- Row CRUD ---

func TestListRows_BasicPagination(t *testing.T) {
	t.Parallel()
	dao := newTestDao(t)
	ctx := context.Background()

	state := database.NewTableState("main", "users")
	state.Limit = 3
	state.Offset = 0

	_, rows, err := dao.ListRows(ctx, state, "", "", nil, nil)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(rows), 3)
	assert.NotEmpty(t, rows)
}

func TestListRows_WithWhere(t *testing.T) {
	t.Parallel()
	dao := newTestDao(t)
	ctx := context.Background()

	state := database.NewTableState("main", "users")
	state.Limit = 100
	state.Offset = 0

	_, rows, err := dao.ListRows(ctx, state, "status = 'active'", "", nil, nil)
	require.NoError(t, err)
	for _, row := range rows {
		assert.Equal(t, "active", row["status"])
	}
}

func TestListRows_WithOrderBy(t *testing.T) {
	t.Parallel()
	dao := newTestDao(t)
	ctx := context.Background()

	state := database.NewTableState("main", "users")
	state.Limit = 100
	state.Offset = 0

	_, rows, err := dao.ListRows(ctx, state, "", "email ASC", nil, nil)
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for i := 1; i < len(rows); i++ {
		prev := rows[i-1]["email"].(string)
		curr := rows[i]["email"].(string)
		assert.LessOrEqual(t, prev, curr, "rows should be ordered by email ASC")
	}
}

func TestInsertRow_GetRow_DeleteRow(t *testing.T) {
	dao := newTestDao(t) // not parallel — modifies shared-nothing DB
	ctx := context.Background()

	const insertedID = "test-uuid-001"
	newRow := database.Row{
		"id":            insertedID,
		"email":         "test@example.com",
		"password_hash": "hash",
		"full_name":     "Test User",
		"status":        "active",
	}

	_, err := dao.InsertRow(ctx, "main", "users", newRow)
	require.NoError(t, err)

	// SQLite's InsertRow returns lastInsertId (rowid), not the text PK.
	// Use the known PK value to retrieve the row.
	pk := database.PrimaryKey{Columns: map[string]any{"id": insertedID}}
	row, err := dao.GetRow(ctx, "main", "users", pk)
	require.NoError(t, err)
	assert.Equal(t, "test@example.com", row["email"])

	err = dao.DeleteRows(ctx, "main", "users", []database.PrimaryKey{pk})
	require.NoError(t, err)

	_, err = dao.GetRow(ctx, "main", "users", pk)
	assert.Error(t, err, "row should not exist after delete")
}

func TestUpdateRow_ChangedField(t *testing.T) {
	dao := newTestDao(t)
	ctx := context.Background()

	// Get an existing row first.
	state := database.NewTableState("main", "users")
	state.Limit = 1
	_, rows, err := dao.ListRows(ctx, state, "", "", nil, nil)
	require.NoError(t, err)
	require.NotEmpty(t, rows)

	original := rows[0]
	pk := database.PrimaryKey{Columns: map[string]any{"id": original["id"]}}

	updated := make(database.Row)
	for k, v := range original {
		updated[k] = v
	}
	updated["full_name"] = "Updated Name"

	err = dao.UpdateRow(ctx, "main", "users", pk, original, updated)
	require.NoError(t, err)

	row, err := dao.GetRow(ctx, "main", "users", pk)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", row["full_name"])
}

func TestDeleteRows_NonExistentPK(t *testing.T) {
	t.Parallel()
	dao := newTestDao(t)
	ctx := context.Background()

	pk := database.PrimaryKey{Columns: map[string]any{"id": "does-not-exist"}}
	err := dao.DeleteRows(ctx, "main", "users", []database.PrimaryKey{pk})
	assert.NoError(t, err, "deleting non-existent row should not error")
}

// --- Raw SQL ---

func TestExecuteQuery_Select(t *testing.T) {
	t.Parallel()
	dao := newTestDao(t)
	ctx := context.Background()

	rows, cols, err := dao.ExecuteQuery(ctx, "SELECT id, email FROM users LIMIT 5")
	require.NoError(t, err)
	require.NotEmpty(t, cols)
	assert.Equal(t, "id", cols[0].Name)
	assert.Equal(t, "email", cols[1].Name)
	assert.NotEmpty(t, rows)
}

func TestExecuteStatement_Insert(t *testing.T) {
	dao := newTestDao(t)
	ctx := context.Background()

	affected, err := dao.ExecuteStatement(ctx,
		`INSERT INTO users (id, email, password_hash, full_name, status) VALUES ('stmt-test-1', 'stmt@test.com', 'hash', 'Stmt User', 'active')`)
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)
}

func TestExplainQuery_ReturnsText(t *testing.T) {
	t.Parallel()
	dao := newTestDao(t)
	ctx := context.Background()

	plan, err := dao.ExplainQuery(ctx, "SELECT * FROM users WHERE id = 'x'")
	require.NoError(t, err)
	assert.NotEmpty(t, plan)
}

func TestListQueryRows_WithPagination(t *testing.T) {
	t.Parallel()
	dao := newTestDao(t)
	ctx := context.Background()

	_, rows, cols, err := dao.ListQueryRows(ctx, "SELECT id, email FROM users", 2, 0, nil)
	require.NoError(t, err)
	require.NotEmpty(t, cols)
	assert.LessOrEqual(t, len(rows), 2)
}

// --- DDL ---

func TestCreateTable_And_DropTable(t *testing.T) {
	dao := newTestDao(t)
	ctx := context.Background()

	ddl := `CREATE TABLE test_ddl (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`
	err := dao.CreateTable(ctx, "main", ddl)
	require.NoError(t, err)

	schemas, err := dao.ListSchemasWithTables(ctx, "")
	require.NoError(t, err)
	assert.Contains(t, schemas[0].Tables, "test_ddl")

	err = dao.DropTable(ctx, "main", "test_ddl")
	require.NoError(t, err)

	schemas, err = dao.ListSchemasWithTables(ctx, "")
	require.NoError(t, err)
	assert.NotContains(t, schemas[0].Tables, "test_ddl")
}

func TestRenameTable_And_RenameBack(t *testing.T) {
	dao := newTestDao(t)
	ctx := context.Background()

	// Create a table to rename so we don't break other tests' fixture tables.
	require.NoError(t, dao.CreateTable(ctx, "main",
		"CREATE TABLE rename_src (id INTEGER PRIMARY KEY)"))

	err := dao.RenameTable(ctx, "main", "rename_src", "rename_dst")
	require.NoError(t, err)

	schemas, _ := dao.ListSchemasWithTables(ctx, "")
	assert.Contains(t, schemas[0].Tables, "rename_dst")
	assert.NotContains(t, schemas[0].Tables, "rename_src")

	err = dao.RenameTable(ctx, "main", "rename_dst", "rename_src")
	require.NoError(t, err)
}

func TestTruncateTable(t *testing.T) {
	dao := newTestDao(t)
	ctx := context.Background()

	// Create and populate a disposable table.
	require.NoError(t, dao.CreateTable(ctx, "main",
		"CREATE TABLE trunc_test (id INTEGER PRIMARY KEY)"))
	_, err := dao.ExecuteStatement(ctx,
		"INSERT INTO trunc_test VALUES (1), (2), (3)")
	require.NoError(t, err)

	err = dao.TruncateTable(ctx, "main", "trunc_test")
	require.NoError(t, err)

	state := database.NewTableState("main", "trunc_test")
	state.Limit = 100
	_, rows, err := dao.ListRows(ctx, state, "", "", nil, nil)
	require.NoError(t, err)
	assert.Empty(t, rows, "table should be empty after TRUNCATE")
}

func TestRenameColumn(t *testing.T) {
	dao := newTestDao(t)
	ctx := context.Background()

	require.NoError(t, dao.CreateTable(ctx, "main",
		"CREATE TABLE rename_col_test (id INTEGER PRIMARY KEY, old_name TEXT)"))

	err := dao.RenameColumn(ctx, "main", "rename_col_test", "old_name", "new_name")
	require.NoError(t, err)

	names, err := dao.GetTableColumnNames(ctx, "main", "rename_col_test")
	require.NoError(t, err)
	assert.Contains(t, names, "new_name")
	assert.NotContains(t, names, "old_name")
}

// --- Indexes ---

func TestCreateIndex_GetIndexes_DropIndex(t *testing.T) {
	dao := newTestDao(t)
	ctx := context.Background()

	def := database.IndexDefinition{
		Name:     "idx_users_full_name",
		Columns:  []string{"full_name"},
		IsUnique: false,
	}
	err := dao.CreateIndex(ctx, "main", "users", def)
	require.NoError(t, err)

	indexes, err := dao.GetIndexes(ctx, "main", "users")
	require.NoError(t, err)
	var found bool
	for _, idx := range indexes {
		if idx.Name == "idx_users_full_name" {
			found = true
		}
	}
	assert.True(t, found, "newly created index should appear in GetIndexes")

	err = dao.DropIndex(ctx, "main", "idx_users_full_name")
	require.NoError(t, err)

	indexes, err = dao.GetIndexes(ctx, "main", "users")
	require.NoError(t, err)
	for _, idx := range indexes {
		assert.NotEqual(t, "idx_users_full_name", idx.Name, "index should be gone after drop")
	}
}

// --- Autocomplete ---

func TestGetTableColumnNames(t *testing.T) {
	t.Parallel()
	dao := newTestDao(t)
	ctx := context.Background()

	names, err := dao.GetTableColumnNames(ctx, "main", "users")
	require.NoError(t, err)
	assert.Contains(t, names, "id")
	assert.Contains(t, names, "email")
}

// --- Server info ---

func TestGetServerInfo(t *testing.T) {
	t.Parallel()
	dao := newTestDao(t)
	ctx := context.Background()

	info, err := dao.GetServerInfo(ctx)
	require.NoError(t, err)
	assert.Contains(t, info.Version, "SQLite")
}

func TestGetActiveSessions_ReturnsZero(t *testing.T) {
	t.Parallel()
	dao := newTestDao(t)
	ctx := context.Background()

	sessions, err := dao.GetActiveSessions(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), sessions)
}

// --- Driver info ---

func TestCommonDataTypes_NonEmpty(t *testing.T) {
	t.Parallel()
	dao := newTestDao(t)

	types := dao.CommonDataTypes()
	assert.NotEmpty(t, types)
}

func TestDefaultCreateTableDDL_ContainsTableName(t *testing.T) {
	t.Parallel()
	dao := newTestDao(t)

	ddl := dao.DefaultCreateTableDDL("main", "my_table")
	assert.Contains(t, ddl, "my_table")
}
