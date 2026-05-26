package sqlserver

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

const quote = util.BracketQuoter

// Dao implements database.Driver for SQL Server.
type Dao struct {
	client *Client
}

func NewDao(client *Client) *Dao {
	return &Dao{client: client}
}

func (d *Dao) Connect(ctx context.Context) error {
	return d.client.Connect(ctx)
}

func (d *Dao) Close(_ context.Context) error {
	d.client.Close()
	return nil
}

func (d *Dao) Ping(ctx context.Context) error {
	return d.client.Ping(ctx)
}

func (d *Dao) GetServerInfo(ctx context.Context) (*database.ServerInfo, error) {
	info := &database.ServerInfo{Extra: make(map[string]string)}

	var version, dbName string
	if err := d.client.DB.QueryRowContext(ctx,
		"SELECT @@VERSION, DB_NAME()").
		Scan(&version, &dbName); err != nil {
		return nil, fmt.Errorf("failed to get server info: %w", err)
	}
	// @@VERSION returns a multi-line string; use first line as version summary.
	if idx := strings.IndexByte(version, '\n'); idx > 0 {
		version = strings.TrimSpace(version[:idx])
	}
	info.Version = version
	info.CurrentDB = dbName

	var host string
	if err := d.client.DB.QueryRowContext(ctx, "SELECT @@SERVERNAME").Scan(&host); err == nil {
		info.Host = host
	}

	var maxConns int64
	if err := d.client.DB.QueryRowContext(ctx,
		"SELECT value_in_use FROM sys.configurations WHERE name = 'max connections'").
		Scan(&maxConns); err == nil {
		info.MaxConnections = maxConns
	}

	var sessions int64
	if err := d.client.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sys.dm_exec_sessions WHERE is_user_process = 1").
		Scan(&sessions); err == nil {
		info.ActiveSessions = sessions
	}

	var dbSizeMB float64
	if err := d.client.DB.QueryRowContext(ctx, `
		SELECT SUM(size * 8.0 / 1024)
		FROM sys.database_files
		WHERE type_desc = 'ROWS'`).Scan(&dbSizeMB); err == nil {
		info.DatabaseSize = fmt.Sprintf("%.2f MiB", dbSizeMB)
	}

	return info, nil
}

func (d *Dao) GetActiveSessions(ctx context.Context) (int64, error) {
	var count int64
	if err := d.client.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sys.dm_exec_sessions WHERE is_user_process = 1").
		Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to get active sessions: %w", err)
	}
	return count, nil
}

func (d *Dao) ListSchemas(ctx context.Context, nameFilter string) ([]database.Schema, error) {
	// System schemas we never want to surface.
	systemSchemas := `'sys','INFORMATION_SCHEMA','guest',
		'db_accessadmin','db_backupoperator','db_datareader','db_datawriter',
		'db_ddladmin','db_denydatareader','db_denydatawriter','db_owner','db_securityadmin'`

	query := fmt.Sprintf(`
		SELECT s.name,
		       STRING_AGG(t.name, ',') WITHIN GROUP (ORDER BY t.name)
		FROM sys.schemas s
		LEFT JOIN sys.tables t ON s.schema_id = t.schema_id
		WHERE s.name NOT IN (%s)`, systemSchemas)

	args := []any{}
	if nameFilter != "" {
		query += " AND (s.name LIKE @p1 OR t.name LIKE @p2)"
		args = append(args, "%"+nameFilter+"%", "%"+nameFilter+"%")
	}
	query += " GROUP BY s.name ORDER BY s.name"

	rows, err := d.client.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []database.Schema
	for rows.Next() {
		var schemaName string
		var tableStr *string
		if err := rows.Scan(&schemaName, &tableStr); err != nil {
			return nil, err
		}
		var tables []string
		if tableStr != nil && *tableStr != "" {
			tables = strings.Split(*tableStr, ",")
		}
		result = append(result, database.Schema{Schema: schemaName, Tables: tables})
	}
	return result, rows.Err()
}

func (d *Dao) GetTableColumns(ctx context.Context, schema, table string) ([]database.ColumnInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT c.name,
		       CASE WHEN tp.name IN ('nvarchar','nchar') THEN
		                tp.name + '(' + CASE WHEN c.max_length = -1 THEN 'MAX' ELSE CAST(c.max_length/2 AS VARCHAR) END + ')'
		            WHEN tp.name IN ('varchar','char','binary','varbinary') THEN
		                tp.name + '(' + CASE WHEN c.max_length = -1 THEN 'MAX' ELSE CAST(c.max_length AS VARCHAR) END + ')'
		            WHEN tp.name IN ('decimal','numeric') THEN
		                tp.name + '(' + CAST(c.precision AS VARCHAR) + ',' + CAST(c.scale AS VARCHAR) + ')'
		            ELSE tp.name
		       END AS column_type,
		       c.is_nullable,
		       dc.definition AS column_default,
		       CAST(CASE WHEN EXISTS (
		           SELECT 1 FROM sys.index_columns ic
		           JOIN sys.indexes i ON ic.object_id = i.object_id AND ic.index_id = i.index_id
		           WHERE ic.object_id = c.object_id AND ic.column_id = c.column_id AND i.is_primary_key = 1
		       ) THEN 1 ELSE 0 END AS BIT) AS is_pk,
		       c.column_id,
		       ''
		FROM sys.columns c
		JOIN sys.tables t ON c.object_id = t.object_id
		JOIN sys.schemas s ON t.schema_id = s.schema_id
		JOIN sys.types tp ON c.user_type_id = tp.user_type_id
		LEFT JOIN sys.default_constraints dc ON c.object_id = dc.parent_object_id AND c.column_id = dc.parent_column_id
		WHERE s.name = @p1 AND t.name = @p2
		ORDER BY c.column_id`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get table columns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var columns []database.ColumnInfo
	for rows.Next() {
		var col database.ColumnInfo
		var isNullable, isPK bool
		var defaultVal sql.NullString
		if err := rows.Scan(&col.Name, &col.DataType, &isNullable,
			&defaultVal, &isPK, &col.Ordinal, &col.Comment); err != nil {
			return nil, err
		}
		col.IsNullable = isNullable
		col.IsPK = isPK
		if defaultVal.Valid {
			col.Default = &defaultVal.String
		}
		columns = append(columns, col)
	}
	return columns, rows.Err()
}

func (d *Dao) GetTableConstraints(ctx context.Context, schema, table string) ([]database.ConstraintInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT tc.name AS constraint_name,
		       CASE tc.type WHEN 'PK' THEN 'PRIMARY KEY' WHEN 'UQ' THEN 'UNIQUE' ELSE tc.type END,
		       STRING_AGG(c.name, ',') WITHIN GROUP (ORDER BY ic.key_ordinal)
		FROM sys.key_constraints tc
		JOIN sys.index_columns ic ON tc.parent_object_id = ic.object_id AND tc.unique_index_id = ic.index_id
		JOIN sys.columns c ON ic.object_id = c.object_id AND ic.column_id = c.column_id
		JOIN sys.tables t ON tc.parent_object_id = t.object_id
		JOIN sys.schemas s ON t.schema_id = s.schema_id
		WHERE s.name = @p1 AND t.name = @p2 AND tc.type IN ('PK','UQ')
		GROUP BY tc.name, tc.type
		ORDER BY tc.type, tc.name`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get table constraints: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var constraints []database.ConstraintInfo
	for rows.Next() {
		var c database.ConstraintInfo
		var colStr string
		if err := rows.Scan(&c.Name, &c.Type, &colStr); err != nil {
			return nil, err
		}
		if colStr != "" {
			c.Columns = strings.Split(colStr, ",")
		}
		constraints = append(constraints, c)
	}
	return constraints, rows.Err()
}

func (d *Dao) GetTableForeignKeys(ctx context.Context, schema, table string) ([]database.ForeignKeyInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT fk.name,
		       STRING_AGG(c.name, ',') WITHIN GROUP (ORDER BY fkc.constraint_column_id),
		       rs.name AS ref_schema,
		       rt.name AS ref_table,
		       STRING_AGG(rc.name, ',') WITHIN GROUP (ORDER BY fkc.constraint_column_id),
		       fk.update_referential_action_desc,
		       fk.delete_referential_action_desc
		FROM sys.foreign_keys fk
		JOIN sys.foreign_key_columns fkc ON fk.object_id = fkc.constraint_object_id
		JOIN sys.columns c  ON fkc.parent_object_id  = c.object_id  AND fkc.parent_column_id  = c.column_id
		JOIN sys.columns rc ON fkc.referenced_object_id = rc.object_id AND fkc.referenced_column_id = rc.column_id
		JOIN sys.tables t   ON fk.parent_object_id    = t.object_id
		JOIN sys.schemas s  ON t.schema_id            = s.schema_id
		JOIN sys.tables rt  ON fk.referenced_object_id = rt.object_id
		JOIN sys.schemas rs ON rt.schema_id            = rs.schema_id
		WHERE s.name = @p1 AND t.name = @p2
		GROUP BY fk.name, rs.name, rt.name, fk.update_referential_action_desc, fk.delete_referential_action_desc
		ORDER BY fk.name`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get foreign keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var fks []database.ForeignKeyInfo
	for rows.Next() {
		var fk database.ForeignKeyInfo
		var colStr, refColStr string
		if err := rows.Scan(&fk.Name, &colStr, &fk.ReferencedSchema, &fk.ReferencedTable,
			&refColStr, &fk.OnUpdate, &fk.OnDelete); err != nil {
			return nil, err
		}
		fk.Columns = strings.Split(colStr, ",")
		fk.ReferencedCols = strings.Split(refColStr, ",")
		// SQL Server uses NO_ACTION/CASCADE/SET_NULL/SET_DEFAULT — normalise to SQL style.
		fk.OnUpdate = strings.ReplaceAll(fk.OnUpdate, "_", " ")
		fk.OnDelete = strings.ReplaceAll(fk.OnDelete, "_", " ")
		fks = append(fks, fk)
	}
	return fks, rows.Err()
}

func (d *Dao) GetIncomingForeignKeys(ctx context.Context, schema, table string) ([]database.IncomingForeignKeyInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT ps.name, pt.name,
		       STRING_AGG(pc.name, ',') WITHIN GROUP (ORDER BY fkc.constraint_column_id),
		       STRING_AGG(rc.name, ',') WITHIN GROUP (ORDER BY fkc.constraint_column_id)
		FROM sys.foreign_keys fk
		JOIN sys.foreign_key_columns fkc ON fk.object_id = fkc.constraint_object_id
		JOIN sys.columns pc ON fkc.parent_object_id     = pc.object_id AND fkc.parent_column_id     = pc.column_id
		JOIN sys.columns rc ON fkc.referenced_object_id = rc.object_id AND fkc.referenced_column_id = rc.column_id
		JOIN sys.tables pt  ON fk.parent_object_id      = pt.object_id
		JOIN sys.schemas ps ON pt.schema_id              = ps.schema_id
		JOIN sys.tables rt  ON fk.referenced_object_id  = rt.object_id
		JOIN sys.schemas rs ON rt.schema_id              = rs.schema_id
		WHERE rs.name = @p1 AND rt.name = @p2
		GROUP BY fk.name, ps.name, pt.name
		ORDER BY ps.name, pt.name`, schema, table)
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
	// sys.partitions gives a fast row-count estimate without a full table scan.
	var count int64
	err := d.client.DB.QueryRowContext(ctx, `
		SELECT SUM(p.rows)
		FROM sys.partitions p
		JOIN sys.tables t  ON p.object_id = t.object_id
		JOIN sys.schemas s ON t.schema_id  = s.schema_id
		WHERE s.name = @p1 AND t.name = @p2 AND p.index_id IN (0, 1)`, schema, table).Scan(&count)
	if err != nil {
		return 0, false, err
	}
	return count, true, nil
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

	// SQL Server requires ORDER BY for OFFSET/FETCH; fall back to (SELECT NULL) when none given.
	order := orderBy
	if order == "" {
		order = "(SELECT NULL)"
	}
	query += " ORDER BY " + order

	offset := int64(state.RowCount())
	// go-mssqldb does not substitute ? in OFFSET/FETCH; inline as integer literals (safe: internal int64 values).
	query += fmt.Sprintf(" OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", offset, state.BatchSize)
	displayQuery := query

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

	pkCols, err := d.getPrimaryKeyColumns(ctx, schema, table)
	if err != nil {
		return database.PrimaryKey{}, fmt.Errorf("failed to get PK columns: %w", err)
	}

	cols := make([]string, 0, len(row))
	placeholders := make([]string, 0, len(row))
	args := make([]any, 0, len(row))
	i := 1
	for col, val := range row {
		cols = append(cols, quote.Ident(col))
		placeholders = append(placeholders, fmt.Sprintf("@p%d", i))
		args = append(args, val)
		i++
	}

	// Use OUTPUT INSERTED to retrieve the generated PK without relying on LastInsertId.
	if len(pkCols) == 1 {
		query := fmt.Sprintf(
			"INSERT INTO %s (%s) OUTPUT INSERTED.%s VALUES (%s)",
			quote.Table(schema, table),
			strings.Join(cols, ", "),
			quote.Ident(pkCols[0]),
			strings.Join(placeholders, ", "),
		)
		var id any
		if err := d.client.DB.QueryRowContext(ctx, query, args...).Scan(&id); err != nil {
			return database.PrimaryKey{}, fmt.Errorf("failed to insert row: %w", err)
		}
		return database.PrimaryKey{Columns: map[string]any{pkCols[0]: fmt.Sprint(id)}}, nil
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quote.Table(schema, table), strings.Join(cols, ", "), strings.Join(placeholders, ", "))
	if _, err := d.client.DB.ExecContext(ctx, query, args...); err != nil {
		return database.PrimaryKey{}, fmt.Errorf("failed to insert row: %w", err)
	}
	return database.PrimaryKey{}, nil
}

func (d *Dao) UpdateRow(ctx context.Context, schema, table string, pk database.PrimaryKey, original, updated database.Row) error {
	log.Info().Str("schema", schema).Str("table", table).Interface("pk", pk.Columns).Msg("Updating row")

	pkSet := make(map[string]bool, len(pk.Columns))
	for col := range pk.Columns {
		pkSet[col] = true
	}

	setClauses := []string{}
	args := []any{}
	idx := 1
	for col, newVal := range updated {
		if col == "_pk" || pkSet[col] {
			continue
		}
		oldVal, exists := original[col]
		if !exists || fmt.Sprint(oldVal) != fmt.Sprint(newVal) {
			setClauses = append(setClauses, fmt.Sprintf("%s = @p%d", quote.Ident(col), idx))
			args = append(args, newVal)
			idx++
		}
	}
	if len(setClauses) == 0 {
		return nil
	}

	whereParts, pkArgs, _ := quote.WhereEqMSSql(pk.Columns, idx)
	args = append(args, pkArgs...)

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
		whereParts, args, _ := quote.WhereEqMSSql(pk.Columns, 1)
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
		"REAL",
		"BIT",
		"NVARCHAR(255)",
		"NVARCHAR(MAX)",
		"VARCHAR(255)",
		"NCHAR(1)",
		"DATE",
		"TIME",
		"DATETIME2",
		"DATETIMEOFFSET",
		"UNIQUEIDENTIFIER",
		"XML",
		"VARBINARY(MAX)",
	}
}

func (d *Dao) DefaultCreateTableDDL(schema, tableName string) string {
	return fmt.Sprintf(
		"CREATE TABLE %s ([id] INT NOT NULL IDENTITY(1,1), PRIMARY KEY ([id]))",
		quote.Table(schema, tableName),
	)
}

func (d *Dao) GetTableDDL(ctx context.Context, schema, table string) (string, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT c.name,
		       CASE WHEN tp.name IN ('nvarchar','nchar') THEN
		                tp.name + '(' + CASE WHEN c.max_length = -1 THEN 'MAX' ELSE CAST(c.max_length/2 AS VARCHAR) END + ')'
		            WHEN tp.name IN ('varchar','char','binary','varbinary') THEN
		                tp.name + '(' + CASE WHEN c.max_length = -1 THEN 'MAX' ELSE CAST(c.max_length AS VARCHAR) END + ')'
		            WHEN tp.name IN ('decimal','numeric') THEN
		                tp.name + '(' + CAST(c.precision AS VARCHAR) + ',' + CAST(c.scale AS VARCHAR) + ')'
		            ELSE tp.name
		       END,
		       c.is_nullable,
		       c.is_identity
		FROM sys.columns c
		JOIN sys.tables t  ON c.object_id = t.object_id
		JOIN sys.schemas s ON t.schema_id  = s.schema_id
		JOIN sys.types tp  ON c.user_type_id = tp.user_type_id
		WHERE s.name = @p1 AND t.name = @p2
		ORDER BY c.column_id`, schema, table)
	if err != nil {
		return "", fmt.Errorf("failed to get table DDL: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sb strings.Builder
	fmt.Fprintf(&sb, "CREATE TABLE %s (\n", quote.Table(schema, table))

	first := true
	for rows.Next() {
		var name, typeName string
		var isNullable, isIdentity bool
		if err := rows.Scan(&name, &typeName, &isNullable, &isIdentity); err != nil {
			return "", err
		}
		if !first {
			sb.WriteString(",\n")
		}
		first = false

		nullStr := " NOT NULL"
		if isNullable {
			nullStr = " NULL"
		}
		identStr := ""
		if isIdentity {
			identStr = " IDENTITY(1,1)"
		}
		fmt.Fprintf(&sb, "    %s %s%s%s", quote.Ident(name), typeName, identStr, nullStr)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	// Append PRIMARY KEY constraint.
	pkRows, err := d.client.DB.QueryContext(ctx, `
		SELECT kc.name,
		       STRING_AGG(c.name, ', ') WITHIN GROUP (ORDER BY ic.key_ordinal)
		FROM sys.key_constraints kc
		JOIN sys.index_columns ic ON kc.parent_object_id = ic.object_id AND kc.unique_index_id = ic.index_id
		JOIN sys.columns c        ON ic.object_id = c.object_id AND ic.column_id = c.column_id
		JOIN sys.tables t         ON kc.parent_object_id = t.object_id
		JOIN sys.schemas s        ON t.schema_id = s.schema_id
		WHERE s.name = @p1 AND t.name = @p2 AND kc.type = 'PK'
		GROUP BY kc.name`, schema, table)
	if err == nil {
		defer func() { _ = pkRows.Close() }()
		if pkRows.Next() {
			var pkName, pkCols string
			if pkRows.Scan(&pkName, &pkCols) == nil {
				quotedCols := make([]string, 0)
				for col := range strings.SplitSeq(pkCols, ", ") {
					quotedCols = append(quotedCols, quote.Ident(col))
				}
				fmt.Fprintf(&sb, ",\n    CONSTRAINT %s PRIMARY KEY (%s)",
					quote.Ident(pkName), strings.Join(quotedCols, ", "))
			}
		}
	}

	sb.WriteString("\n)")
	return sb.String(), nil
}

func (d *Dao) CreateTable(ctx context.Context, _ string, ddl string) error {
	if _, err := d.client.DB.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}
	log.Info().Str("ddl", ddl).Msg("Table created")
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
	// sp_rename takes 'schema.old' as first arg; new name is unqualified.
	_, err := d.client.DB.ExecContext(ctx,
		"EXEC sp_rename ?, ?",
		schema+"."+old, newName)
	if err != nil {
		return fmt.Errorf("failed to rename table: %w", err)
	}
	log.Info().Str("schema", schema).Str("old", old).Str("new", newName).Msg("Table renamed")
	return nil
}

func (d *Dao) RenameColumn(ctx context.Context, schema, table, old, newName string) error {
	// sp_rename with 'COLUMN' type renames a column.
	_, err := d.client.DB.ExecContext(ctx,
		"EXEC sp_rename ?, ?, 'COLUMN'",
		schema+"."+table+"."+old, newName)
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
		SELECT i.name,
		       i.is_unique,
		       i.is_primary_key,
		       i.type_desc,
		       STRING_AGG(c.name, ',') WITHIN GROUP (ORDER BY ic.key_ordinal)
		FROM sys.indexes i
		JOIN sys.index_columns ic ON i.object_id = ic.object_id AND i.index_id = ic.index_id
		JOIN sys.columns c        ON ic.object_id = c.object_id AND ic.column_id = c.column_id
		JOIN sys.tables t         ON i.object_id = t.object_id
		JOIN sys.schemas s        ON t.schema_id = s.schema_id
		WHERE s.name = @p1 AND t.name = @p2 AND i.is_hypothetical = 0 AND ic.is_included_column = 0
		GROUP BY i.name, i.is_unique, i.is_primary_key, i.type_desc
		ORDER BY i.is_primary_key DESC, i.name`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get indexes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var indexes []database.IndexInfo
	for rows.Next() {
		var name, typeDesc, colStr string
		var isUnique, isPrimary bool
		if err := rows.Scan(&name, &isUnique, &isPrimary, &typeDesc, &colStr); err != nil {
			return nil, err
		}
		indexes = append(indexes, database.IndexInfo{
			Name:      name,
			Columns:   strings.Split(colStr, ","),
			IsUnique:  isUnique,
			IsPrimary: isPrimary,
			Type:      strings.ToLower(typeDesc),
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
	// SQL Server DROP INDEX requires the table name: DROP INDEX idx ON table.
	var tableName string
	err := d.client.DB.QueryRowContext(ctx, `
		SELECT t.name FROM sys.indexes i
		JOIN sys.tables t  ON i.object_id = t.object_id
		JOIN sys.schemas s ON t.schema_id  = s.schema_id
		WHERE s.name = @p1 AND i.name = @p2`, schema, indexName).Scan(&tableName)
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

	var query string
	if bypassSubquery {
		query = rawSQL
	} else {
		// go-mssqldb does not substitute ? in OFFSET/FETCH; inline as integer literals (safe: internal int64 values).
		// SQL Server requires ORDER BY for OFFSET/FETCH; (SELECT NULL) satisfies the syntax without imposing an order.
		query = fmt.Sprintf(
			"SELECT * FROM (%s) AS _q ORDER BY (SELECT NULL) OFFSET %d ROWS FETCH NEXT %d ROWS ONLY",
			rawSQL, offset, limit)
	}
	displayQuery := query

	sqlRows, err := d.client.DB.QueryContext(ctx, query)
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

// ExplainPlan uses SET SHOWPLAN_TEXT ON which shows the estimated query plan
// without executing the query.
func (d *Dao) ExplainPlan(ctx context.Context, sqlStr string) (string, error) {
	return d.showplan(ctx, "SET SHOWPLAN_TEXT ON", "SET SHOWPLAN_TEXT OFF", sqlStr)
}

// ExplainAnalyze uses SET SHOWPLAN_ALL ON which returns an extended estimated
// plan with cost information. SQL Server's equivalent of an actual-execution
// plan requires running the query and is not exposed via database/sql cleanly.
func (d *Dao) ExplainAnalyze(ctx context.Context, sqlStr string) (string, error) {
	return d.showplan(ctx, "SET SHOWPLAN_ALL ON", "SET SHOWPLAN_ALL OFF", sqlStr)
}

// showplan runs setOn on a dedicated connection, executes sqlStr (which
// returns the plan rather than results), collects the first column of every
// row across all result sets, then runs setOff.
func (d *Dao) showplan(ctx context.Context, setOn, setOff, sqlStr string) (string, error) {
	conn, err := d.client.DB.Conn(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get connection for plan: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, setOn); err != nil {
		return "", fmt.Errorf("%s failed: %w", setOn, err)
	}
	defer conn.ExecContext(context.Background(), setOff) //nolint:errcheck

	rows, err := conn.QueryContext(ctx, sqlStr)
	if err != nil {
		return "", fmt.Errorf("failed to get query plan: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out strings.Builder
	for {
		for rows.Next() {
			cols, _ := rows.Columns()
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				return "", err
			}
			if len(vals) > 0 {
				if s, ok := vals[0].(string); ok {
					out.WriteString(s)
					out.WriteString("\n")
				}
			}
		}
		if !rows.NextResultSet() {
			break
		}
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

func (d *Dao) GetTableColumnNames(ctx context.Context, schema, table string) ([]string, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT c.name
		FROM sys.columns c
		JOIN sys.tables t  ON c.object_id = t.object_id
		JOIN sys.schemas s ON t.schema_id  = s.schema_id
		WHERE s.name = @p1 AND t.name = @p2
		ORDER BY c.column_id`, schema, table)
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

func (d *Dao) getPrimaryKeyColumns(ctx context.Context, schema, table string) ([]string, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT c.name
		FROM sys.key_constraints kc
		JOIN sys.index_columns ic ON kc.parent_object_id = ic.object_id AND kc.unique_index_id = ic.index_id
		JOIN sys.columns c        ON ic.object_id = c.object_id AND ic.column_id = c.column_id
		JOIN sys.tables t         ON kc.parent_object_id = t.object_id
		JOIN sys.schemas s        ON t.schema_id = s.schema_id
		WHERE s.name = @p1 AND t.name = @p2 AND kc.type = 'PK'
		ORDER BY ic.key_ordinal`, schema, table)
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

	// go-mssqldb returns UNIQUEIDENTIFIER as raw []byte (16 bytes, mixed-endian).
	// Track which columns need UUID formatting to avoid treating them as strings.
	colTypes, _ := rows.ColumnTypes()
	isUUID := make([]bool, len(cols))
	for i, ct := range colTypes {
		isUUID[i] = ct.DatabaseTypeName() == "UNIQUEIDENTIFIER"
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
				if isUUID[i] && len(v) == 16 {
					row[col] = formatUUID(v)
				} else {
					row[col] = string(v)
				}
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

// formatUUID converts a 16-byte UNIQUEIDENTIFIER from go-mssqldb's mixed-endian
// representation to the standard UUID string format (xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx).
// SQL Server stores the first three groups little-endian and the last two big-endian.
func formatUUID(b []byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uint32(b[3])<<24|uint32(b[2])<<16|uint32(b[1])<<8|uint32(b[0]),
		uint16(b[5])<<8|uint16(b[4]),
		uint16(b[7])<<8|uint16(b[6]),
		b[8:10],
		b[10:16])
}
