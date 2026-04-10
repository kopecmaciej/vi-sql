package testutil

import (
	"context"

	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/stretchr/testify/mock"
)

// MockDriver is a testify mock implementing database.Driver.
// Use On() to set expectations before passing to app.SetDriver().
type MockDriver struct {
	mock.Mock
}

func (m *MockDriver) Connect(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func (m *MockDriver) Close(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func (m *MockDriver) Ping(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func (m *MockDriver) GetServerInfo(ctx context.Context) (*database.ServerInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.ServerInfo), args.Error(1)
}

func (m *MockDriver) GetActiveSessions(ctx context.Context) (int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockDriver) ListSchemasWithTables(ctx context.Context, nameFilter string) ([]database.SchemaWithTables, error) {
	args := m.Called(ctx, nameFilter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]database.SchemaWithTables), args.Error(1)
}

func (m *MockDriver) GetTableColumns(ctx context.Context, schema, table string) ([]database.ColumnInfo, error) {
	args := m.Called(ctx, schema, table)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]database.ColumnInfo), args.Error(1)
}

func (m *MockDriver) GetTableConstraints(ctx context.Context, schema, table string) ([]database.ConstraintInfo, error) {
	args := m.Called(ctx, schema, table)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]database.ConstraintInfo), args.Error(1)
}

func (m *MockDriver) GetTableForeignKeys(ctx context.Context, schema, table string) ([]database.ForeignKeyInfo, error) {
	args := m.Called(ctx, schema, table)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]database.ForeignKeyInfo), args.Error(1)
}

func (m *MockDriver) ListRows(ctx context.Context, state *database.TableState, where, orderBy string, columns []string, countCallback func(int64)) (string, []database.Row, error) {
	args := m.Called(ctx, state, where, orderBy, columns, countCallback)
	if args.Get(1) == nil {
		return args.String(0), nil, args.Error(2)
	}
	return args.String(0), args.Get(1).([]database.Row), args.Error(2)
}

func (m *MockDriver) GetRow(ctx context.Context, schema, table string, pk database.PrimaryKey) (database.Row, error) {
	args := m.Called(ctx, schema, table, pk)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(database.Row), args.Error(1)
}

func (m *MockDriver) InsertRow(ctx context.Context, schema, table string, row database.Row) (database.PrimaryKey, error) {
	args := m.Called(ctx, schema, table, row)
	return args.Get(0).(database.PrimaryKey), args.Error(1)
}

func (m *MockDriver) UpdateRow(ctx context.Context, schema, table string, pk database.PrimaryKey, original, updated database.Row) error {
	return m.Called(ctx, schema, table, pk, original, updated).Error(0)
}

func (m *MockDriver) DeleteRows(ctx context.Context, schema, table string, pks []database.PrimaryKey) error {
	return m.Called(ctx, schema, table, pks).Error(0)
}

func (m *MockDriver) CommonDataTypes() []string {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]string)
}

func (m *MockDriver) DefaultCreateTableDDL(schema, tableName string) string {
	return m.Called(schema, tableName).String(0)
}

func (m *MockDriver) CreateTable(ctx context.Context, schema, ddl string) error {
	return m.Called(ctx, schema, ddl).Error(0)
}

func (m *MockDriver) DropTable(ctx context.Context, schema, table string) error {
	return m.Called(ctx, schema, table).Error(0)
}

func (m *MockDriver) RenameTable(ctx context.Context, schema, old, newName string) error {
	return m.Called(ctx, schema, old, newName).Error(0)
}

func (m *MockDriver) TruncateTable(ctx context.Context, schema, table string) error {
	return m.Called(ctx, schema, table).Error(0)
}

func (m *MockDriver) GetIndexes(ctx context.Context, schema, table string) ([]database.IndexInfo, error) {
	args := m.Called(ctx, schema, table)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]database.IndexInfo), args.Error(1)
}

func (m *MockDriver) CreateIndex(ctx context.Context, schema, table string, def database.IndexDefinition) error {
	return m.Called(ctx, schema, table, def).Error(0)
}

func (m *MockDriver) DropIndex(ctx context.Context, schema, indexName string) error {
	return m.Called(ctx, schema, indexName).Error(0)
}

func (m *MockDriver) ListQueryRows(ctx context.Context, rawSQL string, limit, offset int64, countCallback func(int64)) (string, []database.Row, []database.ColumnInfo, error) {
	args := m.Called(ctx, rawSQL, limit, offset, countCallback)
	rows, _ := args.Get(1).([]database.Row)
	cols, _ := args.Get(2).([]database.ColumnInfo)
	return args.String(0), rows, cols, args.Error(3)
}

func (m *MockDriver) ExecuteQuery(ctx context.Context, query string) ([]database.Row, []database.ColumnInfo, error) {
	args := m.Called(ctx, query)
	rows, _ := args.Get(0).([]database.Row)
	cols, _ := args.Get(1).([]database.ColumnInfo)
	return rows, cols, args.Error(2)
}

func (m *MockDriver) ExecuteStatement(ctx context.Context, stmt string) (int64, error) {
	args := m.Called(ctx, stmt)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockDriver) ExplainQuery(ctx context.Context, sql string) (string, error) {
	args := m.Called(ctx, sql)
	return args.String(0), args.Error(1)
}

func (m *MockDriver) GetTableColumnNames(ctx context.Context, schema, table string) ([]string, error) {
	args := m.Called(ctx, schema, table)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}
