//go:build integration

package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testDao *Dao

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("visql_test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.WithInitScripts(testutil.SamplePostgresPath()),
		wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60*time.Second),
	)
	if err != nil {
		panic("failed to start postgres container: " + err.Error())
	}
	defer pgContainer.Terminate(ctx) //nolint:errcheck

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic("failed to get connection string: " + err.Error())
	}

	cfg := &config.SQLConfig{
		Driver:  "postgres",
		DSN:     connStr,
		Timeout: 10,
	}
	client := NewClient(cfg)
	if err := client.Connect(); err != nil {
		panic("failed to connect to postgres: " + err.Error())
	}
	defer client.Close()

	testDao = NewDao(client)
	os.Exit(m.Run())
}

// --- Schema browsing ---

func TestListSchemasWithTables_MultipleSchemas(t *testing.T) {
	ctx := context.Background()

	schemas, err := testDao.ListSchemasWithTables(ctx, "")
	require.NoError(t, err)
	require.NotEmpty(t, schemas)

	schemaNames := make([]string, len(schemas))
	for i, s := range schemas {
		schemaNames[i] = s.Schema
	}
	// The postgres sample fixture creates auth, store, and other schemas.
	// At minimum public should exist.
	assert.NotEmpty(t, schemaNames)
}

func TestListSchemasWithTables_WithFilter(t *testing.T) {
	ctx := context.Background()

	schemas, err := testDao.ListSchemasWithTables(ctx, "pub")
	require.NoError(t, err)
	for _, s := range schemas {
		assert.Contains(t, s.Schema, "pub")
	}
}

// --- Table structure ---

func TestGetTableColumns_WithPGTypes(t *testing.T) {
	ctx := context.Background()

	// The sample fixture uses public schema tables.
	// Find any table that exists and check columns.
	schemas, err := testDao.ListSchemasWithTables(ctx, "")
	require.NoError(t, err)
	require.NotEmpty(t, schemas)

	// Use the first table from the first schema.
	require.NotEmpty(t, schemas[0].Tables)
	schema := schemas[0].Schema
	table := schemas[0].Tables[0]

	cols, err := testDao.GetTableColumns(ctx, schema, table)
	require.NoError(t, err)
	require.NotEmpty(t, cols)

	for _, c := range cols {
		assert.NotEmpty(t, c.Name)
		assert.NotEmpty(t, c.DataType)
	}
}

func TestGetTableConstraints(t *testing.T) {
	ctx := context.Background()

	schemas, err := testDao.ListSchemasWithTables(ctx, "")
	require.NoError(t, err)
	require.NotEmpty(t, schemas)
	require.NotEmpty(t, schemas[0].Tables)

	schema := schemas[0].Schema
	table := schemas[0].Tables[0]

	_, err = testDao.GetTableConstraints(ctx, schema, table)
	require.NoError(t, err)
}

func TestGetTableForeignKeys(t *testing.T) {
	ctx := context.Background()

	schemas, err := testDao.ListSchemasWithTables(ctx, "")
	require.NoError(t, err)
	require.NotEmpty(t, schemas)
	require.NotEmpty(t, schemas[0].Tables)

	_, err = testDao.GetTableForeignKeys(ctx, schemas[0].Schema, schemas[0].Tables[0])
	require.NoError(t, err)
}

// --- Row CRUD ---

func TestListRows_BasicPagination(t *testing.T) {
	ctx := context.Background()

	schemas, err := testDao.ListSchemasWithTables(ctx, "")
	require.NoError(t, err)
	require.NotEmpty(t, schemas[0].Tables)

	state := database.NewTableState(schemas[0].Schema, schemas[0].Tables[0])
	state.Limit = 3

	_, rows, err := testDao.ListRows(ctx, state, "", "", nil, nil)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(rows), 3)
}

func TestExecuteQuery_Select(t *testing.T) {
	ctx := context.Background()

	schemas, err := testDao.ListSchemasWithTables(ctx, "")
	require.NoError(t, err)
	require.NotEmpty(t, schemas[0].Tables)

	schema := schemas[0].Schema
	table := schemas[0].Tables[0]

	rows, cols, err := testDao.ExecuteQuery(ctx,
		"SELECT * FROM \""+schema+"\".\""+table+"\" LIMIT 2")
	require.NoError(t, err)
	assert.NotEmpty(t, cols)
	_ = rows
}

func TestExecuteStatement_CreateDrop(t *testing.T) {
	ctx := context.Background()

	affected, err := testDao.ExecuteStatement(ctx,
		`CREATE TABLE IF NOT EXISTS public.pg_test_stmt (id SERIAL PRIMARY KEY)`)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, affected, int64(0))

	_, err = testDao.ExecuteStatement(ctx, `DROP TABLE IF EXISTS public.pg_test_stmt`)
	require.NoError(t, err)
}

func TestExplainQuery_ReturnsOutput(t *testing.T) {
	ctx := context.Background()

	plan, err := testDao.ExplainQuery(ctx, "SELECT 1")
	require.NoError(t, err)
	assert.NotEmpty(t, plan)
}

// --- Indexes ---

func TestCreateIndex_GetIndexes_DropIndex(t *testing.T) {
	ctx := context.Background()

	// Create a temporary table for this test.
	_, err := testDao.ExecuteStatement(ctx,
		`CREATE TABLE IF NOT EXISTS public.idx_test (id SERIAL PRIMARY KEY, name TEXT)`)
	require.NoError(t, err)
	defer testDao.ExecuteStatement(ctx, `DROP TABLE IF EXISTS public.idx_test`) //nolint:errcheck

	def := database.IndexDefinition{
		Name:     "idx_test_name",
		Columns:  []string{"name"},
		IsUnique: false,
	}
	err = testDao.CreateIndex(ctx, "public", "idx_test", def)
	require.NoError(t, err)

	indexes, err := testDao.GetIndexes(ctx, "public", "idx_test")
	require.NoError(t, err)

	var found bool
	for _, idx := range indexes {
		if idx.Name == "idx_test_name" {
			found = true
		}
	}
	assert.True(t, found, "created index should appear in GetIndexes")

	err = testDao.DropIndex(ctx, "public", "idx_test_name")
	require.NoError(t, err)
}

// --- DDL ---

func TestCreateTable_And_DropTable(t *testing.T) {
	ctx := context.Background()

	ddl := `CREATE TABLE public.pg_ddl_test (id SERIAL PRIMARY KEY, label TEXT NOT NULL)`
	err := testDao.CreateTable(ctx, "public", ddl)
	require.NoError(t, err)

	schemas, err := testDao.ListSchemasWithTables(ctx, "")
	require.NoError(t, err)
	var found bool
	for _, s := range schemas {
		if s.Schema == "public" {
			for _, tbl := range s.Tables {
				if tbl == "pg_ddl_test" {
					found = true
				}
			}
		}
	}
	assert.True(t, found, "created table should appear in schema listing")

	err = testDao.DropTable(ctx, "public", "pg_ddl_test")
	require.NoError(t, err)
}

func TestTruncateTable(t *testing.T) {
	ctx := context.Background()

	_, err := testDao.ExecuteStatement(ctx,
		`CREATE TABLE IF NOT EXISTS public.trunc_pg_test (id SERIAL PRIMARY KEY)`)
	require.NoError(t, err)
	defer testDao.ExecuteStatement(ctx, `DROP TABLE IF EXISTS public.trunc_pg_test`) //nolint:errcheck

	_, err = testDao.ExecuteStatement(ctx,
		`INSERT INTO public.trunc_pg_test DEFAULT VALUES`)
	require.NoError(t, err)

	err = testDao.TruncateTable(ctx, "public", "trunc_pg_test")
	require.NoError(t, err)

	state := database.NewTableState("public", "trunc_pg_test")
	state.Limit = 100
	_, rows, err := testDao.ListRows(ctx, state, "", "", nil, nil)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

// --- Server info ---

func TestGetServerInfo_ReturnsVersion(t *testing.T) {
	ctx := context.Background()

	info, err := testDao.GetServerInfo(ctx)
	require.NoError(t, err)
	assert.Contains(t, info.Version, "PostgreSQL")
}

func TestGetActiveSessions_PositiveCount(t *testing.T) {
	ctx := context.Background()

	sessions, err := testDao.GetActiveSessions(ctx)
	require.NoError(t, err)
	assert.Greater(t, sessions, int64(0))
}

// --- Autocomplete ---

func TestGetTableColumnNames(t *testing.T) {
	ctx := context.Background()

	schemas, err := testDao.ListSchemasWithTables(ctx, "")
	require.NoError(t, err)
	require.NotEmpty(t, schemas[0].Tables)

	names, err := testDao.GetTableColumnNames(ctx, schemas[0].Schema, schemas[0].Tables[0])
	require.NoError(t, err)
	assert.NotEmpty(t, names)
}

// --- Driver info ---

func TestCommonDataTypes_NonEmpty(t *testing.T) {
	types := testDao.CommonDataTypes()
	assert.NotEmpty(t, types)
}

func TestDefaultCreateTableDDL_ContainsTableName(t *testing.T) {
	ddl := testDao.DefaultCreateTableDDL("public", "my_pg_table")
	assert.Contains(t, ddl, "my_pg_table")
}

// --- Row CRUD (insert / get / update / delete) ---

func TestInsertRow_GetRow_UpdateRow_DeleteRow(t *testing.T) {
	ctx := context.Background()

	// Use a dedicated table so this test is self-contained.
	_, err := testDao.ExecuteStatement(ctx,
		`CREATE TABLE IF NOT EXISTS public.crud_pg_test (
			id    SERIAL PRIMARY KEY,
			label TEXT  NOT NULL
		)`)
	require.NoError(t, err)
	defer testDao.ExecuteStatement(ctx, `DROP TABLE IF EXISTS public.crud_pg_test`) //nolint:errcheck

	// InsertRow.
	newRow := database.Row{"label": "hello"}
	pk, err := testDao.InsertRow(ctx, "public", "crud_pg_test", newRow)
	require.NoError(t, err)

	// GetRow — retrieve by the PK returned from InsertRow.
	row, err := testDao.GetRow(ctx, "public", "crud_pg_test", pk)
	require.NoError(t, err)
	assert.Equal(t, "hello", row["label"])

	// UpdateRow.
	updated := make(database.Row)
	for k, v := range row {
		updated[k] = v
	}
	updated["label"] = "world"
	err = testDao.UpdateRow(ctx, "public", "crud_pg_test", pk, row, updated)
	require.NoError(t, err)

	// Verify update persisted.
	after, err := testDao.GetRow(ctx, "public", "crud_pg_test", pk)
	require.NoError(t, err)
	assert.Equal(t, "world", after["label"])

	// DeleteRows.
	err = testDao.DeleteRows(ctx, "public", "crud_pg_test", []database.PrimaryKey{pk})
	require.NoError(t, err)

	_, err = testDao.GetRow(ctx, "public", "crud_pg_test", pk)
	assert.Error(t, err, "row should not exist after delete")
}

// --- ListRows with filtering ---

func TestListRows_WithWhere(t *testing.T) {
	ctx := context.Background()

	_, err := testDao.ExecuteStatement(ctx,
		`CREATE TABLE IF NOT EXISTS public.filter_pg_test (
			id     SERIAL PRIMARY KEY,
			status TEXT NOT NULL
		)`)
	require.NoError(t, err)
	defer testDao.ExecuteStatement(ctx, `DROP TABLE IF EXISTS public.filter_pg_test`) //nolint:errcheck

	_, err = testDao.ExecuteStatement(ctx,
		`INSERT INTO public.filter_pg_test (status) VALUES ('active'),('inactive'),('active')`)
	require.NoError(t, err)

	state := database.NewTableState("public", "filter_pg_test")
	state.Limit = 100

	_, rows, err := testDao.ListRows(ctx, state, "status = 'active'", "", nil, nil)
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		assert.Equal(t, "active", r["status"])
	}
}

func TestListRows_WithOrderBy(t *testing.T) {
	ctx := context.Background()

	_, err := testDao.ExecuteStatement(ctx,
		`CREATE TABLE IF NOT EXISTS public.order_pg_test (
			id    SERIAL PRIMARY KEY,
			label TEXT NOT NULL
		)`)
	require.NoError(t, err)
	defer testDao.ExecuteStatement(ctx, `DROP TABLE IF EXISTS public.order_pg_test`) //nolint:errcheck

	_, err = testDao.ExecuteStatement(ctx,
		`INSERT INTO public.order_pg_test (label) VALUES ('charlie'),('alpha'),('bravo')`)
	require.NoError(t, err)

	state := database.NewTableState("public", "order_pg_test")
	state.Limit = 100

	_, rows, err := testDao.ListRows(ctx, state, "", "label ASC", nil, nil)
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for i := 1; i < len(rows); i++ {
		prev := rows[i-1]["label"].(string)
		curr := rows[i]["label"].(string)
		assert.LessOrEqual(t, prev, curr, "rows should be ordered by label ASC")
	}
}

// --- RenameTable ---

func TestRenameTable_And_RenameBack(t *testing.T) {
	ctx := context.Background()

	_, err := testDao.ExecuteStatement(ctx,
		`CREATE TABLE IF NOT EXISTS public.rename_pg_src (id SERIAL PRIMARY KEY)`)
	require.NoError(t, err)
	defer testDao.ExecuteStatement(ctx, `DROP TABLE IF EXISTS public.rename_pg_src`) //nolint:errcheck
	defer testDao.ExecuteStatement(ctx, `DROP TABLE IF EXISTS public.rename_pg_dst`) //nolint:errcheck

	err = testDao.RenameTable(ctx, "public", "rename_pg_src", "rename_pg_dst")
	require.NoError(t, err)

	schemas, err := testDao.ListSchemasWithTables(ctx, "")
	require.NoError(t, err)
	var tables []string
	for _, s := range schemas {
		if s.Schema == "public" {
			tables = s.Tables
		}
	}
	assert.Contains(t, tables, "rename_pg_dst")
	assert.NotContains(t, tables, "rename_pg_src")

	err = testDao.RenameTable(ctx, "public", "rename_pg_dst", "rename_pg_src")
	require.NoError(t, err)
}

// --- Async count callback ---

func TestListRows_CountCallback(t *testing.T) {
	ctx := context.Background()

	schemas, err := testDao.ListSchemasWithTables(ctx, "")
	require.NoError(t, err)
	require.NotEmpty(t, schemas[0].Tables)

	state := database.NewTableState(schemas[0].Schema, schemas[0].Tables[0])
	state.Limit = 10

	callbackCh := make(chan int64, 1)
	_, _, err = testDao.ListRows(ctx, state, "", "", nil, func(n int64) {
		callbackCh <- n
	})
	require.NoError(t, err)

	select {
	case count := <-callbackCh:
		assert.GreaterOrEqual(t, count, int64(0))
	case <-time.After(3 * time.Second):
		t.Fatal("count callback was not invoked within 3s")
	}
}

func TestGetEstimatedRowCount(t *testing.T) {
	ctx := context.Background()

	schemas, err := testDao.ListSchemasWithTables(ctx, "")
	require.NoError(t, err)
	require.NotEmpty(t, schemas[0].Tables)

	count, err := testDao.GetEstimatedRowCount(ctx, schemas[0].Schema, schemas[0].Tables[0])
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(0))
}

func TestRenameColumn(t *testing.T) {
	ctx := context.Background()

	require.NoError(t, testDao.CreateTable(ctx, "public",
		"CREATE TABLE rename_col_pg (id SERIAL PRIMARY KEY, old_name TEXT)"))
	defer testDao.DropTable(ctx, "public", "rename_col_pg") //nolint:errcheck

	err := testDao.RenameColumn(ctx, "public", "rename_col_pg", "old_name", "new_name")
	require.NoError(t, err)

	names, err := testDao.GetTableColumnNames(ctx, "public", "rename_col_pg")
	require.NoError(t, err)
	assert.Contains(t, names, "new_name")
	assert.NotContains(t, names, "old_name")
}

// --- ListQueryRows ---

func TestListQueryRows_WithPagination(t *testing.T) {
	ctx := context.Background()

	schemas, err := testDao.ListSchemasWithTables(ctx, "")
	require.NoError(t, err)
	require.NotEmpty(t, schemas[0].Tables)

	schema := schemas[0].Schema
	table := schemas[0].Tables[0]

	_, rows, cols, err := testDao.ListQueryRows(ctx,
		`SELECT * FROM "`+schema+`"."`+table+`"`, 2, 0, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, cols)
	assert.LessOrEqual(t, len(rows), 2)
}
