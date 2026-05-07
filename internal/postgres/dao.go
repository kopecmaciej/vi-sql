package postgres

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/rs/zerolog/log"
)

// Dao implements database.Driver for PostgreSQL.
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
	connCfg := d.client.Pool.Config().ConnConfig
	host := d.client.Config.Host
	port := d.client.Config.Port
	if host == "" {
		host = connCfg.Host
	}
	if port == 0 {
		port = int(connCfg.Port)
	}
	info := &database.ServerInfo{
		Host:  host,
		Port:  port,
		Extra: make(map[string]string),
	}

	var version string
	err := d.client.Pool.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return nil, fmt.Errorf("failed to get server version: %w", err)
	}
	info.Version = version

	var uptime string
	err = d.client.Pool.QueryRow(ctx,
		"SELECT (now() - pg_postmaster_start_time())::text").Scan(&uptime)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get server uptime")
	} else {
		info.Uptime = uptime
	}

	var sessions int64
	err = d.client.Pool.QueryRow(ctx,
		"SELECT count(*) FROM pg_stat_activity WHERE state IS NOT NULL").Scan(&sessions)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get active sessions")
	} else {
		info.ActiveSessions = sessions
	}

	info.CurrentDB = d.client.Config.Database

	var maxConns int64
	err = d.client.Pool.QueryRow(ctx,
		"SELECT setting::int FROM pg_settings WHERE name = 'max_connections'").Scan(&maxConns)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get max connections")
	} else {
		info.MaxConnections = maxConns
	}

	var dbSize string
	err = d.client.Pool.QueryRow(ctx,
		"SELECT pg_size_pretty(pg_database_size(current_database()))").Scan(&dbSize)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get database size")
	} else {
		info.DatabaseSize = dbSize
	}

	var cacheHit string
	err = d.client.Pool.QueryRow(ctx, `
		SELECT to_char(round(blks_hit::numeric / nullif(blks_hit + blks_read, 0) * 100, 1), 'FM990.0') || '%'
		FROM pg_stat_database WHERE datname = current_database()`).Scan(&cacheHit)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get cache hit ratio")
	} else {
		info.CacheHitRatio = cacheHit
	}

	return info, nil
}

func (d *Dao) GetActiveSessions(ctx context.Context) (int64, error) {
	var count int64
	err := d.client.Pool.QueryRow(ctx,
		"SELECT count(*) FROM pg_stat_activity WHERE state IS NOT NULL").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get active sessions: %w", err)
	}
	return count, nil
}

func (d *Dao) ListSchemas(ctx context.Context, nameFilter string) ([]database.Schema, error) {
	query := `
		SELECT s.schema_name, COALESCE(array_agg(t.table_name ORDER BY t.table_name) FILTER (WHERE t.table_name IS NOT NULL), '{}')
		FROM information_schema.schemata s
		LEFT JOIN information_schema.tables t ON s.schema_name = t.table_schema AND t.table_type = 'BASE TABLE'
		WHERE s.schema_name NOT IN ('information_schema', 'pg_catalog', 'pg_toast')
	`
	args := []any{}
	argIdx := 1

	if nameFilter != "" {
		query += fmt.Sprintf(` AND (s.schema_name ILIKE $%d OR t.table_name ILIKE $%d)`, argIdx, argIdx)
		args = append(args, "%"+nameFilter+"%")
	}

	query += ` GROUP BY s.schema_name ORDER BY s.schema_name`

	rows, err := d.client.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}
	defer rows.Close()

	var result []database.Schema
	for rows.Next() {
		var schema string
		var tables []string
		if err := rows.Scan(&schema, &tables); err != nil {
			return nil, fmt.Errorf("failed to scan schema row: %w", err)
		}
		result = append(result, database.Schema{
			Schema: schema,
			Tables: tables,
		})
	}

	return result, rows.Err()
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
			COALESCE(pgd.description, '')
		FROM information_schema.columns c
		LEFT JOIN pg_catalog.pg_statio_all_tables st
			ON st.schemaname = c.table_schema AND st.relname = c.table_name
		LEFT JOIN pg_catalog.pg_description pgd
			ON pgd.objoid = st.relid AND pgd.objsubid = c.ordinal_position
		WHERE c.table_schema = $1 AND c.table_name = $2
		ORDER BY c.ordinal_position
	`

	rows, err := d.client.Pool.Query(ctx, query, schema, table)
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
		columns = append(columns, col)
	}

	return columns, rows.Err()
}

func (d *Dao) GetTableConstraints(ctx context.Context, schema, table string) ([]database.ConstraintInfo, error) {
	query := `
		SELECT
			tc.constraint_name,
			tc.constraint_type,
			COALESCE(array_agg(kcu.column_name ORDER BY kcu.ordinal_position) FILTER (WHERE kcu.column_name IS NOT NULL), '{}'),
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

	rows, err := d.client.Pool.Query(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get table constraints: %w", err)
	}
	defer rows.Close()

	var constraints []database.ConstraintInfo
	for rows.Next() {
		var c database.ConstraintInfo
		if err := rows.Scan(&c.Name, &c.Type, &c.Columns, &c.Def); err != nil {
			return nil, fmt.Errorf("failed to scan constraint: %w", err)
		}
		constraints = append(constraints, c)
	}

	return constraints, rows.Err()
}

func (d *Dao) GetTableForeignKeys(ctx context.Context, schema, table string) ([]database.ForeignKeyInfo, error) {
	query := `
		SELECT
			c.conname,
			array_agg(a.attname ORDER BY u.i),
			nf.nspname,
			tf.relname,
			array_agg(af.attname ORDER BY u.i),
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
		CROSS JOIN LATERAL unnest(c.conkey) WITH ORDINALITY u(attnum, i)
		JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = u.attnum
		CROSS JOIN LATERAL unnest(c.confkey) WITH ORDINALITY uf(attnum, i)
		JOIN pg_attribute af ON af.attrelid = c.confrelid AND af.attnum = uf.attnum AND uf.i = u.i
		WHERE c.contype = 'f'
			AND n.nspname = $1 AND t.relname = $2
		GROUP BY c.conname, nf.nspname, tf.relname, c.confupdtype, c.confdeltype
		ORDER BY c.conname
	`

	rows, err := d.client.Pool.Query(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get foreign keys: %w", err)
	}
	defer rows.Close()

	var fks []database.ForeignKeyInfo
	for rows.Next() {
		var fk database.ForeignKeyInfo
		if err := rows.Scan(
			&fk.Name, &fk.Columns, &fk.ReferencedSchema,
			&fk.ReferencedTable, &fk.ReferencedCols,
			&fk.OnUpdate, &fk.OnDelete,
		); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key: %w", err)
		}
		fks = append(fks, fk)
	}

	return fks, rows.Err()
}

func (d *Dao) GetIncomingForeignKeys(ctx context.Context, schema, table string) ([]database.IncomingForeignKeyInfo, error) {
	query := `
		SELECT
			ns.nspname,
			t.relname,
			array_agg(a.attname ORDER BY u.i),
			array_agg(af.attname ORDER BY u.i)
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_namespace ns ON ns.oid = t.relnamespace
		JOIN pg_class tf ON tf.oid = c.confrelid
		JOIN pg_namespace nf ON nf.oid = tf.relnamespace
		CROSS JOIN LATERAL unnest(c.conkey) WITH ORDINALITY u(attnum, i)
		JOIN pg_attribute a ON a.attrelid = c.conrelid AND a.attnum = u.attnum
		CROSS JOIN LATERAL unnest(c.confkey) WITH ORDINALITY uf(attnum, i)
		JOIN pg_attribute af ON af.attrelid = c.confrelid AND af.attnum = uf.attnum AND uf.i = u.i
		WHERE c.contype = 'f'
			AND nf.nspname = $1 AND tf.relname = $2
		GROUP BY ns.nspname, t.relname, c.conname
		ORDER BY ns.nspname, t.relname
	`

	rows, err := d.client.Pool.Query(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get incoming foreign keys: %w", err)
	}
	defer rows.Close()

	var fks []database.IncomingForeignKeyInfo
	for rows.Next() {
		var fk database.IncomingForeignKeyInfo
		if err := rows.Scan(&fk.Schema, &fk.Table, &fk.Columns, &fk.ReferencedCols); err != nil {
			return nil, fmt.Errorf("failed to scan incoming foreign key: %w", err)
		}
		fks = append(fks, fk)
	}

	return fks, rows.Err()
}

func (d *Dao) GetEstimatedRowCount(ctx context.Context, schema, table string) (int64, bool, error) {
	var count int64
	err := d.client.Pool.QueryRow(ctx,
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
	fqTable := pgx.Identifier{state.Schema, state.Table}.Sanitize()
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

	rows, err := d.client.Pool.Query(ctx, query, pgx.QueryResultFormats{pgx.TextFormatCode})
	if err != nil {
		return "", nil, fmt.Errorf("failed to list rows: %w", err)
	}
	defer rows.Close()

	result, err := scanTextRows(rows)
	if err != nil {
		return "", nil, err
	}

	return query, result, nil
}

func (d *Dao) GetRow(ctx context.Context, schema, table string, pk database.PrimaryKey) (database.Row, error) {
	fqTable := pgx.Identifier{schema, table}.Sanitize()
	whereParts, args := buildPKWhere(pk)
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s", fqTable, strings.Join(whereParts, " AND "))

	queryArgs := append([]any{pgx.QueryResultFormats{pgx.TextFormatCode}}, args...)
	rows, err := d.client.Pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to get row: %w", err)
	}
	defer rows.Close()

	result, err := scanTextRows(rows)
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("row not found")
	}

	return result[0], nil
}

func (d *Dao) InsertRow(ctx context.Context, schema, table string, row database.Row) (database.PrimaryKey, error) {
	log.Info().Str("schema", schema).Str("table", table).Msg("Inserting row")
	fqTable := pgx.Identifier{schema, table}.Sanitize()

	cols := make([]string, 0, len(row))
	placeholders := make([]string, 0, len(row))
	args := make([]any, 0, len(row))
	i := 1
	for col, val := range row {
		cols = append(cols, pgx.Identifier{col}.Sanitize())
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		args = append(args, val)
		i++
	}

	// Try to get PK columns to use RETURNING
	pkCols, err := d.getPrimaryKeyColumns(ctx, schema, table)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get PK columns for RETURNING clause")
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		fqTable, strings.Join(cols, ", "), strings.Join(placeholders, ", "))

	if len(pkCols) > 0 {
		quotedPK := make([]string, len(pkCols))
		for j, c := range pkCols {
			quotedPK[j] = pgx.Identifier{c}.Sanitize()
		}
		query += " RETURNING " + strings.Join(quotedPK, ", ")

		insertArgs := append([]any{pgx.QueryResultFormats{pgx.TextFormatCode}}, args...)
		rows, err := d.client.Pool.Query(ctx, query, insertArgs...)
		if err != nil {
			return database.PrimaryKey{}, fmt.Errorf("failed to insert row: %w", err)
		}
		defer rows.Close()

		if rows.Next() {
			raw := rows.RawValues()
			pk := database.PrimaryKey{Columns: make(map[string]any)}
			for j, col := range pkCols {
				if raw[j] == nil {
					pk.Columns[col] = nil
				} else {
					pk.Columns[col] = string(raw[j])
				}
			}
			return pk, nil
		}
		return database.PrimaryKey{}, fmt.Errorf("insert returned no rows")
	}

	_, err = d.client.Pool.Exec(ctx, query, args...)
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
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", pgx.Identifier{col}.Sanitize(), argIdx))
			args = append(args, newVal)
			argIdx++
		}
	}

	if len(setClauses) == 0 {
		return nil
	}

	fqTable := pgx.Identifier{schema, table}.Sanitize()
	whereParts := []string{}
	for col, val := range pk.Columns {
		whereParts = append(whereParts, fmt.Sprintf("%s = $%d", pgx.Identifier{col}.Sanitize(), argIdx))
		args = append(args, val)
		argIdx++
	}

	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s",
		fqTable, strings.Join(setClauses, ", "), strings.Join(whereParts, " AND "))

	result, err := d.client.Pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update row: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("no row found to update")
	}

	return nil
}

func (d *Dao) DeleteRows(ctx context.Context, schema, table string, pks []database.PrimaryKey) error {
	log.Info().Str("schema", schema).Str("table", table).Int("count", len(pks)).Msg("Deleting rows")
	fqTable := pgx.Identifier{schema, table}.Sanitize()

	for _, pk := range pks {
		whereParts, args := buildPKWhere(pk)
		query := fmt.Sprintf("DELETE FROM %s WHERE %s", fqTable, strings.Join(whereParts, " AND "))

		result, err := d.client.Pool.Exec(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("failed to delete row: %w", err)
		}
		if result.RowsAffected() == 0 {
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

func (d *Dao) DefaultCreateTableDDL(schema, tableName string) string {
	return fmt.Sprintf("CREATE TABLE %s (id serial PRIMARY KEY)",
		pgx.Identifier{schema, tableName}.Sanitize())
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
	return buildPostgresDDL(schema, table, columns, constraints, fks), nil
}

func buildPostgresDDL(schema, table string, cols []database.ColumnInfo, constraints []database.ConstraintInfo, fks []database.ForeignKeyInfo) string {
	var lines []string
	for _, col := range cols {
		line := fmt.Sprintf("  %s %s", pgx.Identifier{col.Name}.Sanitize(), col.DataType)
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
			quoted[i] = pgx.Identifier{col}.Sanitize()
		}
		switch c.Type {
		case "PRIMARY KEY":
			lines = append(lines, fmt.Sprintf("  CONSTRAINT %s PRIMARY KEY (%s)",
				pgx.Identifier{c.Name}.Sanitize(), strings.Join(quoted, ", ")))
		case "UNIQUE":
			lines = append(lines, fmt.Sprintf("  CONSTRAINT %s UNIQUE (%s)",
				pgx.Identifier{c.Name}.Sanitize(), strings.Join(quoted, ", ")))
		case "CHECK":
			lines = append(lines, fmt.Sprintf("  CONSTRAINT %s CHECK (%s)",
				pgx.Identifier{c.Name}.Sanitize(), c.Def))
		}
	}
	for _, fk := range fks {
		cols := make([]string, len(fk.Columns))
		for i, col := range fk.Columns {
			cols[i] = pgx.Identifier{col}.Sanitize()
		}
		refCols := make([]string, len(fk.ReferencedCols))
		for i, col := range fk.ReferencedCols {
			refCols[i] = pgx.Identifier{col}.Sanitize()
		}
		refTable := pgx.Identifier{fk.ReferencedSchema, fk.ReferencedTable}.Sanitize()
		line := fmt.Sprintf("  CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s (%s)",
			pgx.Identifier{fk.Name}.Sanitize(), strings.Join(cols, ", "), refTable, strings.Join(refCols, ", "))
		if fk.OnUpdate != "" && fk.OnUpdate != "NO ACTION" {
			line += " ON UPDATE " + fk.OnUpdate
		}
		if fk.OnDelete != "" && fk.OnDelete != "NO ACTION" {
			line += " ON DELETE " + fk.OnDelete
		}
		lines = append(lines, line)
	}
	fqTable := pgx.Identifier{schema, table}.Sanitize()
	return fmt.Sprintf("CREATE TABLE %s (\n%s\n)", fqTable, strings.Join(lines, ",\n"))
}

func (d *Dao) CreateTable(ctx context.Context, schema, ddl string) error {
	_, err := d.client.Pool.Exec(ctx, ddl)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}
	log.Info().Str("schema", schema).Str("ddl", ddl).Msg("Table created")
	return nil
}

func (d *Dao) DropTable(ctx context.Context, schema, table string) error {
	fqTable := pgx.Identifier{schema, table}.Sanitize()
	_, err := d.client.Pool.Exec(ctx, fmt.Sprintf("DROP TABLE %s", fqTable))
	if err != nil {
		return fmt.Errorf("failed to drop table: %w", err)
	}
	log.Info().Str("schema", schema).Str("table", table).Msg("Table dropped")
	return nil
}

func (d *Dao) RenameTable(ctx context.Context, schema, old, newName string) error {
	fqTable := pgx.Identifier{schema, old}.Sanitize()
	_, err := d.client.Pool.Exec(ctx,
		fmt.Sprintf("ALTER TABLE %s RENAME TO %s", fqTable, pgx.Identifier{newName}.Sanitize()))
	if err != nil {
		return fmt.Errorf("failed to rename table: %w", err)
	}
	log.Info().Str("schema", schema).Str("old", old).Str("new", newName).Msg("Table renamed")
	return nil
}

func (d *Dao) RenameColumn(ctx context.Context, schema, table, old, newName string) error {
	fqTable := pgx.Identifier{schema, table}.Sanitize()
	_, err := d.client.Pool.Exec(ctx,
		fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s",
			fqTable, pgx.Identifier{old}.Sanitize(), pgx.Identifier{newName}.Sanitize()))
	if err != nil {
		return fmt.Errorf("failed to rename column: %w", err)
	}
	log.Info().Str("schema", schema).Str("table", table).Str("old", old).Str("new", newName).Msg("Column renamed")
	return nil
}

func (d *Dao) TruncateTable(ctx context.Context, schema, table string) error {
	fqTable := pgx.Identifier{schema, table}.Sanitize()
	_, err := d.client.Pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s", fqTable))
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
			array_agg(a.attname ORDER BY x.ordinality) AS columns,
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
		CROSS JOIN LATERAL unnest(ix.indkey) WITH ORDINALITY AS x(attnum, ordinality)
		JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = x.attnum
		WHERE n.nspname = $1 AND t.relname = $2
		GROUP BY i.relname, ix.indisunique, ix.indisprimary, am.amname, i.oid, ix.indexrelid
		ORDER BY i.relname
	`

	rows, err := d.client.Pool.Query(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get indexes: %w", err)
	}
	defer rows.Close()

	var indexes []database.IndexInfo
	for rows.Next() {
		var idx database.IndexInfo
		if err := rows.Scan(
			&idx.Name, &idx.Columns, &idx.IsUnique,
			&idx.IsPrimary, &idx.Type, &idx.Size, &idx.Definition,
		); err != nil {
			return nil, fmt.Errorf("failed to scan index: %w", err)
		}
		indexes = append(indexes, idx)
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
		quotedCols[i] = pgx.Identifier{parts[0]}.Sanitize()
		if len(parts) > 1 {
			quotedCols[i] += " " + parts[1]
		}
	}

	fqTable := pgx.Identifier{schema, table}.Sanitize()
	indexName := pgx.Identifier{def.Name}.Sanitize()

	usingClause := ""
	if def.Type != "" && def.Type != "btree" {
		usingClause = fmt.Sprintf(" USING %s", def.Type)
	}

	query := fmt.Sprintf("CREATE %sINDEX %s ON %s%s (%s)",
		uniqueStr, indexName, fqTable, usingClause, strings.Join(quotedCols, ", "))

	_, err := d.client.Pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}
	log.Info().Str("schema", schema).Str("table", table).Str("index", def.Name).Msg("Index created")
	return nil
}

func (d *Dao) DropIndex(ctx context.Context, schema, indexName string) error {
	fqIndex := pgx.Identifier{schema, indexName}.Sanitize()
	_, err := d.client.Pool.Exec(ctx, fmt.Sprintf("DROP INDEX %s", fqIndex))
	if err != nil {
		return fmt.Errorf("failed to drop index: %w", err)
	}
	log.Info().Str("schema", schema).Str("index", indexName).Msg("Index dropped")
	return nil
}

func (d *Dao) FetchQueryRows(ctx context.Context, rawSQL string, limit, offset int64) (string, []database.Row, []database.ColumnInfo, error) {
	bypassSubquery := database.IsExplainQuery(rawSQL) || database.IsReturningDML(rawSQL)

	var paged string
	if bypassSubquery {
		paged = rawSQL
	} else {
		paged = fmt.Sprintf("SELECT * FROM (%s) AS _q LIMIT %d OFFSET %d", rawSQL, limit, offset)
	}

	rows, err := d.client.Pool.Query(ctx, paged, pgx.QueryResultFormats{pgx.TextFormatCode})
	if err != nil {
		return "", nil, nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	fieldDescs := rows.FieldDescriptions()
	typeMap := pgtype.NewMap()
	cols := make([]database.ColumnInfo, len(fieldDescs))
	for i, fd := range fieldDescs {
		dataType := ""
		if t, ok := typeMap.TypeForOID(fd.DataTypeOID); ok {
			dataType = t.Name
		}
		cols[i] = database.ColumnInfo{Name: fd.Name, DataType: dataType, Ordinal: i + 1}
	}

	result, err := scanTextRows(rows)
	if err != nil {
		return "", nil, nil, err
	}

	return paged, result, cols, nil
}

func (d *Dao) ExecuteQuery(ctx context.Context, query string) ([]database.Row, []database.ColumnInfo, error) {
	rows, err := d.client.Pool.Query(ctx, query, pgx.QueryResultFormats{pgx.TextFormatCode})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	fieldDescs := rows.FieldDescriptions()
	colInfos := make([]database.ColumnInfo, len(fieldDescs))
	for i, fd := range fieldDescs {
		colInfos[i] = database.ColumnInfo{Name: fd.Name, Ordinal: i + 1}
	}

	result, err := scanTextRows(rows)
	if err != nil {
		return nil, nil, err
	}

	return result, colInfos, nil
}

func (d *Dao) ExecuteStatement(ctx context.Context, stmt string) (int64, error) {
	result, err := d.client.Pool.Exec(ctx, stmt)
	if err != nil {
		return 0, fmt.Errorf("failed to execute statement: %w", err)
	}
	affected := result.RowsAffected()
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

	rows, err := d.client.Pool.Query(ctx, query, schema, table)
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

// scanTextRows reads all rows from a text-format pgx result set and returns
// them as database.Row maps. Values are plain strings; NULL columns are nil.
func scanTextRows(rows pgx.Rows) ([]database.Row, error) {
	fieldDescs := rows.FieldDescriptions()
	var result []database.Row

	for rows.Next() {
		raw := rows.RawValues()
		row := make(database.Row, len(fieldDescs))
		for i, fd := range fieldDescs {
			if raw[i] == nil {
				row[fd.Name] = nil
			} else {
				row[fd.Name] = string(raw[i])
			}
		}
		result = append(result, row)
	}

	return result, rows.Err()
}

// getPrimaryKeyColumns returns the primary key column names for a table.
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

	rows, err := d.client.Pool.Query(ctx, query, schema, table)
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
