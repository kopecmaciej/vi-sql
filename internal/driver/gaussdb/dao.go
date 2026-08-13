package gaussdb

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	"github.com/kopecmaciej/vi-sql/internal/database"
	sqlpkg "github.com/kopecmaciej/vi-sql/internal/sql"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog/log"
)

type Dao struct {
	client *Client
}

func NewDao(client *Client) *Dao {
	return &Dao{client: client}
}

func (d *Dao) Connect(ctx context.Context) error {
	return d.client.Connect()
}

func (d *Dao) Close(ctx context.Context) error {
	d.client.Close()
	return nil
}

func (d *Dao) Ping(ctx context.Context) error {
	return d.client.Ping()
}

func (d *Dao) GetServerInfo(ctx context.Context) (*database.ServerInfo, error) {
	host := d.client.Config.Host
	port := d.client.Config.Port
	info := &database.ServerInfo{
		Host:  host,
		Port:  port,
		Extra: make(map[string]string),
	}

	var version string
	err := d.client.DB.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return nil, fmt.Errorf("failed to get server version: %w", err)
	}
	info.Version = version

	var uptime string
	err = d.client.DB.QueryRowContext(ctx,
		"SELECT (now() - pg_postmaster_start_time())::text").Scan(&uptime)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get server uptime")
	} else {
		info.Uptime = uptime
	}

	var sessions int64
	err = d.client.DB.QueryRowContext(ctx,
		"SELECT count(*) FROM pg_stat_activity WHERE state IS NOT NULL").Scan(&sessions)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get active sessions")
	} else {
		info.ActiveSessions = sessions
	}

	info.CurrentDB = d.client.Config.Database

	var maxConns int64
	err = d.client.DB.QueryRowContext(ctx,
		"SELECT setting::int FROM pg_settings WHERE name = 'max_connections'").Scan(&maxConns)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get max connections")
	} else {
		info.MaxConnections = maxConns
	}

	var dbSize string
	err = d.client.DB.QueryRowContext(ctx,
		"SELECT pg_size_pretty(pg_database_size(current_database()))").Scan(&dbSize)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get database size")
	} else {
		info.DatabaseSize = dbSize
	}

	var cacheHit string
	err = d.client.DB.QueryRowContext(ctx, `
		SELECT to_char(round(blks_hit::numeric / nullif(blks_hit + blks_read, 0) * 100, 1), 'FM990.0') || '%'
		FROM pg_stat_database WHERE datname = current_database()`).Scan(&cacheHit)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get cache hit ratio")
	} else {
		info.CacheHitRatio = cacheHit
	}

	var sslActive bool
	var sslVersion, sslCipher string
	err = d.client.DB.QueryRowContext(ctx,
		"SELECT ssl, version, cipher FROM pg_stat_ssl WHERE pid = pg_backend_pid()").
		Scan(&sslActive, &sslVersion, &sslCipher)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get SSL status")
	} else if sslActive {
		info.TLS = sslVersion + " (" + sslCipher + ")"
	}

	return info, nil
}

func (d *Dao) GetActiveSessions(ctx context.Context) (int64, error) {
	var count int64
	err := d.client.DB.QueryRowContext(ctx,
		"SELECT count(*) FROM pg_stat_activity WHERE state IS NOT NULL").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get active sessions: %w", err)
	}
	return count, nil
}

func (d *Dao) ListSchemas(ctx context.Context, nameFilter string) ([]database.Schema, error) {
	query := `
		SELECT schema_name
		FROM information_schema.schemata
		WHERE schema_name NOT IN ('information_schema', 'pg_catalog', 'pg_toast')
			AND schema_name NOT LIKE 'pg\_temp\_%'
			AND schema_name NOT LIKE 'pg\_toast\_temp\_%'
			AND schema_name NOT IN ('blockchain', 'db4ai', 'dbe_pldebugger', 'snapshot', 'sqladvisor', 'sys')
	`
	args := []any{}
	argIdx := 1

	if nameFilter != "" {
		query += fmt.Sprintf(` AND LOWER(schema_name) LIKE LOWER($%d)`, argIdx)
		args = append(args, "%"+nameFilter+"%")
	}

	query += ` ORDER BY schema_name`

	rows, err := d.client.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}
	defer rows.Close()

	var result []database.Schema
	for rows.Next() {
		var schema string
		if err := rows.Scan(&schema); err != nil {
			return nil, fmt.Errorf("failed to scan schema row: %w", err)
		}
		result = append(result, database.Schema{
			Schema: schema,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	tableQuery := `
		SELECT table_name, table_type
		FROM information_schema.tables
		WHERE table_schema = $1 AND table_type IN ('BASE TABLE', 'VIEW')
		ORDER BY table_name
	`
	for i := range result {
		tableRows, err := d.client.DB.QueryContext(ctx, tableQuery, result[i].Schema)
		if err != nil {
			return nil, fmt.Errorf("failed to list tables for schema %s: %w", result[i].Schema, err)
		}
		for tableRows.Next() {
			var tableName, tableType string
			if err := tableRows.Scan(&tableName, &tableType); err != nil {
				tableRows.Close()
				return nil, fmt.Errorf("failed to scan table row: %w", err)
			}
			if tableType == "BASE TABLE" {
				result[i].Tables = append(result[i].Tables, tableName)
			} else if tableType == "VIEW" {
				result[i].Views = append(result[i].Views, tableName)
			}
		}
		tableRows.Close()
		if err := tableRows.Err(); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (d *Dao) GetViewDDL(ctx context.Context, schema, view string) (string, error) {
	var def string
	fqTable := util.BacktickQuoter.Ident(schema) + "." + util.BacktickQuoter.Ident(view)
	err := d.client.DB.QueryRowContext(ctx,
		"SELECT pg_get_viewdef($1::regclass, true)", fqTable).Scan(&def)
	if err != nil {
		return "", fmt.Errorf("failed to get view DDL: %w", err)
	}
	return fmt.Sprintf("CREATE OR REPLACE VIEW %s AS\n%s",
		fqTable, strings.TrimSpace(def)), nil
}

func (d *Dao) GetTableColumns(ctx context.Context, schema, table string) ([]database.ColumnInfo, error) {
	query := `
		SELECT
			c.column_name,
			CASE WHEN c.data_type = 'USER-DEFINED' THEN c.udt_name ELSE c.data_type END,
			c.is_nullable = 'YES',
			c.column_default,
			EXISTS (
				SELECT 1
				FROM information_schema.key_column_usage kcu
				JOIN information_schema.table_constraints tc
					ON kcu.constraint_name = tc.constraint_name
					AND kcu.table_schema = tc.table_schema
				WHERE tc.constraint_type = 'PRIMARY KEY'
					AND kcu.table_schema = c.table_schema
					AND kcu.table_name = c.table_name
					AND kcu.column_name = c.column_name
			),
			c.ordinal_position,
			COALESCE((
				SELECT pgd.description
				FROM pg_catalog.pg_class pc
				JOIN pg_catalog.pg_namespace pn ON pn.oid = pc.relnamespace
				JOIN pg_catalog.pg_description pgd ON pgd.objoid = pc.oid AND pgd.objsubid = c.ordinal_position
				WHERE pn.nspname = c.table_schema AND pc.relname = c.table_name
				LIMIT 1
			), '')
		FROM information_schema.columns c
		WHERE c.table_schema = $1 AND c.table_name = $2
		ORDER BY c.ordinal_position
	`

	rows, err := d.client.DB.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get table columns: %w", err)
	}
	defer rows.Close()

	var columns []database.ColumnInfo
	for rows.Next() {
		var col database.ColumnInfo
		if err := rows.Scan(
			&col.Name, &col.DataType, &col.IsNullable,
			&col.Default, &col.IsPK, &col.Ordinal, &col.Comment,
		); err != nil {
			return nil, fmt.Errorf("failed to scan column: %w", err)
		}
		if col.Default != nil {
			lower := strings.ToLower(*col.Default)
			col.IsAutoGenerated = strings.Contains(lower, "nextval(") || strings.Contains(lower, "generated always")
		}
		columns = append(columns, col)
	}

	return columns, rows.Err()
}

func (d *Dao) GetTableConstraints(ctx context.Context, schema, table string) ([]database.ConstraintInfo, error) {
	query := `
		SELECT
			tc.constraint_name,
			tc.constraint_type,
			COALESCE(string_agg(CASE WHEN kcu.column_name IS NOT NULL THEN kcu.column_name END, ',' ORDER BY kcu.ordinal_position), ''),
			COALESCE(cc.check_clause, '')
		FROM information_schema.table_constraints tc
		LEFT JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		LEFT JOIN information_schema.check_constraints cc
			ON tc.constraint_name = cc.constraint_name
			AND tc.constraint_schema = cc.constraint_schema
		WHERE tc.table_schema = $1 AND tc.table_name = $2
		GROUP BY tc.constraint_name, tc.constraint_type, cc.check_clause
		ORDER BY tc.constraint_type, tc.constraint_name
	`

	rows, err := d.client.DB.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get table constraints: %w", err)
	}
	defer rows.Close()

	var constraints []database.ConstraintInfo
	for rows.Next() {
		var c database.ConstraintInfo
		var colsRaw string
		if err := rows.Scan(&c.Name, &c.Type, &colsRaw, &c.Def); err != nil {
			return nil, fmt.Errorf("failed to scan constraint: %w", err)
		}
		if colsRaw != "" {
			c.Columns = strings.Split(colsRaw, ",")
		}
		constraints = append(constraints, c)
	}

	return constraints, rows.Err()
}

func (d *Dao) GetTableForeignKeys(ctx context.Context, schema, table string) ([]database.ForeignKeyInfo, error) {
	query := `
		SELECT
			c.conname,
			c.conkey,
			nf.nspname,
			tf.relname,
			c.confkey,
			CASE c.confupdtype
				WHEN 'a' THEN 'NO ACTION' WHEN 'r' THEN 'RESTRICT'
				WHEN 'c' THEN 'CASCADE'   WHEN 'n' THEN 'SET NULL'
				WHEN 'd' THEN 'SET DEFAULT'
			END,
			CASE c.confdeltype
				WHEN 'a' THEN 'NO ACTION' WHEN 'r' THEN 'RESTRICT'
				WHEN 'c' THEN 'CASCADE'   WHEN 'n' THEN 'SET NULL'
				WHEN 'd' THEN 'SET DEFAULT'
			END
		FROM pg_constraint c
		JOIN pg_namespace n ON n.oid = c.connamespace
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_class tf ON tf.oid = c.confrelid
		JOIN pg_namespace nf ON nf.oid = tf.relnamespace
		WHERE c.contype = 'f'
			AND n.nspname = $1 AND t.relname = $2
		ORDER BY c.conname
	`

	rows, err := d.client.DB.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get foreign keys: %w", err)
	}
	defer rows.Close()

	var fks []database.ForeignKeyInfo
	for rows.Next() {
		var fk database.ForeignKeyInfo
		var conkeyRaw, confkeyRaw string
		if err := rows.Scan(
			&fk.Name, &conkeyRaw, &fk.ReferencedSchema,
			&fk.ReferencedTable, &confkeyRaw,
			&fk.OnUpdate, &fk.OnDelete,
		); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key: %w", err)
		}
		fk.Columns = resolveAttNames(ctx, d.client.DB, schema, table, conkeyRaw)
		fk.ReferencedCols = resolveAttNames(ctx, d.client.DB, fk.ReferencedSchema, fk.ReferencedTable, confkeyRaw)
		fks = append(fks, fk)
	}

	return fks, rows.Err()
}

func (d *Dao) GetIncomingForeignKeys(ctx context.Context, schema, table string) ([]database.IncomingForeignKeyInfo, error) {
	query := `
		SELECT
			ns.nspname,
			t.relname,
			c.conkey,
			c.confkey
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace ns ON ns.oid = t.relnamespace
		JOIN pg_class tf ON tf.oid = c.confrelid
		JOIN pg_namespace nf ON nf.oid = tf.relnamespace
		WHERE c.contype = 'f'
			AND nf.nspname = $1 AND tf.relname = $2
		ORDER BY ns.nspname, t.relname
	`

	rows, err := d.client.DB.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get incoming foreign keys: %w", err)
	}
	defer rows.Close()

	var fks []database.IncomingForeignKeyInfo
	for rows.Next() {
		var fk database.IncomingForeignKeyInfo
		var conkeyRaw, confkeyRaw string
		if err := rows.Scan(&fk.Schema, &fk.Table, &conkeyRaw, &confkeyRaw); err != nil {
			return nil, fmt.Errorf("failed to scan incoming foreign key: %w", err)
		}
		fk.Columns = resolveAttNames(ctx, d.client.DB, fk.Schema, fk.Table, conkeyRaw)
		fk.ReferencedCols = resolveAttNames(ctx, d.client.DB, schema, table, confkeyRaw)
		fks = append(fks, fk)
	}

	return fks, rows.Err()
}

func resolveAttNames(ctx context.Context, db *sql.DB, schema, table, arrStr string) []string {
	arrStr = strings.Trim(arrStr, "{}")
	if arrStr == "" {
		return nil
	}
	parts := strings.Split(arrStr, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	query := `
		SELECT attname, attnum
		FROM pg_attribute
		WHERE attrelid = (SELECT oid FROM pg_class WHERE relname = $2
			AND relnamespace = (SELECT oid FROM pg_namespace WHERE nspname = $1))
			AND attnum > 0 AND NOT attisdropped
	`
	attMap := make(map[string]string)
	rows, err := db.QueryContext(ctx, query, schema, table)
	if err != nil {
		return parts
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var attnum int
		if err := rows.Scan(&name, &attnum); err != nil {
			return parts
		}
		attMap[fmt.Sprintf("%d", attnum)] = name
	}
	result := make([]string, len(parts))
	for i, p := range parts {
		if name, ok := attMap[p]; ok {
			result[i] = name
		} else {
			result[i] = p
		}
	}
	return result
}

func (d *Dao) GetEstimatedRowCount(ctx context.Context, schema, table string) (int64, bool, error) {
	var count int64
	err := d.client.DB.QueryRowContext(ctx,
		`SELECT reltuples::bigint
		   FROM pg_class c
		   JOIN pg_namespace n ON n.oid = c.relnamespace
		  WHERE n.nspname = $1 AND c.relname = $2`,
		schema, table,
	).Scan(&count)
	if err != nil {
		return 0, false, err
	}
	if count < 0 {
		return 0, false, nil
	}
	return count, true, nil
}

func (d *Dao) FetchTableRows(ctx context.Context, state *database.TableState, where, orderBy string) (string, []database.Row, error) {
	fqTable := util.BacktickQuoter.Ident(state.Schema) + "." + util.BacktickQuoter.Ident(state.Table)
	query := fmt.Sprintf("SELECT * FROM %s", fqTable)

	if where != "" {
		if err := database.SanitizeWhereClause(where); err != nil {
			return "", nil, err
		}
		query += " WHERE " + where
	}
	if orderBy != "" {
		query += " ORDER BY " + orderBy
	}
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", state.BatchSize, int64(state.RowCount()))

	rows, err := d.client.DB.QueryContext(ctx, query)
	if err != nil {
		return "", nil, fmt.Errorf("failed to list rows: %w", err)
	}
	defer rows.Close()

	result, err := scanRows(rows)
	if err != nil {
		return "", nil, err
	}

	return query, result, nil
}

func (d *Dao) InsertRow(ctx context.Context, schema, table string, row database.Row) (database.PrimaryKey, error) {
	log.Info().Str("schema", schema).Str("table", table).Msg("Inserting row")
	fqTable := util.BacktickQuoter.Ident(schema) + "." + util.BacktickQuoter.Ident(table)

	cols := make([]string, 0, len(row))
	placeholders := make([]string, 0, len(row))
	args := make([]any, 0, len(row))
	i := 1
	for col, val := range row {
		cols = append(cols, util.BacktickQuoter.Ident(col))
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		args = append(args, val)
		i++
	}

	pkCols, err := d.getPrimaryKeyColumns(ctx, schema, table)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get PK columns for RETURNING clause")
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		fqTable, strings.Join(cols, ", "), strings.Join(placeholders, ", "))

	if len(pkCols) > 0 {
		quotedPK := make([]string, len(pkCols))
		for j, c := range pkCols {
			quotedPK[j] = util.BacktickQuoter.Ident(c)
		}
		query += " RETURNING " + strings.Join(quotedPK, ", ")

		var returnArgs []any
		returnArgs = append(returnArgs, args...)
		var returnVals []any
		for range pkCols {
			returnVals = append(returnVals, new(any))
		}

		row := d.client.DB.QueryRowContext(ctx, query, args...)
		if err := row.Scan(returnVals...); err != nil {
			return database.PrimaryKey{}, fmt.Errorf("failed to insert row: %w", err)
		}

		pk := database.PrimaryKey{Columns: make(map[string]any)}
		for j, col := range pkCols {
			pk.Columns[col] = reflect.ValueOf(returnVals[j]).Elem().Interface()
		}
		return pk, nil
	}

	_, err = d.client.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return database.PrimaryKey{}, fmt.Errorf("failed to insert row: %w", err)
	}
	return database.PrimaryKey{}, nil
}

func (d *Dao) UpdateRow(ctx context.Context, schema, table string, pk database.PrimaryKey, original, updated database.Row) error {
	log.Info().Str("schema", schema).Str("table", table).Interface("pk", pk.Columns).Msg("Updating row")
	setClauses := []string{}
	args := []any{}
	argIdx := 1

	pkSet := make(map[string]bool, len(pk.Columns))
	for col := range pk.Columns {
		pkSet[col] = true
	}

	for col, newVal := range updated {
		if col == "_pk" || pkSet[col] {
			continue
		}
		oldVal, exists := original[col]
		if !exists || !reflect.DeepEqual(oldVal, newVal) {
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", util.BacktickQuoter.Ident(col), argIdx))
			args = append(args, newVal)
			argIdx++
		}
	}

	if len(setClauses) == 0 {
		return nil
	}

	fqTable := util.BacktickQuoter.Ident(schema) + "." + util.BacktickQuoter.Ident(table)
	whereParts := []string{}
	for col, val := range pk.Columns {
		whereParts = append(whereParts, fmt.Sprintf("%s = $%d", util.BacktickQuoter.Ident(col), argIdx))
		args = append(args, val)
		argIdx++
	}

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		fqTable, strings.Join(setClauses, ", "), strings.Join(whereParts, " AND "))

	result, err := d.client.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update row: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no row found to update")
	}

	return nil
}

func (d *Dao) DeleteRows(ctx context.Context, schema, table string, pks []database.PrimaryKey) error {
	log.Info().Str("schema", schema).Str("table", table).Int("count", len(pks)).Msg("Deleting rows")
	fqTable := util.BacktickQuoter.Ident(schema) + "." + util.BacktickQuoter.Ident(table)

	for _, pk := range pks {
		parts, args := util.BacktickQuoter.WhereEqIndexed(pk.Columns)
		query := fmt.Sprintf("DELETE FROM %s WHERE %s", fqTable, strings.Join(parts, " AND "))

		result, err := d.client.DB.ExecContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("failed to delete row: %w", err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to get rows affected: %w", err)
		}
		if rowsAffected == 0 {
			log.Warn().Interface("pk", pk.Columns).Msg("No row found to delete")
		}
	}

	return nil
}

func (d *Dao) CommonDataTypes() []string {
	return []string{
		"INTEGER",
		"BIGINT",
		"SMALLINT",
		"SERIAL",
		"BIGSERIAL",
		"BOOLEAN",
		"TEXT",
		"VARCHAR(255)",
		"CHAR(1)",
		"UUID",
		"DATE",
		"TIME",
		"TIMESTAMP",
		"TIMESTAMPTZ",
		"NUMERIC",
		"DECIMAL",
		"REAL",
		"DOUBLE PRECISION",
		"JSON",
		"JSONB",
		"BYTEA",
	}
}

func (d *Dao) DefaultPKType() string { return "SERIAL" }

func (d *Dao) DefaultCreateTableDDL(schema, tableName string) string {
	return fmt.Sprintf("CREATE TABLE %s (id serial PRIMARY KEY)",
		util.BacktickQuoter.Ident(schema)+"."+util.BacktickQuoter.Ident(tableName))
}

func (d *Dao) GetTableDDL(ctx context.Context, schema, table string) (string, error) {
	columns, err := d.GetTableColumns(ctx, schema, table)
	if err != nil {
		return "", err
	}
	constraints, err := d.GetTableConstraints(ctx, schema, table)
	if err != nil {
		return "", err
	}
	fks, err := d.GetTableForeignKeys(ctx, schema, table)
	if err != nil {
		return "", err
	}
	return buildGaussDBDDL(schema, table, columns, constraints, fks), nil
}

func buildGaussDBDDL(schema, table string, cols []database.ColumnInfo, constraints []database.ConstraintInfo, fks []database.ForeignKeyInfo) string {
	var lines []string
	for _, col := range cols {
		line := fmt.Sprintf("  %s %s", util.BacktickQuoter.Ident(col.Name), col.DataType)
		if col.Default != nil && *col.Default != "" {
			line += " DEFAULT " + *col.Default
		}
		if !col.IsNullable {
			line += " NOT NULL"
		}
		lines = append(lines, line)
	}
	for _, c := range constraints {
		quoted := make([]string, len(c.Columns))
		for i, col := range c.Columns {
			quoted[i] = util.BacktickQuoter.Ident(col)
		}
		switch c.Type {
		case "PRIMARY KEY":
			lines = append(lines, fmt.Sprintf("  CONSTRAINT %s PRIMARY KEY (%s)",
				util.BacktickQuoter.Ident(c.Name), strings.Join(quoted, ", ")))
		case "UNIQUE":
			lines = append(lines, fmt.Sprintf("  CONSTRAINT %s UNIQUE (%s)",
				util.BacktickQuoter.Ident(c.Name), strings.Join(quoted, ", ")))
		case "CHECK":
			lines = append(lines, fmt.Sprintf("  CONSTRAINT %s CHECK (%s)",
				util.BacktickQuoter.Ident(c.Name), c.Def))
		}
	}
	for _, fk := range fks {
		fkCols := make([]string, len(fk.Columns))
		for i, col := range fk.Columns {
			fkCols[i] = util.BacktickQuoter.Ident(col)
		}
		refCols := make([]string, len(fk.ReferencedCols))
		for i, col := range fk.ReferencedCols {
			refCols[i] = util.BacktickQuoter.Ident(col)
		}
		refTable := util.BacktickQuoter.Ident(fk.ReferencedSchema) + "." + util.BacktickQuoter.Ident(fk.ReferencedTable)
		line := fmt.Sprintf("  CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)",
			util.BacktickQuoter.Ident(fk.Name), strings.Join(fkCols, ", "), refTable, strings.Join(refCols, ", "))
		if fk.OnUpdate != "" && fk.OnUpdate != "NO ACTION" {
			line += " ON UPDATE " + fk.OnUpdate
		}
		if fk.OnDelete != "" && fk.OnDelete != "NO ACTION" {
			line += " ON DELETE " + fk.OnDelete
		}
		lines = append(lines, line)
	}
	fqTable := util.BacktickQuoter.Ident(schema) + "." + util.BacktickQuoter.Ident(table)
	return fmt.Sprintf("CREATE TABLE %s (\n%s\n)", fqTable, strings.Join(lines, ",\n"))
}

func (d *Dao) CreateTable(ctx context.Context, schema, ddl string) error {
	_, err := d.client.DB.ExecContext(ctx, ddl)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}
	log.Info().Str("schema", schema).Str("ddl", ddl).Msg("Table created")
	return nil
}

func (d *Dao) DropTable(ctx context.Context, schema, table string) error {
	fqTable := util.BacktickQuoter.Ident(schema) + "." + util.BacktickQuoter.Ident(table)
	_, err := d.client.DB.ExecContext(ctx, fmt.Sprintf("DROP TABLE %s", fqTable))
	if err != nil {
		return fmt.Errorf("failed to drop table: %w", err)
	}
	log.Info().Str("schema", schema).Str("table", table).Msg("Table dropped")
	return nil
}

func (d *Dao) RenameTable(ctx context.Context, schema, old, newName string) error {
	fqTable := util.BacktickQuoter.Ident(schema) + "." + util.BacktickQuoter.Ident(old)
	_, err := d.client.DB.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s RENAME TO %s", fqTable, util.BacktickQuoter.Ident(newName)))
	if err != nil {
		return fmt.Errorf("failed to rename table: %w", err)
	}
	log.Info().Str("schema", schema).Str("old", old).Str("new", newName).Msg("Table renamed")
	return nil
}

func (d *Dao) RenameColumn(ctx context.Context, schema, table, old, newName string) error {
	fqTable := util.BacktickQuoter.Ident(schema) + "." + util.BacktickQuoter.Ident(table)
	_, err := d.client.DB.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s",
			fqTable, util.BacktickQuoter.Ident(old), util.BacktickQuoter.Ident(newName)))
	if err != nil {
		return fmt.Errorf("failed to rename column: %w", err)
	}
	log.Info().Str("schema", schema).Str("table", table).Str("old", old).Str("new", newName).Msg("Column renamed")
	return nil
}

func (d *Dao) TruncateTable(ctx context.Context, schema, table string) error {
	fqTable := util.BacktickQuoter.Ident(schema) + "." + util.BacktickQuoter.Ident(table)
	_, err := d.client.DB.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %s", fqTable))
	if err != nil {
		return fmt.Errorf("failed to truncate table: %w", err)
	}
	log.Info().Str("schema", schema).Str("table", table).Msg("Table truncated")
	return nil
}

func (d *Dao) GetIndexes(ctx context.Context, schema, table string) ([]database.IndexInfo, error) {
	query := `
		SELECT
			i.relname AS index_name,
			ix.indisunique,
			ix.indisprimary,
			am.amname AS index_type,
			COALESCE(pg_relation_size(i.oid), 0) AS index_size,
			pg_get_indexdef(ix.indexrelid) AS definition
		FROM pg_index ix
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_class t ON t.oid = ix.indrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		JOIN pg_am am ON am.oid = i.relam
		WHERE n.nspname = $1 AND t.relname = $2
		ORDER BY i.relname
	`

	rows, err := d.client.DB.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get indexes: %w", err)
	}
	defer rows.Close()

	var indexes []database.IndexInfo
	for rows.Next() {
		var idx database.IndexInfo
		if err := rows.Scan(
			&idx.Name, &idx.IsUnique,
			&idx.IsPrimary, &idx.Type, &idx.Size, &idx.Definition,
		); err != nil {
			return nil, fmt.Errorf("failed to scan index: %w", err)
		}
		idx.Columns = parseIndexColumns(idx.Definition)
		indexes = append(indexes, idx)
	}

	return indexes, rows.Err()
}

func parseIndexColumns(definition string) []string {
	start := strings.Index(definition, "(")
	end := strings.LastIndex(definition, ")")
	if start < 0 || end < 0 || end <= start {
		return nil
	}
	colPart := definition[start+1 : end]
	parts := strings.Split(colPart, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		col := strings.TrimSpace(p)
		if col != "" {
			result = append(result, col)
		}
	}
	return result
}

func (d *Dao) CreateIndex(ctx context.Context, schema, table string, def database.IndexDefinition) error {
	uniqueStr := ""
	if def.IsUnique {
		uniqueStr = "UNIQUE "
	}

	quotedCols := make([]string, len(def.Columns))
	for i, c := range def.Columns {
		parts := strings.Fields(c)
		quotedCols[i] = util.BacktickQuoter.Ident(parts[0])
		if len(parts) > 1 {
			quotedCols[i] += " " + parts[1]
		}
	}

	fqTable := util.BacktickQuoter.Ident(schema) + "." + util.BacktickQuoter.Ident(table)
	indexName := util.BacktickQuoter.Ident(def.Name)

	usingClause := ""
	if def.Type != "" && def.Type != "btree" {
		usingClause = fmt.Sprintf(" USING %s", def.Type)
	}

	query := fmt.Sprintf("CREATE %sINDEX %s ON %s%s (%s)",
		uniqueStr, indexName, fqTable, usingClause, strings.Join(quotedCols, ", "))

	_, err := d.client.DB.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}
	log.Info().Str("schema", schema).Str("table", table).Str("index", def.Name).Msg("Index created")
	return nil
}

func (d *Dao) DropIndex(ctx context.Context, schema, indexName string) error {
	fqIndex := util.BacktickQuoter.Ident(schema) + "." + util.BacktickQuoter.Ident(indexName)
	_, err := d.client.DB.ExecContext(ctx, fmt.Sprintf("DROP INDEX %s", fqIndex))
	if err != nil {
		return fmt.Errorf("failed to drop index: %w", err)
	}
	log.Info().Str("schema", schema).Str("index", indexName).Msg("Index dropped")
	return nil
}

func (d *Dao) FetchQueryRows(ctx context.Context, rawSQL string, limit, offset int64) (string, []database.Row, []database.ColumnInfo, error) {
	bypassSubquery := sqlpkg.IsExplainQuery(rawSQL) || sqlpkg.IsReturningDML(rawSQL)

	var paged string
	if bypassSubquery {
		paged = rawSQL
	} else {
		paged = fmt.Sprintf("SELECT * FROM (%s) AS _q LIMIT %d OFFSET %d", rawSQL, limit, offset)
	}

	rows, err := d.client.DB.QueryContext(ctx, paged)
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	cols, err := rows.ColumnTypes()
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to get column types: %w", err)
	}

	colInfos := make([]database.ColumnInfo, len(cols))
	for i, ct := range cols {
		colInfos[i] = database.ColumnInfo{Name: ct.Name(), DataType: ct.DatabaseTypeName(), Ordinal: i + 1}
	}

	result, err := scanRows(rows)
	if err != nil {
		return "", nil, nil, err
	}

	return paged, result, colInfos, nil
}

func (d *Dao) ExecuteQuery(ctx context.Context, query string) ([]database.Row, []database.ColumnInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	cols, err := rows.ColumnTypes()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get column types: %w", err)
	}

	colInfos := make([]database.ColumnInfo, len(cols))
	for i, ct := range cols {
		colInfos[i] = database.ColumnInfo{Name: ct.Name(), Ordinal: i + 1}
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

func (d *Dao) ExplainPlan(ctx context.Context, sql string) (string, error) {
	return d.explainRaw(ctx, "EXPLAIN (FORMAT JSON) "+sql)
}

func (d *Dao) ExplainAnalyze(ctx context.Context, sql string) (string, error) {
	return d.explainRaw(ctx, "EXPLAIN (ANALYZE, FORMAT JSON) "+sql)
}

func (d *Dao) explainRaw(ctx context.Context, query string) (string, error) {
	rows, _, err := d.ExecuteQuery(ctx, query)
	if err != nil {
		return "", err
	}
	var parts []string
	for _, row := range rows {
		for _, v := range row {
			if v != nil {
				parts = append(parts, v.(string))
			}
		}
	}
	return strings.Join(parts, ""), nil
}

func (d *Dao) GetTableColumnNames(ctx context.Context, schema, table string) ([]string, error) {
	query := `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position
	`

	rows, err := d.client.DB.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get column names: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan column name: %w", err)
		}
		names = append(names, name)
	}

	return names, rows.Err()
}

func scanRows(rows *sql.Rows) ([]database.Row, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	log.Debug().Strs("columns", cols).Int("count", len(cols)).Msg("scanRows: scanning columns")

	var result []database.Row
	for rows.Next() {
		values := make([]any, len(cols))
		valuePtrs := make([]any, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(database.Row, len(cols))
		for i, col := range cols {
			if values[i] == nil {
				row[col] = nil
			} else {
				switch v := values[i].(type) {
				case []byte:
					row[col] = string(v)
				default:
					row[col] = fmt.Sprintf("%v", v)
				}
			}
		}
		result = append(result, row)
	}

	log.Debug().Int("rows_scanned", len(result)).Msg("scanRows: done")
	return result, rows.Err()
}

func (d *Dao) getPrimaryKeyColumns(ctx context.Context, schema, table string) ([]string, error) {
	query := `
		SELECT kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY'
			AND tc.table_schema = $1
			AND tc.table_name = $2
		ORDER BY kcu.ordinal_position
	`

	rows, err := d.client.DB.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, err
		}
		cols = append(cols, col)
	}

	return cols, rows.Err()
}
