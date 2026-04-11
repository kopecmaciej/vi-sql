package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/rs/zerolog/log"
)

// Dao implements database.Driver for SQLite.
type Dao struct {
	client *Client
}

func NewDao(client *Client) *Dao {
	return &Dao{client: client}
}

func (d *Dao) Connect(ctx context.Context) error {
	return d.client.Connect(ctx)
}

func (d *Dao) Close(ctx context.Context) error {
	d.client.Close()
	return nil
}

func (d *Dao) Ping(ctx context.Context) error {
	return d.client.Ping(ctx)
}

func (d *Dao) GetServerInfo(ctx context.Context) (*database.ServerInfo, error) {
	var version string
	if err := d.client.DB.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&version); err != nil {
		return nil, fmt.Errorf("failed to get sqlite version: %w", err)
	}
	return &database.ServerInfo{
		Version: "SQLite " + version,
		Extra:   make(map[string]string),
	}, nil
}

func (d *Dao) GetActiveSessions(_ context.Context) (int64, error) {
	return 0, nil
}

func (d *Dao) ListSchemasWithTables(ctx context.Context, nameFilter string) ([]database.SchemaWithTables, error) {
	query := "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
	args := []any{}
	if nameFilter != "" {
		query += " AND name LIKE ?"
		args = append(args, "%"+nameFilter+"%")
	}
	query += " ORDER BY name"

	rows, err := d.client.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return []database.SchemaWithTables{{Schema: "main", Tables: tables}}, nil
}

func (d *Dao) GetTableColumns(ctx context.Context, schema, table string) ([]database.ColumnInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx,
		fmt.Sprintf("PRAGMA table_info(%s)", quoteSQLiteIdent(table)))
	if err != nil {
		return nil, fmt.Errorf("failed to get table columns: %w", err)
	}
	defer rows.Close()

	var columns []database.ColumnInfo
	for rows.Next() {
		var cid, notNull, pk int
		var name, dataType string
		var dfltValue *string
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk); err != nil {
			return nil, err
		}
		columns = append(columns, database.ColumnInfo{
			Name:       name,
			DataType:   dataType,
			IsNullable: notNull == 0,
			Default:    dfltValue,
			IsPK:       pk > 0,
			Ordinal:    cid + 1,
		})
	}
	return columns, rows.Err()
}

func (d *Dao) GetTableConstraints(ctx context.Context, schema, table string) ([]database.ConstraintInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx,
		fmt.Sprintf("PRAGMA table_info(%s)", quoteSQLiteIdent(table)))
	if err != nil {
		return nil, fmt.Errorf("failed to get table constraints: %w", err)
	}
	defer rows.Close()

	var pkCols []string
	for rows.Next() {
		var cid, notNull, pk int
		var name, dataType string
		var dfltValue *string
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk); err != nil {
			return nil, err
		}
		if pk > 0 {
			pkCols = append(pkCols, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(pkCols) == 0 {
		return nil, nil
	}
	return []database.ConstraintInfo{{
		Name:    "PRIMARY KEY",
		Type:    "PRIMARY KEY",
		Columns: pkCols,
	}}, nil
}

func (d *Dao) GetTableForeignKeys(ctx context.Context, schema, table string) ([]database.ForeignKeyInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx,
		fmt.Sprintf("PRAGMA foreign_key_list(%s)", quoteSQLiteIdent(table)))
	if err != nil {
		return nil, fmt.Errorf("failed to get foreign keys: %w", err)
	}
	defer rows.Close()

	fkMap := map[int]*database.ForeignKeyInfo{}
	fkOrder := []int{}

	for rows.Next() {
		var id, seq int
		var refTable, fromCol, toCol, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &seq, &refTable, &fromCol, &toCol, &onUpdate, &onDelete, &match); err != nil {
			return nil, err
		}
		if _, ok := fkMap[id]; !ok {
			fkMap[id] = &database.ForeignKeyInfo{
				Name:            fmt.Sprintf("fk_%s_%d", table, id),
				ReferencedTable: refTable,
				OnUpdate:        onUpdate,
				OnDelete:        onDelete,
			}
			fkOrder = append(fkOrder, id)
		}
		fkMap[id].Columns = append(fkMap[id].Columns, fromCol)
		fkMap[id].ReferencedCols = append(fkMap[id].ReferencedCols, toCol)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]database.ForeignKeyInfo, 0, len(fkOrder))
	for _, id := range fkOrder {
		result = append(result, *fkMap[id])
	}
	return result, nil
}

func (d *Dao) ListRows(ctx context.Context, state *database.TableState, where, orderBy string,
	columns []string, countCallback func(int64)) (string, []database.Row, error) {

	colExpr := "*"
	if len(columns) > 0 {
		quoted := make([]string, len(columns))
		for i, c := range columns {
			quoted[i] = quoteSQLiteIdent(c)
		}
		colExpr = strings.Join(quoted, ", ")
	}

	query := fmt.Sprintf("SELECT %s FROM %s", colExpr, quoteSQLiteIdent(state.Table))
	args := []any{}

	if where != "" {
		if err := database.SanitizeWhereClause(where); err != nil {
			return "", nil, err
		}
		query += " WHERE " + where
	}
	if orderBy != "" {
		query += " ORDER BY " + orderBy
	}
	displayQuery := query + fmt.Sprintf(" LIMIT %d OFFSET %d", state.Limit, state.Offset)
	query += " LIMIT ? OFFSET ?"
	args = append(args, state.Limit, state.Offset)

	rows, err := d.client.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return "", nil, fmt.Errorf("failed to list rows: %w", err)
	}
	defer rows.Close()

	result, err := scanRows(rows)
	if err != nil {
		return "", nil, err
	}

	if countCallback != nil {
		go func() {
			countQuery := fmt.Sprintf("SELECT count(*) FROM %s", quoteSQLiteIdent(state.Table))
			if where != "" {
				countQuery += " WHERE " + where
			}
			var count int64
			if err := d.client.DB.QueryRowContext(ctx, countQuery).Scan(&count); err != nil {
				log.Error().Err(err).Msg("Failed to count rows")
				return
			}
			countCallback(count)
		}()
	}

	return displayQuery, result, nil
}

func (d *Dao) GetRow(ctx context.Context, schema, table string, pk database.PrimaryKey) (database.Row, error) {
	whereParts, args := buildPKWhere(pk)
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s",
		quoteSQLiteIdent(table), strings.Join(whereParts, " AND "))

	rows, err := d.client.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get row: %w", err)
	}
	defer rows.Close()

	result, err := scanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("row not found")
	}
	return result[0], nil
}

func (d *Dao) InsertRow(ctx context.Context, schema, table string, row database.Row) (database.PrimaryKey, error) {
	cols := make([]string, 0, len(row))
	placeholders := make([]string, 0, len(row))
	args := make([]any, 0, len(row))

	for col, val := range row {
		cols = append(cols, quoteSQLiteIdent(col))
		placeholders = append(placeholders, "?")
		args = append(args, val)
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quoteSQLiteIdent(table), strings.Join(cols, ", "), strings.Join(placeholders, ", "))

	result, err := d.client.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return database.PrimaryKey{}, fmt.Errorf("failed to insert row: %w", err)
	}

	lastID, err := result.LastInsertId()
	if err != nil {
		return database.PrimaryKey{}, nil
	}

	pkCols, err := d.getPrimaryKeyColumns(ctx, table)
	if err != nil || len(pkCols) != 1 {
		return database.PrimaryKey{}, nil
	}

	return database.PrimaryKey{
		Columns: map[string]any{pkCols[0]: fmt.Sprintf("%d", lastID)},
	}, nil
}

func (d *Dao) UpdateRow(ctx context.Context, schema, table string, pk database.PrimaryKey, original, updated database.Row) error {
	setClauses := []string{}
	args := []any{}

	pkSet := make(map[string]bool, len(pk.Columns))
	for col := range pk.Columns {
		pkSet[col] = true
	}

	for col, newVal := range updated {
		if col == "_pk" || pkSet[col] {
			continue
		}
		oldVal, exists := original[col]
		if !exists || fmt.Sprint(oldVal) != fmt.Sprint(newVal) {
			setClauses = append(setClauses, fmt.Sprintf("%s = ?", quoteSQLiteIdent(col)))
			args = append(args, newVal)
		}
	}

	if len(setClauses) == 0 {
		return nil
	}

	whereParts := []string{}
	for col, val := range pk.Columns {
		whereParts = append(whereParts, fmt.Sprintf("%s = ?", quoteSQLiteIdent(col)))
		args = append(args, val)
	}

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		quoteSQLiteIdent(table), strings.Join(setClauses, ", "), strings.Join(whereParts, " AND "))

	result, err := d.client.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update row: %w", err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return fmt.Errorf("no row found to update")
	}
	return nil
}

func (d *Dao) DeleteRows(ctx context.Context, schema, table string, pks []database.PrimaryKey) error {
	for _, pk := range pks {
		whereParts, args := buildPKWhere(pk)
		query := fmt.Sprintf("DELETE FROM %s WHERE %s",
			quoteSQLiteIdent(table), strings.Join(whereParts, " AND "))

		result, err := d.client.DB.ExecContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("failed to delete row: %w", err)
		}
		if n, _ := result.RowsAffected(); n == 0 {
			log.Warn().Interface("pk", pk.Columns).Msg("No row found to delete")
		}
	}
	return nil
}

func (d *Dao) CommonDataTypes() []string {
	return []string{
		"INTEGER",
		"REAL",
		"TEXT",
		"BLOB",
		"NUMERIC",
		"BOOLEAN",
		"DATE",
		"DATETIME",
		"TIMESTAMP",
		"VARCHAR(255)",
		"CHAR(1)",
	}
}

func (d *Dao) DefaultCreateTableDDL(schema, tableName string) string {
	return fmt.Sprintf("CREATE TABLE %s (id INTEGER PRIMARY KEY AUTOINCREMENT)", quoteSQLiteIdent(tableName))
}

func (d *Dao) CreateTable(ctx context.Context, schema, ddl string) error {
	_, err := d.client.DB.ExecContext(ctx, ddl)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}
	return nil
}

func (d *Dao) DropTable(ctx context.Context, schema, table string) error {
	_, err := d.client.DB.ExecContext(ctx,
		fmt.Sprintf("DROP TABLE IF EXISTS %s", quoteSQLiteIdent(table)))
	if err != nil {
		return fmt.Errorf("failed to drop table: %w", err)
	}
	return nil
}

func (d *Dao) RenameTable(ctx context.Context, schema, old, newName string) error {
	_, err := d.client.DB.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s RENAME TO %s", quoteSQLiteIdent(old), quoteSQLiteIdent(newName)))
	if err != nil {
		return fmt.Errorf("failed to rename table: %w", err)
	}
	return nil
}

func (d *Dao) TruncateTable(ctx context.Context, schema, table string) error {
	_, err := d.client.DB.ExecContext(ctx,
		fmt.Sprintf("DELETE FROM %s", quoteSQLiteIdent(table)))
	if err != nil {
		return fmt.Errorf("failed to truncate table: %w", err)
	}
	return nil
}

func (d *Dao) GetIndexes(ctx context.Context, schema, table string) ([]database.IndexInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx,
		fmt.Sprintf("PRAGMA index_list(%s)", quoteSQLiteIdent(table)))
	if err != nil {
		return nil, fmt.Errorf("failed to get index list: %w", err)
	}
	defer rows.Close()

	type idxRow struct {
		seq     int
		name    string
		unique  int
		origin  string
		partial int
	}

	var idxRows []idxRow
	for rows.Next() {
		var r idxRow
		if err := rows.Scan(&r.seq, &r.name, &r.unique, &r.origin, &r.partial); err != nil {
			return nil, err
		}
		idxRows = append(idxRows, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var indexes []database.IndexInfo
	for _, r := range idxRows {
		colRows, err := d.client.DB.QueryContext(ctx,
			fmt.Sprintf("PRAGMA index_info(%s)", quoteSQLiteIdent(r.name)))
		if err != nil {
			return nil, err
		}
		var cols []string
		for colRows.Next() {
			var seqno, cid int
			var colName string
			if err := colRows.Scan(&seqno, &cid, &colName); err != nil {
				colRows.Close()
				return nil, err
			}
			cols = append(cols, colName)
		}
		colRows.Close()
		if err := colRows.Err(); err != nil {
			return nil, err
		}

		indexes = append(indexes, database.IndexInfo{
			Name:      r.name,
			Columns:   cols,
			IsUnique:  r.unique == 1,
			IsPrimary: r.origin == "pk",
		})
	}

	return indexes, nil
}

func (d *Dao) CreateIndex(ctx context.Context, schema, table string, def database.IndexDefinition) error {
	uniqueStr := ""
	if def.IsUnique {
		uniqueStr = "UNIQUE "
	}

	quotedCols := make([]string, len(def.Columns))
	for i, c := range def.Columns {
		parts := strings.Fields(c)
		quotedCols[i] = quoteSQLiteIdent(parts[0])
		if len(parts) > 1 {
			quotedCols[i] += " " + parts[1]
		}
	}

	query := fmt.Sprintf("CREATE %sINDEX %s ON %s (%s)",
		uniqueStr, quoteSQLiteIdent(def.Name), quoteSQLiteIdent(table), strings.Join(quotedCols, ", "))

	_, err := d.client.DB.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}
	return nil
}

func (d *Dao) DropIndex(ctx context.Context, schema, indexName string) error {
	_, err := d.client.DB.ExecContext(ctx,
		fmt.Sprintf("DROP INDEX %s", quoteSQLiteIdent(indexName)))
	if err != nil {
		return fmt.Errorf("failed to drop index: %w", err)
	}
	return nil
}

func (d *Dao) ListQueryRows(ctx context.Context, rawSQL string, limit, offset int64,
	countCallback func(int64)) (string, []database.Row, []database.ColumnInfo, error) {

	var displayQuery, paramQuery string
	if database.HasLimitClause(rawSQL) || database.IsExplainQuery(rawSQL) {
		displayQuery = rawSQL
		paramQuery = rawSQL
	} else {
		displayQuery = fmt.Sprintf("SELECT * FROM (%s) AS _q LIMIT %d OFFSET %d", rawSQL, limit, offset)
		paramQuery = fmt.Sprintf("SELECT * FROM (%s) AS _q LIMIT ? OFFSET ?", rawSQL)
	}

	rows, err := d.client.DB.QueryContext(ctx, paramQuery, limit, offset)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return "", nil, nil, err
	}
	cols := make([]database.ColumnInfo, len(colTypes))
	for i, ct := range colTypes {
		cols[i] = database.ColumnInfo{Name: ct.Name(), DataType: strings.ToLower(ct.DatabaseTypeName()), Ordinal: i + 1}
	}

	result, err := scanRows(rows)
	if err != nil {
		return "", nil, nil, err
	}

	if countCallback != nil && !database.IsExplainQuery(rawSQL) {
		go func() {
			countQuery := fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS _q", rawSQL)
			var count int64
			if err := d.client.DB.QueryRowContext(ctx, countQuery).Scan(&count); err != nil {
				log.Error().Err(err).Msg("Failed to count query rows")
				return
			}
			countCallback(count)
		}()
	}

	return displayQuery, result, cols, nil
}

func (d *Dao) ExecuteQuery(ctx context.Context, query string) ([]database.Row, []database.ColumnInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	colInfos := make([]database.ColumnInfo, len(cols))
	for i, c := range cols {
		colInfos[i] = database.ColumnInfo{Name: c, Ordinal: i + 1}
	}

	result, err := scanRows(rows)
	if err != nil {
		return nil, nil, err
	}

	return result, colInfos, nil
}

func (d *Dao) ExecuteStatement(ctx context.Context, stmt string) (int64, error) {
	result, err := d.client.DB.ExecContext(ctx, stmt)
	if err != nil {
		return 0, fmt.Errorf("failed to execute statement: %w", err)
	}
	return result.RowsAffected()
}

func (d *Dao) ExplainQuery(ctx context.Context, sql string) (string, error) {
	rows, err := d.client.DB.QueryContext(ctx, "EXPLAIN QUERY PLAN "+sql)
	if err != nil {
		return "", fmt.Errorf("failed to explain query: %w", err)
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			return "", err
		}
		lines = append(lines, detail)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}

func (d *Dao) GetTableColumnNames(ctx context.Context, schema, table string) ([]string, error) {
	rows, err := d.client.DB.QueryContext(ctx,
		fmt.Sprintf("PRAGMA table_info(%s)", quoteSQLiteIdent(table)))
	if err != nil {
		return nil, fmt.Errorf("failed to get column names: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var cid, notNull, pk int
		var name, dataType string
		var dfltValue *string
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// getPrimaryKeyColumns returns the PK column names for a table, sorted by pk index.
func (d *Dao) getPrimaryKeyColumns(ctx context.Context, table string) ([]string, error) {
	rows, err := d.client.DB.QueryContext(ctx,
		fmt.Sprintf("PRAGMA table_info(%s)", quoteSQLiteIdent(table)))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type pkEntry struct {
		name string
		idx  int
	}
	var pkCols []pkEntry
	for rows.Next() {
		var cid, notNull, pk int
		var name, dataType string
		var dfltValue *string
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &dfltValue, &pk); err != nil {
			return nil, err
		}
		if pk > 0 {
			pkCols = append(pkCols, pkEntry{name, pk})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(pkCols, func(i, j int) bool { return pkCols[i].idx < pkCols[j].idx })

	names := make([]string, len(pkCols))
	for i, c := range pkCols {
		names[i] = c.name
	}
	return names, nil
}

// scanRows scans all rows from a database/sql result set into database.Row maps.
func scanRows(rows *sql.Rows) ([]database.Row, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var result []database.Row
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(database.Row, len(cols))
		for i, col := range cols {
			switch v := vals[i].(type) {
			case nil:
				row[col] = nil
			case []byte:
				row[col] = string(v)
			default:
				row[col] = fmt.Sprint(v)
			}
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
