package database

import "context"

// Driver is the core abstraction for database backends.
// Implement this interface to add support for a new SQL database.
type Driver interface {
	// Lifecycle
	Connect(ctx context.Context) error
	Close(ctx context.Context) error
	Ping(ctx context.Context) error

	// Server
	GetServerInfo(ctx context.Context) (*ServerInfo, error)
	GetActiveSessions(ctx context.Context) (int64, error)

	// Schema browsing
	ListSchemasWithTables(ctx context.Context, nameFilter string) ([]SchemaWithTables, error)

	// Table structure
	GetTableColumns(ctx context.Context, schema, table string) ([]ColumnInfo, error)
	GetTableConstraints(ctx context.Context, schema, table string) ([]ConstraintInfo, error)
	GetTableForeignKeys(ctx context.Context, schema, table string) ([]ForeignKeyInfo, error)

	// Row CRUD
	ListRows(ctx context.Context, state *TableState, where, orderBy string,
		columns []string, countCallback func(int64)) ([]Row, error)
	GetRow(ctx context.Context, schema, table string, pk PrimaryKey) (Row, error)
	InsertRow(ctx context.Context, schema, table string, row Row) (PrimaryKey, error)
	UpdateRow(ctx context.Context, schema, table string, pk PrimaryKey, original, updated Row) error
	DeleteRows(ctx context.Context, schema, table string, pks []PrimaryKey) error

	// DDL
	CreateTable(ctx context.Context, schema, ddl string) error
	DropTable(ctx context.Context, schema, table string) error
	RenameTable(ctx context.Context, schema, old, newName string) error
	TruncateTable(ctx context.Context, schema, table string) error

	// Indexes
	GetIndexes(ctx context.Context, schema, table string) ([]IndexInfo, error)
	CreateIndex(ctx context.Context, schema, table string, def IndexDefinition) error
	DropIndex(ctx context.Context, schema, indexName string) error

	// Raw SQL
	ExecuteQuery(ctx context.Context, query string) ([]Row, []ColumnInfo, error)
	ExecuteStatement(ctx context.Context, stmt string) (int64, error)

	// Autocomplete
	GetTableColumnNames(ctx context.Context, schema, table string) ([]string, error)
}
