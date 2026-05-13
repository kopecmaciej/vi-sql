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
	ListSchemas(ctx context.Context, nameFilter string) ([]Schema, error)
	// Table structure
	GetTableColumns(ctx context.Context, schema, table string) ([]ColumnInfo, error)
	GetTableConstraints(ctx context.Context, schema, table string) ([]ConstraintInfo, error)
	GetTableForeignKeys(ctx context.Context, schema, table string) ([]ForeignKeyInfo, error)
	GetIncomingForeignKeys(ctx context.Context, schema, table string) ([]IncomingForeignKeyInfo, error)
	// GetEstimatedRowCount runs estimate count if available, if not normal count is executed
	GetEstimatedRowCount(ctx context.Context, schema, table string) (count int64, isEstimate bool, err error)
	FetchTableRows(ctx context.Context, state *TableState, where, orderBy string) (string, []Row, error)
	InsertRow(ctx context.Context, schema, table string, row Row) (PrimaryKey, error)
	UpdateRow(ctx context.Context, schema, table string, pk PrimaryKey, original, updated Row) error
	DeleteRows(ctx context.Context, schema, table string, pks []PrimaryKey) error
	// Views
	GetViewDDL(ctx context.Context, schema, view string) (string, error)
	// DDL
	CommonDataTypes() []string
	DefaultPKType() string
	DefaultCreateTableDDL(schema, tableName string) string
	GetTableDDL(ctx context.Context, schema, table string) (string, error)
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
	// FetchQueryRows wraps rawSQL in a subquery and applies LIMIT/OFFSET for
	// pagination, mirroring the behaviour of FetchTableRows for regular tables.
	FetchQueryRows(ctx context.Context, rawSQL string, limit, offset int64) (query string, rows []Row, cols []ColumnInfo, err error)
	ExecuteQuery(ctx context.Context, query string) ([]Row, []ColumnInfo, error)
	ExecuteStatement(ctx context.Context, stmt string) (int64, error)
	ExplainPlan(ctx context.Context, sql string) (string, error)
	ExplainAnalyze(ctx context.Context, sql string) (string, error)
	// Autocomplete
	GetTableColumnNames(ctx context.Context, schema, table string) ([]string, error)
}
