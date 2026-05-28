//go:build wezterm

package harness

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/util"

	_ "github.com/kopecmaciej/vi-sql/internal/driver/mysql"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/postgres"
	_ "github.com/kopecmaciej/vi-sql/internal/driver/sqlserver"
)

const (
	fixtureSchemaPrefix = "vi_sql_test_"
	fixtureTableName    = "fixture"
)

var fixtureSeq atomic.Int64

// FixtureDB is a direct database connection used to arrange and verify state
// independently of the vi-sql UI under test. Tests arrange via this connection,
// act through the UI, then assert via this connection.
type FixtureDB struct {
	t          *testing.T
	driver     database.Driver
	driverName string
}

func fixtureCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

// NewFixtureSchema creates an empty, uniquely-named schema and registers a
// teardown as test cleanup. The teardown runs on pass or fail, but not on
// SIGKILL/timeout — SweepFixtureSchemas covers that case on the next run.
func NewFixtureSchema(t *testing.T) (schema string, db *FixtureDB) {
	t.Helper()

	dsn := os.Getenv(EnvDSN)
	if dsn == "" {
		t.Skipf("%s not set — skipping fixture test", EnvDSN)
	}
	driverName, err := util.DetectDriverFromDSN(dsn)
	if err != nil || (driverName != "postgres" && driverName != "sqlserver" && driverName != "mysql") {
		t.Skipf("fixture helpers require a Postgres, SQL Server, or MySQL DSN — skipping")
	}

	driver := openFixtureDriver(t, dsn)
	db = &FixtureDB{t: t, driver: driver, driverName: driverName}

	schema = fmt.Sprintf("%s%d_%d", fixtureSchemaPrefix, time.Now().Unix(), fixtureSeq.Add(1))
	db.Exec("CREATE SCHEMA " + schema)

	t.Cleanup(func() {
		ctx, cancel := fixtureCtx()
		defer cancel()
		if err := dropSchema(ctx, driver, driverName, schema); err != nil {
			t.Errorf("fixture cleanup: drop schema %s: %v", schema, err)
		}
		_ = driver.Close(context.Background())
	})

	return schema, db
}

// NewFixtureTable creates a fixture schema with one empty table inside it.
// columns is the body of the CREATE TABLE column list, e.g.
// "id serial primary key, name text, email text". Postgres-specific syntax
// ("serial") is translated to the SQL Server equivalent automatically.
// Seed rows via db.Exec.
func NewFixtureTable(t *testing.T, columns string) (schema, table string, db *FixtureDB) {
	t.Helper()
	schema, db = NewFixtureSchema(t)
	table = fixtureTableName
	db.Exec(fmt.Sprintf("CREATE TABLE %s (%s)", db.Qualified(schema, table), db.translateColumns(columns)))
	return schema, table, db
}

// SweepFixtureSchemas drops every schema left over from a previous run whose
// name carries fixtureSchemaPrefix. Call it from TestMain before m.Run() so
// orphans from crashed runs never accumulate. Best-effort: failures are logged,
// never fatal.
func SweepFixtureSchemas() {
	dsn := os.Getenv(EnvDSN)
	if dsn == "" {
		return
	}
	driverName, err := util.DetectDriverFromDSN(dsn)
	if err != nil || (driverName != "postgres" && driverName != "sqlserver" && driverName != "mysql") {
		return
	}

	cfg, err := database.BuildConfigFromDSN("sweep", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fixture sweep: build config: %v\n", err)
		return
	}
	driver, _, err := database.NewDriver(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fixture sweep: new driver: %v\n", err)
		return
	}
	ctx, cancel := fixtureCtx()
	defer cancel()
	if err := driver.Connect(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "fixture sweep: connect: %v\n", err)
		return
	}
	defer func() { _ = driver.Close(context.Background()) }()

	schemas, err := driver.ListSchemas(ctx, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "fixture sweep: list schemas: %v\n", err)
		return
	}
	for _, s := range schemas {
		if !strings.HasPrefix(s.Schema, fixtureSchemaPrefix) {
			continue
		}
		if err := dropSchema(ctx, driver, driverName, s.Schema); err != nil {
			fmt.Fprintf(os.Stderr, "fixture sweep: drop %s: %v\n", s.Schema, err)
		}
	}
}

// Qualified returns a quoted, schema-qualified identifier for use in SQL.
func (db *FixtureDB) Qualified(schema, table string) string {
	switch db.driverName {
	case "sqlserver":
		return util.BracketQuoter.Table(schema, table)
	case "mysql":
		return util.BacktickQuoter.Table(schema, table)
	default:
		return util.ANSIQuoter.Table(schema, table)
	}
}

// translateColumns rewrites Postgres-specific column syntax to the equivalent
// for the target driver. Callers write portable column lists using Postgres
// syntax (e.g. "id serial primary key") and this function adapts them.
func (db *FixtureDB) translateColumns(columns string) string {
	switch db.driverName {
	case "sqlserver":
		columns = strings.ReplaceAll(columns, "serial", "int identity(1,1)")
		columns = strings.ReplaceAll(columns, "timestamptz", "datetimeoffset")
		columns = strings.ReplaceAll(columns, "now()", "SYSDATETIMEOFFSET()")
		columns = strings.ReplaceAll(columns, " text", " nvarchar(max)")
		return columns
	case "mysql":
		columns = strings.ReplaceAll(columns, "serial", "INT AUTO_INCREMENT")
		columns = strings.ReplaceAll(columns, "timestamptz", "datetime")
		return columns
	default:
		return columns
	}
}

// dropSchema drops schema and all its tables.
// Postgres supports CASCADE; MySQL drops all tables implicitly;
// SQL Server requires dropping every table first.
func dropSchema(ctx context.Context, driver database.Driver, driverName, schema string) error {
	switch driverName {
	case "mysql":
		_, err := driver.ExecuteStatement(ctx, "DROP SCHEMA "+util.BacktickQuoter.Ident(schema))
		return err
	case "sqlserver":
		escapedSchema := strings.ReplaceAll(schema, "'", "''")
		fkRows, _, err := driver.ExecuteQuery(ctx,
			"SELECT tc.table_name, tc.constraint_name FROM information_schema.table_constraints tc "+
				"WHERE tc.constraint_type = 'FOREIGN KEY' AND tc.table_schema = '"+escapedSchema+"'")
		if err != nil {
			return fmt.Errorf("list fk constraints: %w", err)
		}
		for _, row := range fkRows {
			tbl := fmt.Sprintf("%v", row["table_name"])
			con := fmt.Sprintf("%v", row["constraint_name"])
			if _, err := driver.ExecuteStatement(ctx, "ALTER TABLE "+util.BracketQuoter.Table(schema, tbl)+" DROP CONSTRAINT ["+con+"]"); err != nil {
				return fmt.Errorf("drop fk constraint %s: %w", con, err)
			}
		}
		rows, _, err := driver.ExecuteQuery(ctx,
			"SELECT table_name FROM information_schema.tables WHERE table_schema = '"+escapedSchema+"'")
		if err != nil {
			return fmt.Errorf("list tables: %w", err)
		}
		for _, row := range rows {
			tbl := fmt.Sprintf("%v", row["table_name"])
			if _, err := driver.ExecuteStatement(ctx, "DROP TABLE "+util.BracketQuoter.Table(schema, tbl)); err != nil {
				return fmt.Errorf("drop table %s: %w", tbl, err)
			}
		}
		_, err = driver.ExecuteStatement(ctx, "DROP SCHEMA ["+schema+"]")
		return err
	default: // postgres
		_, err := driver.ExecuteStatement(ctx, "DROP SCHEMA "+schema+" CASCADE")
		return err
	}
}

// CreateTable creates a table in schema using the portable column list syntax.
// Portable means callers write Postgres-style ("id serial primary key, name text")
// and translateColumns adapts it to the target driver automatically.
func (db *FixtureDB) CreateTable(schema, table, columns string) {
	db.t.Helper()
	db.Exec(fmt.Sprintf("CREATE TABLE %s (%s)", db.Qualified(schema, table), db.translateColumns(columns)))
}

// InsertDefault inserts one row using all-default column values.
// Postgres/SQL Server: DEFAULT VALUES; MySQL: VALUES ().
func (db *FixtureDB) InsertDefault(schema, table string) {
	db.t.Helper()
	if db.driverName == "mysql" {
		db.Exec(fmt.Sprintf("INSERT INTO %s VALUES ()", db.Qualified(schema, table)))
	} else {
		db.Exec(fmt.Sprintf("INSERT INTO %s DEFAULT VALUES", db.Qualified(schema, table)))
	}
}

// FKConstraint returns a table-level FOREIGN KEY clause suitable for use
// inside a CREATE TABLE column list. This works across all three supported
// drivers; MySQL ignores inline REFERENCES but honours table-level FK syntax.
func (db *FixtureDB) FKConstraint(col, refSchema, refTable, refCol string) string {
	return fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s(%s)", col, db.Qualified(refSchema, refTable), refCol)
}

// Exec runs a non-row-returning statement (DDL, INSERT/UPDATE/DELETE) and
// fails the test on error.
func (db *FixtureDB) Exec(stmt string) {
	db.t.Helper()
	ctx, cancel := fixtureCtx()
	defer cancel()
	if _, err := db.driver.ExecuteStatement(ctx, stmt); err != nil {
		db.t.Fatalf("fixture exec %q: %v", stmt, err)
	}
}

// Query runs a SELECT and returns the rows, failing the test on error.
func (db *FixtureDB) Query(sql string) []database.Row {
	db.t.Helper()
	ctx, cancel := fixtureCtx()
	defer cancel()
	rows, _, err := db.driver.ExecuteQuery(ctx, sql)
	if err != nil {
		db.t.Fatalf("fixture query %q: %v", sql, err)
	}
	return rows
}

// Count returns the number of rows in schema.table.
func (db *FixtureDB) Count(schema, table string) int64 {
	db.t.Helper()
	rows := db.Query("SELECT count(*) AS c FROM " + db.Qualified(schema, table))
	if len(rows) != 1 {
		db.t.Fatalf("fixture count: expected 1 row, got %d", len(rows))
	}
	return fixtureInt64(db.t, rows[0]["c"])
}

// TableExists reports whether schema.table is present in the catalog.
func (db *FixtureDB) TableExists(schema, table string) bool {
	db.t.Helper()
	rows := db.Query(fmt.Sprintf(
		"SELECT 1 FROM information_schema.tables WHERE table_schema = %s AND table_name = %s",
		fixtureLiteral(schema), fixtureLiteral(table)))
	return len(rows) > 0
}

// ColumnExists reports whether schema.table has a column named column.
func (db *FixtureDB) ColumnExists(schema, table, column string) bool {
	db.t.Helper()
	rows := db.Query(fmt.Sprintf(
		"SELECT 1 FROM information_schema.columns WHERE table_schema = %s AND table_name = %s AND column_name = %s",
		fixtureLiteral(schema), fixtureLiteral(table), fixtureLiteral(column)))
	return len(rows) > 0
}

// DropIndexSQL returns a driver-appropriate DROP INDEX statement.
func (db *FixtureDB) DropIndexSQL(schema, table, indexName string) string {
	switch db.driverName {
	case "mysql":
		return fmt.Sprintf("DROP INDEX %s ON %s", util.BacktickQuoter.Ident(indexName), db.Qualified(schema, table))
	case "sqlserver":
		return fmt.Sprintf("DROP INDEX %s ON %s", util.BracketQuoter.Ident(indexName), db.Qualified(schema, table))
	default: // postgres
		return fmt.Sprintf("DROP INDEX %s", util.ANSIQuoter.Table(schema, indexName))
	}
}

// IndexExists reports whether an index named indexName exists on schema.table.
func (db *FixtureDB) IndexExists(schema, table, indexName string) bool {
	db.t.Helper()
	var q string
	switch db.driverName {
	case "mysql":
		q = fmt.Sprintf(
			"SELECT 1 FROM information_schema.statistics WHERE table_schema = %s AND table_name = %s AND index_name = %s LIMIT 1",
			fixtureLiteral(schema), fixtureLiteral(table), fixtureLiteral(indexName))
	case "sqlserver":
		q = fmt.Sprintf(
			"SELECT TOP 1 1 FROM sys.indexes si JOIN sys.objects so ON si.object_id = so.object_id JOIN sys.schemas ss ON so.schema_id = ss.schema_id WHERE ss.name = %s AND so.name = %s AND si.name = %s",
			fixtureLiteral(schema), fixtureLiteral(table), fixtureLiteral(indexName))
	default: // postgres
		q = fmt.Sprintf(
			"SELECT 1 FROM pg_indexes WHERE schemaname = %s AND tablename = %s AND indexname = %s",
			fixtureLiteral(schema), fixtureLiteral(table), fixtureLiteral(indexName))
	}
	rows := db.Query(q)
	return len(rows) > 0
}

func openFixtureDriver(t *testing.T, dsn string) database.Driver {
	t.Helper()
	cfg, err := database.BuildConfigFromDSN("fixture", dsn)
	if err != nil {
		t.Fatalf("fixture: build config: %v", err)
	}
	driver, _, err := database.NewDriver(cfg)
	if err != nil {
		t.Fatalf("fixture: new driver: %v", err)
	}
	ctx, cancel := fixtureCtx()
	defer cancel()
	if err := driver.Connect(ctx); err != nil {
		t.Fatalf("fixture: connect: %v", err)
	}
	return driver
}

func fixtureInt64(t *testing.T, v any) int64 {
	t.Helper()
	switch n := v.(type) {
	case int64:
		return n
	case int32:
		return int64(n)
	case int:
		return int64(n)
	case string:
		i, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			t.Fatalf("fixture: expected integer, got string %q", n)
		}
		return i
	default:
		t.Fatalf("fixture: expected integer, got %T", v)
		return 0
	}
}

// fixtureLiteral wraps s as a SQL string literal, escaping embedded quotes.
func fixtureLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
