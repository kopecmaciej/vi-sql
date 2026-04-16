package database

import "context"

// Driver is the core abstraction for database backends.
// Implement this interface to add support for a new SQL database.
type Driver interface {
	// Basic
	Connect(ctx context.Context) error
	Close(ctx context.Context) error
	Ping(ctx context.Context) error
	GetServerInfo(ctx context.Context) (*ServerInfo, error)
	GetActiveSessions(ctx context.Context) (int64, error)
	// Schema browsing
	ListSchemas(ctx context.Context, nameFilter string) ([]Schema, error)
	// Table structure
	GetTableColumns(ctx context.Context, schema, table string) ([]ColumnInfo, error)
	GetTableConstraints(ctx context.Context, schema, table string) ([]ConstraintInfo, error)
	GetTableForeignKeys(ctx context.Context, schema, table string) ([]ForeignKeyInfo, error)
	// Row CRUD
	// GetEstimatedRowCount returns a fast row-count estimate for a table.
	// PostgreSQL reads pg_class.reltuples (instant); SQLite returns 0 (no
	// equivalent — the caller should fall back to the async exact count).
	GetEstimatedRowCount(ctx context.Context, schema, table string) (int64, error)
	// ListRows fetches a page of rows. ctx controls both the fetch and the
	// background COUNT(*) goroutine; cancel it to abort an in-flight count.
	ListRows(ctx context.Context, state *TableState, where, orderBy string,
		columns []string, countCallback func(int64)) (string, []Row, error)
	GetRow(ctx context.Context, schema, table string, pk PrimaryKey) (Row, error)
	InsertRow(ctx context.Context, schema, table string, row Row) (PrimaryKey, error)
	UpdateRow(ctx context.Context, schema, table string, pk PrimaryKey, original, updated Row) error
	DeleteRows(ctx context.Context, schema, table string, pks []PrimaryKey) error
	// DDL
	CommonDataTypes() []string
	DefaultCreateTableDDL(schema, tableName string) string
	CreateTable(ctx context.Context, schema, ddl string) error
	DropTable(ctx context.Context, schema, table string) error
	RenameTable(ctx context.Context, schema, old, newName string) error
	RenameColumn(ctx context.Context, schema, table, old, newName string) error
	TruncateTable(ctx context.Context, schema, table string) error
	// Indexes
	GetIndexes(ctx context.Context, schema, table string) ([]IndexInfo, error)
	CreateIndex(ctx context.Context, schema, table string, def IndexDefinition) error
	DropIndex(ctx context.Context, schema, indexName string) error
	// Raw SQL
	// ListQueryRows wraps rawSQL in a subquery and applies LIMIT/OFFSET for
	// pagination, mirroring the behaviour of ListRows for regular tables.
	// countCallback is fired asynchronously with the total result-set size.
	ListQueryRows(ctx context.Context, rawSQL string, limit, offset int64,
		countCallback func(int64)) (query string, rows []Row, cols []ColumnInfo, err error)
	ExecuteQuery(ctx context.Context, query string) ([]Row, []ColumnInfo, error)
	ExecuteStatement(ctx context.Context, stmt string) (int64, error)
	// Postgres: EXPLAIN (FORMAT JSON); SQLite: EXPLAIN QUERY PLAN.
	ExplainPlan(ctx context.Context, sql string) (string, error)
	// Postgres: EXPLAIN (ANALYZE, FORMAT JSON); SQLite: same as ExplainPlan.
	ExplainAnalyze(ctx context.Context, sql string) (string, error)
	// Autocomplete
	GetTableColumnNames(ctx context.Context, schema, table string) ([]string, error)
}
