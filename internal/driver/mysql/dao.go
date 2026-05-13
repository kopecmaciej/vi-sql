package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/kopecmaciej/vi-sql/internal/database"
	sqlpkg "github.com/kopecmaciej/vi-sql/internal/sql"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog/log"
)

const quote = util.BacktickQuoter

// Dao implements database.Driver for MySQL.
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
	info := &database.ServerInfo{Extra: make(map[string]string)}

	var version, host, dbName string
	var port, maxConns int64
	if err := d.client.DB.QueryRowContext(ctx,
		"SELECT @@version, @@hostname, @@port, DATABASE(), @@max_connections").
		Scan(&version, &host, &port, &dbName, &maxConns); err != nil {
		return nil, fmt.Errorf("failed to get server info: %w", err)
	}
	info.Version = "MySQL " + version
	info.Host = host
	info.Port = int(port)
	info.CurrentDB = dbName
	info.MaxConnections = maxConns

	var varName, uptime string
	if err := d.client.DB.QueryRowContext(ctx, "SHOW STATUS LIKE 'Uptime'").
		Scan(&varName, &uptime); err == nil {
		info.Uptime = uptime + "s"
	}

	var sessName string
	var sessions int64
	if err := d.client.DB.QueryRowContext(ctx, "SHOW STATUS LIKE 'Threads_connected'").
		Scan(&sessName, &sessions); err == nil {
		info.ActiveSessions = sessions
	}

	var dbSizeMB *float64
	if err := d.client.DB.QueryRowContext(ctx, `
		SELECT ROUND(SUM(data_length + index_length) / 1024 / 1024, 2)
		FROM information_schema.tables
		WHERE table_schema = DATABASE()`).Scan(&dbSizeMB); err == nil && dbSizeMB != nil {
		info.DatabaseSize = fmt.Sprintf("%.2f MiB", *dbSizeMB)
	}

	var sslVar, sslVersion, sslCipher string
	if err := d.client.DB.QueryRowContext(ctx, "SHOW STATUS LIKE 'Ssl_version'").
		Scan(&sslVar, &sslVersion); err == nil && sslVersion != "" {
		if err := d.client.DB.QueryRowContext(ctx, "SHOW STATUS LIKE 'Ssl_cipher'").
			Scan(&sslVar, &sslCipher); err == nil {
			info.TLS = sslVersion + " (" + sslCipher + ")"
		} else {
			info.TLS = sslVersion
		}
	}

	return info, nil
}

func (d *Dao) GetActiveSessions(ctx context.Context) (int64, error) {
	var varName string
	var count int64
	if err := d.client.DB.QueryRowContext(ctx, "SHOW STATUS LIKE 'Threads_connected'").
		Scan(&varName, &count); err != nil {
		return 0, fmt.Errorf("failed to get active sessions: %w", err)
	}
	return count, nil
}

func (d *Dao) ListSchemas(ctx context.Context, nameFilter string) ([]database.Schema, error) {
	query := `
		SELECT s.schema_name,
		       GROUP_CONCAT(CASE WHEN t.table_type = 'BASE TABLE' THEN t.table_name END ORDER BY t.table_name SEPARATOR ','),
		       GROUP_CONCAT(CASE WHEN t.table_type = 'VIEW'       THEN t.table_name END ORDER BY t.table_name SEPARATOR ',')
		FROM information_schema.schemata s
		LEFT JOIN information_schema.tables t
			ON s.schema_name = t.table_schema AND t.table_type IN ('BASE TABLE', 'VIEW')
		WHERE s.schema_name NOT IN ('information_schema', 'mysql', 'performance_schema', 'sys')`

	args := []any{}
	if nameFilter != "" {
		query += " AND (s.schema_name LIKE ? OR t.table_name LIKE ?)"
		args = append(args, "%"+nameFilter+"%", "%"+nameFilter+"%")
	}
	query += " GROUP BY s.schema_name ORDER BY s.schema_name"

	rows, err := d.client.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []database.Schema
	for rows.Next() {
		var schemaName string
		var tableStr, viewStr *string
		if err := rows.Scan(&schemaName, &tableStr, &viewStr); err != nil {
			return nil, err
		}
		var tables, views []string
		if tableStr != nil && *tableStr != "" {
			tables = strings.Split(*tableStr, ",")
		}
		if viewStr != nil && *viewStr != "" {
			views = strings.Split(*viewStr, ",")
		}
		result = append(result, database.Schema{Schema: schemaName, Tables: tables, Views: views})
	}
	return result, rows.Err()
}

func (d *Dao) GetViewDDL(ctx context.Context, schema, view string) (string, error) {
	var viewName, def, charset, collation string
	err := d.client.DB.QueryRowContext(ctx,
		fmt.Sprintf("SHOW CREATE VIEW %s", quote.Table(schema, view))).
		Scan(&viewName, &def, &charset, &collation)
	if err != nil {
		return "", fmt.Errorf("failed to get view DDL: %w", err)
	}
	return def, nil
}

func (d *Dao) GetTableColumns(ctx context.Context, schema, table string) ([]database.ColumnInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT column_name, column_type, is_nullable = 'YES',
		       column_default, column_key = 'PRI', ordinal_position, column_comment, extra
		FROM information_schema.columns
		WHERE table_schema = ? AND table_name = ?
		ORDER BY ordinal_position`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get table columns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var columns []database.ColumnInfo
	for rows.Next() {
		var col database.ColumnInfo
		var isPK, isNullable uint8
		var extra string
		if err := rows.Scan(&col.Name, &col.DataType, &isNullable,
			&col.Default, &isPK, &col.Ordinal, &col.Comment, &extra); err != nil {
			return nil, err
		}
		col.IsNullable = isNullable == 1
		col.IsPK = isPK == 1
		col.IsAutoGenerated = strings.Contains(extra, "auto_increment")
		columns = append(columns, col)
	}
	return columns, rows.Err()
}

func (d *Dao) GetTableConstraints(ctx context.Context, schema, table string) ([]database.ConstraintInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT tc.constraint_name, tc.constraint_type,
		       GROUP_CONCAT(kcu.column_name ORDER BY kcu.ordinal_position SEPARATOR ',')
		FROM information_schema.table_constraints tc
		LEFT JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
			AND tc.table_name = kcu.table_name
		WHERE tc.table_schema = ? AND tc.table_name = ?
		  AND tc.constraint_type IN ('PRIMARY KEY', 'UNIQUE')
		GROUP BY tc.constraint_name, tc.constraint_type
		ORDER BY tc.constraint_type, tc.constraint_name`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get table constraints: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var constraints []database.ConstraintInfo
	for rows.Next() {
		var c database.ConstraintInfo
		var colStr *string
		if err := rows.Scan(&c.Name, &c.Type, &colStr); err != nil {
			return nil, err
		}
		if colStr != nil && *colStr != "" {
			c.Columns = strings.Split(*colStr, ",")
		}
		constraints = append(constraints, c)
	}
	return constraints, rows.Err()
}

func (d *Dao) GetTableForeignKeys(ctx context.Context, schema, table string) ([]database.ForeignKeyInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT kcu.constraint_name,
		       GROUP_CONCAT(kcu.column_name ORDER BY kcu.ordinal_position SEPARATOR ','),
		       kcu.referenced_table_schema,
		       kcu.referenced_table_name,
		       GROUP_CONCAT(kcu.referenced_column_name ORDER BY kcu.ordinal_position SEPARATOR ','),
		       rc.update_rule, rc.delete_rule
		FROM information_schema.key_column_usage kcu
		JOIN information_schema.referential_constraints rc
			ON kcu.constraint_name = rc.constraint_name
			AND kcu.table_schema = rc.constraint_schema
			AND kcu.table_name = rc.table_name
		WHERE kcu.table_schema = ? AND kcu.table_name = ?
		  AND kcu.referenced_table_name IS NOT NULL
		GROUP BY kcu.constraint_name, kcu.referenced_table_schema,
		         kcu.referenced_table_name, rc.update_rule, rc.delete_rule
		ORDER BY kcu.constraint_name`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get foreign keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var fks []database.ForeignKeyInfo
	for rows.Next() {
		var fk database.ForeignKeyInfo
		var colStr, refColStr string
		if err := rows.Scan(&fk.Name, &colStr, &fk.ReferencedSchema,
			&fk.ReferencedTable, &refColStr, &fk.OnUpdate, &fk.OnDelete); err != nil {
			return nil, err
		}
		fk.Columns = strings.Split(colStr, ",")
		fk.ReferencedCols = strings.Split(refColStr, ",")
		fks = append(fks, fk)
	}
	return fks, rows.Err()
}

func (d *Dao) GetIncomingForeignKeys(ctx context.Context, schema, table string) ([]database.IncomingForeignKeyInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT kcu.table_schema, kcu.table_name,
		       GROUP_CONCAT(kcu.column_name ORDER BY kcu.ordinal_position SEPARATOR ','),
		       GROUP_CONCAT(kcu.referenced_column_name ORDER BY kcu.ordinal_position SEPARATOR ',')
		FROM information_schema.key_column_usage kcu
		WHERE kcu.referenced_table_schema = ? AND kcu.referenced_table_name = ?
		  AND kcu.referenced_column_name IS NOT NULL
		GROUP BY kcu.table_schema, kcu.table_name, kcu.constraint_name
		ORDER BY kcu.table_schema, kcu.table_name`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get incoming foreign keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []database.IncomingForeignKeyInfo
	for rows.Next() {
		var fk database.IncomingForeignKeyInfo
		var colStr, refColStr string
		if err := rows.Scan(&fk.Schema, &fk.Table, &colStr, &refColStr); err != nil {
			return nil, err
		}
		fk.Columns = strings.Split(colStr, ",")
		fk.ReferencedCols = strings.Split(refColStr, ",")
		result = append(result, fk)
	}
	return result, rows.Err()
}

func (d *Dao) GetEstimatedRowCount(ctx context.Context, schema, table string) (int64, bool, error) {
	var count *int64
	err := d.client.DB.QueryRowContext(ctx, `
		SELECT table_rows
		FROM information_schema.tables
		WHERE table_schema = ? AND table_name = ?`, schema, table).Scan(&count)
	if err != nil {
		return 0, false, err
	}
	if count == nil {
		return 0, false, nil
	}
	return *count, true, nil
}

func (d *Dao) FetchTableRows(ctx context.Context, state *database.TableState, where, orderBy string) (string, []database.Row, error) {
	query := fmt.Sprintf("SELECT * FROM %s", quote.Table(state.Schema, state.Table))
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
	offset := int64(state.RowCount())
	displayQuery := query + fmt.Sprintf(" LIMIT %d OFFSET %d", state.BatchSize, offset)
	query += " LIMIT ? OFFSET ?"
	args = append(args, state.BatchSize, offset)

	rows, err := d.client.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return "", nil, fmt.Errorf("failed to fetch rows: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result, err := scanRows(rows)
	if err != nil {
		return "", nil, err
	}
	return displayQuery, result, nil
}

func (d *Dao) InsertRow(ctx context.Context, schema, table string, row database.Row) (database.PrimaryKey, error) {
	log.Info().Str("schema", schema).Str("table", table).Msg("Inserting row")
	cols := make([]string, 0, len(row))
	placeholders := make([]string, 0, len(row))
	args := make([]any, 0, len(row))

	for col, val := range row {
		cols = append(cols, quote.Ident(col))
		placeholders = append(placeholders, "?")
		args = append(args, val)
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quote.Table(schema, table), strings.Join(cols, ", "), strings.Join(placeholders, ", "))

	result, err := d.client.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return database.PrimaryKey{}, fmt.Errorf("failed to insert row: %w", err)
	}

	lastID, err := result.LastInsertId()
	if err != nil {
		return database.PrimaryKey{}, nil
	}

	pkCols, err := d.getPrimaryKeyColumns(ctx, schema, table)
	if err != nil || len(pkCols) != 1 {
		return database.PrimaryKey{}, nil
	}

	return database.PrimaryKey{
		Columns: map[string]any{pkCols[0]: fmt.Sprintf("%d", lastID)},
	}, nil
}

func (d *Dao) UpdateRow(ctx context.Context, schema, table string, pk database.PrimaryKey, original, updated database.Row) error {
	log.Info().Str("schema", schema).Str("table", table).Interface("pk", pk.Columns).Msg("Updating row")
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
			setClauses = append(setClauses, fmt.Sprintf("%s = ?", quote.Ident(col)))
			args = append(args, newVal)
		}
	}

	if len(setClauses) == 0 {
		return nil
	}

	whereParts := []string{}
	for col, val := range pk.Columns {
		whereParts = append(whereParts, fmt.Sprintf("%s = ?", quote.Ident(col)))
		args = append(args, val)
	}

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		quote.Table(schema, table), strings.Join(setClauses, ", "), strings.Join(whereParts, " AND "))

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
	log.Info().Str("schema", schema).Str("table", table).Int("count", len(pks)).Msg("Deleting rows")
	for _, pk := range pks {
		whereParts, args := quote.WhereEqAnon(pk.Columns)
		query := fmt.Sprintf("DELETE FROM %s WHERE %s",
			quote.Table(schema, table), strings.Join(whereParts, " AND "))

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
		"INT",
		"BIGINT",
		"SMALLINT",
		"TINYINT",
		"DECIMAL(10,2)",
		"FLOAT",
		"DOUBLE",
		"BOOLEAN",
		"VARCHAR(255)",
		"TEXT",
		"BLOB",
		"DATE",
		"TIME",
		"DATETIME",
		"TIMESTAMP",
		"JSON",
		"CHAR(1)",
		"ENUM('a','b')",
	}
}

func (d *Dao) DefaultPKType() string { return "INT AUTO_INCREMENT" }

func (d *Dao) DefaultCreateTableDDL(schema, tableName string) string {
	return fmt.Sprintf("CREATE TABLE %s (id INT NOT NULL AUTO_INCREMENT, PRIMARY KEY (id))",
		quote.Table(schema, tableName))
}

func (d *Dao) GetTableDDL(ctx context.Context, schema, table string) (string, error) {
	var tableName, ddl string
	err := d.client.DB.QueryRowContext(ctx,
		fmt.Sprintf("SHOW CREATE TABLE %s", quote.Table(schema, table))).
		Scan(&tableName, &ddl)
	if err != nil {
		return "", fmt.Errorf("failed to get table DDL: %w", err)
	}
	return ddl, nil
}

func (d *Dao) CreateTable(ctx context.Context, schema, ddl string) error {
	if _, err := d.client.DB.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}
	log.Info().Str("schema", schema).Str("ddl", ddl).Msg("Table created")
	return nil
}

func (d *Dao) DropTable(ctx context.Context, schema, table string) error {
	_, err := d.client.DB.ExecContext(ctx,
		fmt.Sprintf("DROP TABLE IF EXISTS %s", quote.Table(schema, table)))
	if err != nil {
		return fmt.Errorf("failed to drop table: %w", err)
	}
	log.Info().Str("schema", schema).Str("table", table).Msg("Table dropped")
	return nil
}

func (d *Dao) RenameTable(ctx context.Context, schema, old, newName string) error {
	_, err := d.client.DB.ExecContext(ctx, fmt.Sprintf("RENAME TABLE %s TO %s",
		quote.Table(schema, old), quote.Table(schema, newName)))
	if err != nil {
		return fmt.Errorf("failed to rename table: %w", err)
	}
	log.Info().Str("schema", schema).Str("old", old).Str("new", newName).Msg("Table renamed")
	return nil
}

func (d *Dao) RenameColumn(ctx context.Context, schema, table, old, newName string) error {
	// RENAME COLUMN is available in MySQL 8.0+.
	_, err := d.client.DB.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s",
		quote.Table(schema, table), quote.Ident(old), quote.Ident(newName)))
	if err != nil {
		return fmt.Errorf("failed to rename column: %w", err)
	}
	log.Info().Str("table", table).Str("old", old).Str("new", newName).Msg("Column renamed")
	return nil
}

func (d *Dao) TruncateTable(ctx context.Context, schema, table string) error {
	_, err := d.client.DB.ExecContext(ctx,
		fmt.Sprintf("TRUNCATE TABLE %s", quote.Table(schema, table)))
	if err != nil {
		return fmt.Errorf("failed to truncate table: %w", err)
	}
	log.Info().Str("schema", schema).Str("table", table).Msg("Table truncated")
	return nil
}

func (d *Dao) GetIndexes(ctx context.Context, schema, table string) ([]database.IndexInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT index_name, non_unique, index_type,
		       GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',')
		FROM information_schema.statistics
		WHERE table_schema = ? AND table_name = ?
		GROUP BY index_name, non_unique, index_type
		ORDER BY index_name = 'PRIMARY' DESC, index_name`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get indexes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var indexes []database.IndexInfo
	for rows.Next() {
		var name, indexType, colStr string
		var nonUnique int
		if err := rows.Scan(&name, &nonUnique, &indexType, &colStr); err != nil {
			return nil, err
		}
		indexes = append(indexes, database.IndexInfo{
			Name:      name,
			Columns:   strings.Split(colStr, ","),
			IsUnique:  nonUnique == 0,
			IsPrimary: name == "PRIMARY",
			Type:      strings.ToLower(indexType),
		})
	}
	return indexes, rows.Err()
}

func (d *Dao) CreateIndex(ctx context.Context, schema, table string, def database.IndexDefinition) error {
	uniqueStr := ""
	if def.IsUnique {
		uniqueStr = "UNIQUE "
	}

	quotedCols := make([]string, len(def.Columns))
	for i, c := range def.Columns {
		parts := strings.Fields(c)
		quotedCols[i] = quote.Ident(parts[0])
		if len(parts) > 1 {
			quotedCols[i] += " " + parts[1]
		}
	}

	query := fmt.Sprintf("CREATE %sINDEX %s ON %s (%s)",
		uniqueStr, quote.Ident(def.Name), quote.Table(schema, table),
		strings.Join(quotedCols, ", "))

	if _, err := d.client.DB.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}
	log.Info().Str("schema", schema).Str("table", table).Str("index", def.Name).Msg("Index created")
	return nil
}

func (d *Dao) DropIndex(ctx context.Context, schema, indexName string) error {
	// MySQL requires the table name in DROP INDEX; look it up from information_schema.
	var tableName string
	err := d.client.DB.QueryRowContext(ctx, `
		SELECT table_name FROM information_schema.statistics
		WHERE table_schema = ? AND index_name = ? LIMIT 1`, schema, indexName).Scan(&tableName)
	if err != nil {
		return fmt.Errorf("failed to locate index %q: %w", indexName, err)
	}
	_, err = d.client.DB.ExecContext(ctx,
		fmt.Sprintf("DROP INDEX %s ON %s", quote.Ident(indexName), quote.Table(schema, tableName)))
	if err != nil {
		return fmt.Errorf("failed to drop index: %w", err)
	}
	log.Info().Str("schema", schema).Str("index", indexName).Msg("Index dropped")
	return nil
}

func (d *Dao) FetchQueryRows(ctx context.Context, rawSQL string, limit, offset int64) (string, []database.Row, []database.ColumnInfo, error) {
	bypassSubquery := sqlpkg.IsExplainQuery(rawSQL) || sqlpkg.IsReturningDML(rawSQL)

	var displayQuery, paramQuery string
	if bypassSubquery {
		displayQuery = rawSQL
		paramQuery = rawSQL
	} else {
		displayQuery = fmt.Sprintf("SELECT * FROM (%s) AS _q LIMIT %d OFFSET %d", rawSQL, limit, offset)
		paramQuery = fmt.Sprintf("SELECT * FROM (%s) AS _q LIMIT ? OFFSET ?", rawSQL)
	}

	var sqlRows *sql.Rows
	var err error
	if bypassSubquery {
		sqlRows, err = d.client.DB.QueryContext(ctx, paramQuery)
	} else {
		sqlRows, err = d.client.DB.QueryContext(ctx, paramQuery, limit, offset)
	}
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer func() { _ = sqlRows.Close() }()

	colTypes, err := sqlRows.ColumnTypes()
	if err != nil {
		return "", nil, nil, err
	}
	cols := make([]database.ColumnInfo, len(colTypes))
	for i, ct := range colTypes {
		cols[i] = database.ColumnInfo{Name: ct.Name(), DataType: strings.ToLower(ct.DatabaseTypeName()), Ordinal: i + 1}
	}

	result, err := scanRows(sqlRows)
	if err != nil {
		return "", nil, nil, err
	}
	return displayQuery, result, cols, nil
}

func (d *Dao) ExecuteQuery(ctx context.Context, query string) ([]database.Row, []database.ColumnInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer func() { _ = rows.Close() }()

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
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}
	log.Info().Int64("rows_affected", affected).Msg("Statement executed")
	return affected, nil
}

// ExplainPlan uses EXPLAIN FORMAT=TREE (MySQL 8.0.16+) which returns a
// human-readable tree as a single TEXT value.
func (d *Dao) ExplainPlan(ctx context.Context, sqlStr string) (string, error) {
	return d.explainText(ctx, "EXPLAIN FORMAT=TREE "+sqlStr)
}

// ExplainAnalyze uses EXPLAIN ANALYZE (MySQL 8.0.18+) which includes actual
// execution statistics in the same tree format.
func (d *Dao) ExplainAnalyze(ctx context.Context, sqlStr string) (string, error) {
	return d.explainText(ctx, "EXPLAIN ANALYZE "+sqlStr)
}

func (d *Dao) explainText(ctx context.Context, query string) (string, error) {
	var result string
	if err := d.client.DB.QueryRowContext(ctx, query).Scan(&result); err != nil {
		return "", fmt.Errorf("failed to explain query: %w", err)
	}
	return result, nil
}

func (d *Dao) GetTableColumnNames(ctx context.Context, schema, table string) ([]string, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = ? AND table_name = ?
		ORDER BY ordinal_position`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get column names: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// getPrimaryKeyColumns returns the PK column names for a table.
func (d *Dao) getPrimaryKeyColumns(ctx context.Context, schema, table string) ([]string, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT column_name
		FROM information_schema.key_column_usage
		WHERE table_schema = ? AND table_name = ? AND constraint_name = 'PRIMARY'
		ORDER BY ordinal_position`, schema, table)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
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
			case time.Time:
				row[col] = v.Format(time.RFC3339Nano)
			default:
				row[col] = fmt.Sprint(v)
			}
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
