//go:build integration

package oracle

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	oraclePass = "test"
	testSchema = "AUTH"
)

var testDao *Dao

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "gvenzl/oracle-xe:21-slim",
			ExposedPorts: []string{"1521/tcp"},
			Env:          map[string]string{"ORACLE_PASSWORD": oraclePass},
			WaitingFor: wait.ForLog("DATABASE IS READY TO USE").
				WithStartupTimeout(5 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		panic("failed to start oracle container: " + err.Error())
	}
	defer container.Terminate(ctx) //nolint:errcheck

	host, err := container.Host(ctx)
	if err != nil {
		panic("failed to get container host: " + err.Error())
	}
	port, err := container.MappedPort(ctx, "1521")
	if err != nil {
		panic("failed to get container port: " + err.Error())
	}

	sysDSN := fmt.Sprintf("oracle://system:%s@%s:%s/XEPDB1", oraclePass, host, port.Port())
	sysCfg := &config.SQLConfig{Driver: "oracle", DSN: sysDSN, Timeout: 60}
	sysClient := NewClient(sysCfg)
	if err := sysClient.Connect(ctx); err != nil {
		panic("failed to connect as SYSTEM: " + err.Error())
	}
	for _, stmt := range []string{
		`CREATE USER auth IDENTIFIED BY auth`,
		`GRANT DBA TO auth`,
	} {
		if _, err := sysClient.DB.ExecContext(ctx, stmt); err != nil {
			panic(fmt.Sprintf("grant failed %q: %v", stmt, err))
		}
	}
	sysClient.Close()

	authDSN := fmt.Sprintf("oracle://auth:auth@%s:%s/XEPDB1", host, port.Port())
	authCfg := &config.SQLConfig{Driver: "oracle", DSN: authDSN, Timeout: 60}
	client := NewClient(authCfg)
	if err := client.Connect(ctx); err != nil {
		panic("failed to connect as AUTH: " + err.Error())
	}
	defer client.Close()

	if err := loadSampleSQL(ctx, client, testutil.SampleOraclePath()); err != nil {
		panic("failed to load sample SQL: " + err.Error())
	}

	testDao = NewDao(client)
	os.Exit(m.Run())
}

// loadSampleSQL splits on ";" and executes each non-empty statement.
// Comment lines (--) are stripped from each segment before execution so that
// a leading comment block doesn't cause a segment containing real SQL to be skipped.
func loadSampleSQL(ctx context.Context, client *Client, sqlPath string) error {
	content, err := os.ReadFile(sqlPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", sqlPath, err)
	}
	for i, raw := range strings.Split(string(content), ";") {
		var kept []string
		for _, line := range strings.Split(raw, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "--") {
				kept = append(kept, line)
			}
		}
		stmt := strings.TrimSpace(strings.Join(kept, "\n"))
		if stmt == "" {
			continue
		}
		if _, err := client.DB.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("statement %d: %w\n--- stmt ---\n%.200s", i, err, stmt)
		}
	}
	return nil
}

// --- Schema browsing ---

func TestListSchemas(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		filter string
		setup  func(t *testing.T)
		check  func(t *testing.T, schemas []database.Schema)
	}{
		{
			name:   "returns auth schema with tables",
			filter: "",
			check: func(t *testing.T, schemas []database.Schema) {
				var found *database.Schema
				for i := range schemas {
					if schemas[i].Schema == testSchema {
						found = &schemas[i]
					}
				}
				require.NotNil(t, found, "AUTH schema should be present")
				assert.Contains(t, found.Tables, "USERS")
				assert.Contains(t, found.Tables, "ROLES")
				assert.Contains(t, found.Tables, "USER_ROLES")
			},
		},
		{
			name:   "filter by name",
			filter: "auth",
			check: func(t *testing.T, schemas []database.Schema) {
				require.NotEmpty(t, schemas)
				for _, s := range schemas {
					assert.Contains(t, strings.ToUpper(s.Schema), "AUTH")
				}
			},
		},
		{
			name:   "includes views",
			filter: "auth",
			setup: func(t *testing.T) {
				_, err := testDao.ExecuteStatement(ctx,
					`CREATE OR REPLACE VIEW v_test_users AS SELECT id, email FROM users`)
				require.NoError(t, err)
			},
			check: func(t *testing.T, schemas []database.Schema) {
				require.Len(t, schemas, 1)
				assert.Contains(t, schemas[0].Views, "V_TEST_USERS")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup(t)
			}
			schemas, err := testDao.ListSchemas(ctx, tt.filter)
			require.NoError(t, err)
			tt.check(t, schemas)
		})
	}
}

func TestGetViewDDL_ReturnsDefinition(t *testing.T) {
	ctx := context.Background()

	_, err := testDao.ExecuteStatement(ctx,
		`CREATE OR REPLACE VIEW v_ddl_test AS SELECT id, email FROM users`)
	require.NoError(t, err)

	ddl, err := testDao.GetViewDDL(ctx, testSchema, "V_DDL_TEST")
	require.NoError(t, err)
	assert.NotEmpty(t, ddl)
	assert.Contains(t, strings.ToUpper(ddl), "USERS")
}

// --- Table structure ---

func TestGetTableColumns_WithOracleTypes(t *testing.T) {
	ctx := context.Background()

	cols, err := testDao.GetTableColumns(ctx, testSchema, "USERS")
	require.NoError(t, err)
	require.NotEmpty(t, cols)

	for _, c := range cols {
		assert.NotEmpty(t, c.Name)
		assert.NotEmpty(t, c.DataType)
	}
}

func TestGetTableColumns_IdentityIsAutoGenerated(t *testing.T) {
	ctx := context.Background()

	require.NoError(t, testDao.CreateTable(ctx, testSchema,
		`CREATE TABLE identity_test (
			id    NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			label VARCHAR2(255) NOT NULL
		)`))
	defer testDao.DropTable(ctx, testSchema, "IDENTITY_TEST") //nolint:errcheck

	cols, err := testDao.GetTableColumns(ctx, testSchema, "IDENTITY_TEST")
	require.NoError(t, err)

	var idCol, labelCol *database.ColumnInfo
	for i := range cols {
		switch cols[i].Name {
		case "ID":
			idCol = &cols[i]
		case "LABEL":
			labelCol = &cols[i]
		}
	}
	require.NotNil(t, idCol)
	require.NotNil(t, labelCol)
	assert.True(t, idCol.IsAutoGenerated, "IDENTITY column must be marked IsAutoGenerated")
	assert.False(t, labelCol.IsAutoGenerated, "plain column must not be marked IsAutoGenerated")
}

func TestGetTableConstraints(t *testing.T) {
	ctx := context.Background()

	constraints, err := testDao.GetTableConstraints(ctx, testSchema, "USERS")
	require.NoError(t, err)
	require.NotEmpty(t, constraints)
}

func TestGetTableForeignKeys(t *testing.T) {
	ctx := context.Background()

	fks, err := testDao.GetTableForeignKeys(ctx, testSchema, "USER_ROLES")
	require.NoError(t, err)
	require.NotEmpty(t, fks)
}

// --- Row CRUD ---

func TestListRows_BasicPagination(t *testing.T) {
	ctx := context.Background()

	state := database.NewTableState(testSchema, "USERS")
	state.BatchSize = 3

	_, rows, err := testDao.FetchTableRows(ctx, state, "", "")
	require.NoError(t, err)
	assert.LessOrEqual(t, len(rows), 3)
}

func TestExecuteQuery_Select(t *testing.T) {
	ctx := context.Background()

	rows, cols, err := testDao.ExecuteQuery(ctx,
		`SELECT * FROM "AUTH"."USERS" FETCH FIRST 2 ROWS ONLY`)
	require.NoError(t, err)
	assert.NotEmpty(t, cols)
	_ = rows
}

func TestExecuteStatement_CreateDrop(t *testing.T) {
	ctx := context.Background()

	affected, err := testDao.ExecuteStatement(ctx,
		`CREATE TABLE stmt_test (id NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY)`)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, affected, int64(0))

	_, err = testDao.ExecuteStatement(ctx, `DROP TABLE stmt_test`)
	require.NoError(t, err)
}

func TestExplainPlan_ReturnsOutput(t *testing.T) {
	ctx := context.Background()

	plan, err := testDao.ExplainPlan(ctx, `SELECT * FROM "AUTH"."USERS"`)
	require.NoError(t, err)
	assert.NotEmpty(t, plan)
}

// --- Indexes ---

func TestCreateIndex_GetIndexes_DropIndex(t *testing.T) {
	ctx := context.Background()

	require.NoError(t, testDao.CreateTable(ctx, testSchema,
		`CREATE TABLE idx_ora_test (id NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY, name VARCHAR2(255))`))
	defer testDao.DropTable(ctx, testSchema, "IDX_ORA_TEST") //nolint:errcheck

	def := database.IndexDefinition{
		Name:     "IDX_ORA_TEST_NAME",
		Columns:  []string{"NAME"},
		IsUnique: false,
	}
	err := testDao.CreateIndex(ctx, testSchema, "IDX_ORA_TEST", def)
	require.NoError(t, err)

	indexes, err := testDao.GetIndexes(ctx, testSchema, "IDX_ORA_TEST")
	require.NoError(t, err)

	var found bool
	for _, idx := range indexes {
		if idx.Name == "IDX_ORA_TEST_NAME" {
			found = true
		}
	}
	assert.True(t, found, "created index should appear in GetIndexes")

	err = testDao.DropIndex(ctx, testSchema, "IDX_ORA_TEST_NAME")
	require.NoError(t, err)
}

// --- DDL ---

func TestCreateTable_And_DropTable(t *testing.T) {
	ctx := context.Background()

	err := testDao.CreateTable(ctx, testSchema,
		`CREATE TABLE ddl_ora_test (id NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY, label VARCHAR2(255) NOT NULL)`)
	require.NoError(t, err)

	schemas, err := testDao.ListSchemas(ctx, "auth")
	require.NoError(t, err)
	var found bool
	for _, s := range schemas {
		if s.Schema == testSchema {
			for _, tbl := range s.Tables {
				if tbl == "DDL_ORA_TEST" {
					found = true
				}
			}
		}
	}
	assert.True(t, found, "created table should appear in schema listing")

	err = testDao.DropTable(ctx, testSchema, "DDL_ORA_TEST")
	require.NoError(t, err)
}

func TestTruncateTable(t *testing.T) {
	ctx := context.Background()

	require.NoError(t, testDao.CreateTable(ctx, testSchema,
		`CREATE TABLE trunc_ora_test (id NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY)`))
	defer testDao.DropTable(ctx, testSchema, "TRUNC_ORA_TEST") //nolint:errcheck

	_, err := testDao.ExecuteStatement(ctx, `INSERT INTO trunc_ora_test (id) VALUES (DEFAULT)`)
	require.NoError(t, err)

	err = testDao.TruncateTable(ctx, testSchema, "TRUNC_ORA_TEST")
	require.NoError(t, err)

	state := database.NewTableState(testSchema, "TRUNC_ORA_TEST")
	state.BatchSize = 100
	_, rows, err := testDao.FetchTableRows(ctx, state, "", "")
	require.NoError(t, err)
	assert.Empty(t, rows)
}

// --- Server info ---

func TestGetServerInfo_ReturnsVersion(t *testing.T) {
	ctx := context.Background()

	info, err := testDao.GetServerInfo(ctx)
	require.NoError(t, err)
	assert.Contains(t, info.Version, "Oracle")
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

	names, err := testDao.GetTableColumnNames(ctx, testSchema, "USERS")
	require.NoError(t, err)
	assert.NotEmpty(t, names)
}

// --- Driver info ---

func TestCommonDataTypes_NonEmpty(t *testing.T) {
	types := testDao.CommonDataTypes()
	assert.NotEmpty(t, types)
}

func TestDefaultCreateTableDDL_ContainsTableName(t *testing.T) {
	ddl := testDao.DefaultCreateTableDDL(testSchema, "MY_ORA_TABLE")
	assert.Contains(t, ddl, "MY_ORA_TABLE")
}

// --- Row CRUD (insert / update / delete) ---

func TestInsertRow_UpdateRow_DeleteRow(t *testing.T) {
	ctx := context.Background()

	require.NoError(t, testDao.CreateTable(ctx, testSchema,
		`CREATE TABLE crud_ora_test (
			id    NUMBER          GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
			label VARCHAR2(255)   NOT NULL
		)`))
	defer testDao.DropTable(ctx, testSchema, "CRUD_ORA_TEST") //nolint:errcheck

	newRow := database.Row{"LABEL": "hello"}
	pk, err := testDao.InsertRow(ctx, testSchema, "CRUD_ORA_TEST", newRow)
	require.NoError(t, err)

	rows, _, err := testDao.ExecuteQuery(ctx,
		fmt.Sprintf(`SELECT LABEL FROM "AUTH"."CRUD_ORA_TEST" WHERE ID = %v`, pk.Columns["ID"]))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	original := rows[0]
	assert.Equal(t, "hello", original["LABEL"])

	updated := make(database.Row)
	for k, v := range original {
		updated[k] = v
	}
	updated["LABEL"] = "world"
	updated["ID"] = pk.Columns["ID"]
	err = testDao.UpdateRow(ctx, testSchema, "CRUD_ORA_TEST", pk, original, updated)
	require.NoError(t, err)

	rows, _, err = testDao.ExecuteQuery(ctx,
		fmt.Sprintf(`SELECT LABEL FROM "AUTH"."CRUD_ORA_TEST" WHERE ID = %v`, pk.Columns["ID"]))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "world", rows[0]["LABEL"])

	err = testDao.DeleteRows(ctx, testSchema, "CRUD_ORA_TEST", []database.PrimaryKey{pk})
	require.NoError(t, err)

	rows, _, err = testDao.ExecuteQuery(ctx,
		fmt.Sprintf(`SELECT ID FROM "AUTH"."CRUD_ORA_TEST" WHERE ID = %v`, pk.Columns["ID"]))
	require.NoError(t, err)
	assert.Empty(t, rows, "row should not exist after delete")
}

// --- FetchTableRows with filtering ---

func TestListRows_WithWhere(t *testing.T) {
	ctx := context.Background()

	require.NoError(t, testDao.CreateTable(ctx, testSchema,
		`CREATE TABLE filter_ora_test (id NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY, status VARCHAR2(50) NOT NULL)`))
	defer testDao.DropTable(ctx, testSchema, "FILTER_ORA_TEST") //nolint:errcheck

	for _, status := range []string{"active", "inactive", "active"} {
		_, err := testDao.ExecuteStatement(ctx,
			fmt.Sprintf(`INSERT INTO filter_ora_test (status) VALUES ('%s')`, status))
		require.NoError(t, err)
	}

	state := database.NewTableState(testSchema, "FILTER_ORA_TEST")
	state.BatchSize = 100

	_, rows, err := testDao.FetchTableRows(ctx, state, "STATUS = 'active'", "")
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		assert.Equal(t, "active", r["STATUS"])
	}
}

func TestListRows_WithOrderBy(t *testing.T) {
	ctx := context.Background()

	require.NoError(t, testDao.CreateTable(ctx, testSchema,
		`CREATE TABLE order_ora_test (id NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY, label VARCHAR2(255) NOT NULL)`))
	defer testDao.DropTable(ctx, testSchema, "ORDER_ORA_TEST") //nolint:errcheck

	for _, label := range []string{"charlie", "alpha", "bravo"} {
		_, err := testDao.ExecuteStatement(ctx,
			fmt.Sprintf(`INSERT INTO order_ora_test (label) VALUES ('%s')`, label))
		require.NoError(t, err)
	}

	state := database.NewTableState(testSchema, "ORDER_ORA_TEST")
	state.BatchSize = 100

	_, rows, err := testDao.FetchTableRows(ctx, state, "", "LABEL ASC")
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	for i := 1; i < len(rows); i++ {
		prev := rows[i-1]["LABEL"].(string)
		curr := rows[i]["LABEL"].(string)
		assert.LessOrEqual(t, prev, curr, "rows should be ordered by LABEL ASC")
	}
}

// --- RenameTable ---

func TestRenameTable_And_RenameBack(t *testing.T) {
	ctx := context.Background()

	require.NoError(t, testDao.CreateTable(ctx, testSchema,
		`CREATE TABLE rename_ora_src (id NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY)`))
	defer testDao.DropTable(ctx, testSchema, "RENAME_ORA_SRC") //nolint:errcheck
	defer testDao.DropTable(ctx, testSchema, "RENAME_ORA_DST") //nolint:errcheck

	err := testDao.RenameTable(ctx, testSchema, "RENAME_ORA_SRC", "RENAME_ORA_DST")
	require.NoError(t, err)

	schemas, err := testDao.ListSchemas(ctx, "auth")
	require.NoError(t, err)
	var tables []string
	for _, s := range schemas {
		if s.Schema == testSchema {
			tables = s.Tables
		}
	}
	assert.Contains(t, tables, "RENAME_ORA_DST")
	assert.NotContains(t, tables, "RENAME_ORA_SRC")

	err = testDao.RenameTable(ctx, testSchema, "RENAME_ORA_DST", "RENAME_ORA_SRC")
	require.NoError(t, err)
}

func TestGetEstimatedRowCount(t *testing.T) {
	ctx := context.Background()

	count, _, err := testDao.GetEstimatedRowCount(ctx, testSchema, "USERS")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(0))
}

func TestRenameColumn(t *testing.T) {
	ctx := context.Background()

	require.NoError(t, testDao.CreateTable(ctx, testSchema,
		`CREATE TABLE rename_col_ora (id NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY, old_name VARCHAR2(255))`))
	defer testDao.DropTable(ctx, testSchema, "RENAME_COL_ORA") //nolint:errcheck

	err := testDao.RenameColumn(ctx, testSchema, "RENAME_COL_ORA", "OLD_NAME", "NEW_NAME")
	require.NoError(t, err)

	names, err := testDao.GetTableColumnNames(ctx, testSchema, "RENAME_COL_ORA")
	require.NoError(t, err)
	assert.Contains(t, names, "NEW_NAME")
	assert.NotContains(t, names, "OLD_NAME")
}

// --- FetchQueryRows ---

func TestListQueryRows_WithPagination(t *testing.T) {
	ctx := context.Background()

	_, rows, cols, err := testDao.FetchQueryRows(ctx,
		`SELECT * FROM "AUTH"."USERS"`, 2, 0)
	require.NoError(t, err)
	assert.NotEmpty(t, cols)
	assert.LessOrEqual(t, len(rows), 2)
}

func TestListQueryRows_NoLimit_Paginates(t *testing.T) {
	ctx := context.Background()

	const batch = 3
	_, rows, _, err := testDao.FetchQueryRows(ctx, `SELECT * FROM "AUTH"."USERS"`, batch, 0)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(rows), batch)
	assert.NotEmpty(t, rows)

	_, rows2, _, err := testDao.FetchQueryRows(ctx, `SELECT * FROM "AUTH"."USERS"`, batch, batch)
	require.NoError(t, err)
	assert.Len(t, rows2, batch, "second page should also have a full batch")
}

func TestListQueryRows_WithUserLimit_Paginates(t *testing.T) {
	ctx := context.Background()

	const batch = 2

	_, page1, _, err := testDao.FetchQueryRows(ctx,
		`SELECT * FROM "AUTH"."USERS" FETCH FIRST 5 ROWS ONLY`, batch, 0)
	require.NoError(t, err)
	assert.Len(t, page1, batch, "first page should return batch rows")

	_, page2, _, err := testDao.FetchQueryRows(ctx,
		`SELECT * FROM "AUTH"."USERS" FETCH FIRST 5 ROWS ONLY`, batch, batch)
	require.NoError(t, err)
	assert.Len(t, page2, batch, "second page should return batch rows")

	_, page3, _, err := testDao.FetchQueryRows(ctx,
		`SELECT * FROM "AUTH"."USERS" FETCH FIRST 5 ROWS ONLY`, batch, batch*2)
	require.NoError(t, err)
	assert.Len(t, page3, 1, "last page should have the remainder row")
}

func TestPing(t *testing.T) {
	err := testDao.Ping(context.Background())
	require.NoError(t, err)
}

func TestDefaultPKType(t *testing.T) {
	assert.Equal(t, "NUMBER", testDao.DefaultPKType())
}

func TestGetTableDDL(t *testing.T) {
	ctx := context.Background()

	ddl, err := testDao.GetTableDDL(ctx, testSchema, "USERS")
	require.NoError(t, err)
	assert.Contains(t, ddl, "CREATE TABLE")
	assert.Contains(t, ddl, "USERS")
	assert.Contains(t, ddl, "EMAIL")
}

func TestGetIncomingForeignKeys(t *testing.T) {
	ctx := context.Background()

	// USER_ROLES has FKs pointing to USERS, so USERS should have incoming FKs.
	fks, err := testDao.GetIncomingForeignKeys(ctx, testSchema, "USERS")
	require.NoError(t, err)
	require.NotEmpty(t, fks)

	var found bool
	for _, fk := range fks {
		if fk.Table == "USER_ROLES" {
			found = true
		}
	}
	assert.True(t, found, "USER_ROLES should appear as an incoming FK on USERS")
}

func TestExplainAnalyze_ReturnsOutput(t *testing.T) {
	ctx := context.Background()

	plan, err := testDao.ExplainAnalyze(ctx, `SELECT * FROM "AUTH"."USERS"`)
	require.NoError(t, err)
	assert.NotEmpty(t, plan)
}
