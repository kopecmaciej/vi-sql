package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
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
	info := &database.ServerInfo{
		Host:  d.client.Config.Host,
		Port:  d.client.Config.Port,
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
		"SELECT now() - pg_postmaster_start_time()").Scan(&uptime)
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

func (d *Dao) ListSchemasWithTables(ctx context.Context, nameFilter string) ([]database.SchemaWithTables, error) {
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
		argIdx++
	}

	query += ` GROUP BY s.schema_name ORDER BY s.schema_name`

	rows, err := d.client.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}
	defer rows.Close()

	var result []database.SchemaWithTables
	for rows.Next() {
		var schema string
		var tables []string
		if err := rows.Scan(&schema, &tables); err != nil {
			return nil, fmt.Errorf("failed to scan schema row: %w", err)
		}
		result = append(result, database.SchemaWithTables{
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
			c.data_type,
			c.is_nullable = 'YES',
			c.column_default,
			COALESCE(tc.constraint_type = 'PRIMARY KEY', false),
			c.ordinal_position,
			COALESCE(pgd.description, '')
		FROM information_schema.columns c
		LEFT JOIN information_schema.key_column_usage kcu
			ON c.table_schema = kcu.table_schema
			AND c.table_name = kcu.table_name
			AND c.column_name = kcu.column_name
		LEFT JOIN information_schema.table_constraints tc
			ON kcu.constraint_name = tc.constraint_name
			AND kcu.table_schema = tc.table_schema
			AND tc.constraint_type = 'PRIMARY KEY'
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
			array_agg(kcu.column_name ORDER BY kcu.ordinal_position),
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
			tc.constraint_name,
			array_agg(kcu.column_name ORDER BY kcu.ordinal_position),
			ccu.table_schema,
			ccu.table_name,
			array_agg(ccu.column_name ORDER BY kcu.ordinal_position),
			rc.update_rule,
			rc.delete_rule
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON ccu.constraint_name = tc.constraint_name
			AND ccu.table_schema = tc.constraint_schema
		JOIN information_schema.referential_constraints rc
			ON tc.constraint_name = rc.constraint_name
			AND tc.constraint_schema = rc.constraint_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_schema = $1 AND tc.table_name = $2
		GROUP BY tc.constraint_name, ccu.table_schema, ccu.table_name, rc.update_rule, rc.delete_rule
		ORDER BY tc.constraint_name
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

func (d *Dao) ListRows(ctx context.Context, state *database.TableState, where, orderBy string,
	columns []string, countCallback func(int64)) ([]database.Row, error) {

	colExpr := "*"
	if len(columns) > 0 {
		quoted := make([]string, len(columns))
		for i, c := range columns {
			quoted[i] = pgx.Identifier{c}.Sanitize()
		}
		colExpr = strings.Join(quoted, ", ")
	}

	fqTable := pgx.Identifier{state.Schema, state.Table}.Sanitize()
	query := fmt.Sprintf("SELECT %s FROM %s", colExpr, fqTable)

	if where != "" {
		if err := database.SanitizeWhereClause(where); err != nil {
			return nil, err
		}
		query += " WHERE " + where
	}
	if orderBy != "" {
		query += " ORDER BY " + orderBy
	}
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", state.Limit, state.Offset)

	rows, err := d.client.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list rows: %w", err)
	}
	defer rows.Close()

	fieldDescs := rows.FieldDescriptions()
	var result []database.Row
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("failed to scan row values: %w", err)
		}
		row := make(database.Row, len(fieldDescs))
		for i, fd := range fieldDescs {
			row[fd.Name] = values[i]
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Count rows asynchronously
	if countCallback != nil {
		go func() {
			countQuery := fmt.Sprintf("SELECT count(*) FROM %s", fqTable)
			if where != "" {
				countQuery += " WHERE " + where
			}
			var count int64
			err := d.client.Pool.QueryRow(ctx, countQuery).Scan(&count)
			if err != nil {
				log.Error().Err(err).Msg("Failed to count rows")
				return
			}
			countCallback(count)
		}()
	}

	return result, nil
}

func (d *Dao) GetRow(ctx context.Context, schema, table string, pk database.PrimaryKey) (database.Row, error) {
	fqTable := pgx.Identifier{schema, table}.Sanitize()
	whereParts, args := buildPKWhere(pk)
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s", fqTable, strings.Join(whereParts, " AND "))

	rows, err := d.client.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get row: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, fmt.Errorf("row not found")
	}

	fieldDescs := rows.FieldDescriptions()
	values, err := rows.Values()
	if err != nil {
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}

	row := make(database.Row, len(fieldDescs))
	for i, fd := range fieldDescs {
		row[fd.Name] = values[i]
	}

	return row, nil
}

func (d *Dao) InsertRow(ctx context.Context, schema, table string, row database.Row) (database.PrimaryKey, error) {
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

		rows, err := d.client.Pool.Query(ctx, query, args...)
		if err != nil {
			return database.PrimaryKey{}, fmt.Errorf("failed to insert row: %w", err)
		}
		defer rows.Close()

		if rows.Next() {
			values, err := rows.Values()
			if err != nil {
				return database.PrimaryKey{}, fmt.Errorf("failed to scan returned PK: %w", err)
			}
			pk := database.PrimaryKey{Columns: make(map[string]any)}
			for j, col := range pkCols {
				pk.Columns[col] = values[j]
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
	setClauses := []string{}
	args := []any{}
	argIdx := 1

	for col, newVal := range updated {
		if col == "_pk" {
			continue
		}
		oldVal, exists := original[col]
		if !exists || oldVal != newVal {
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

func (d *Dao) CreateTable(ctx context.Context, schema, ddl string) error {
	_, err := d.client.Pool.Exec(ctx, ddl)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}
	return nil
}

func (d *Dao) DropTable(ctx context.Context, schema, table string) error {
	fqTable := pgx.Identifier{schema, table}.Sanitize()
	_, err := d.client.Pool.Exec(ctx, fmt.Sprintf("DROP TABLE %s", fqTable))
	if err != nil {
		return fmt.Errorf("failed to drop table: %w", err)
	}
	return nil
}

func (d *Dao) RenameTable(ctx context.Context, schema, old, newName string) error {
	fqTable := pgx.Identifier{schema, old}.Sanitize()
	_, err := d.client.Pool.Exec(ctx,
		fmt.Sprintf("ALTER TABLE %s RENAME TO %s", fqTable, pgx.Identifier{newName}.Sanitize()))
	if err != nil {
		return fmt.Errorf("failed to rename table: %w", err)
	}
	return nil
}

func (d *Dao) TruncateTable(ctx context.Context, schema, table string) error {
	fqTable := pgx.Identifier{schema, table}.Sanitize()
	_, err := d.client.Pool.Exec(ctx, fmt.Sprintf("TRUNCATE TABLE %s", fqTable))
	if err != nil {
		return fmt.Errorf("failed to truncate table: %w", err)
	}
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
		quotedCols[i] = pgx.Identifier{c}.Sanitize()
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
	return nil
}

func (d *Dao) DropIndex(ctx context.Context, schema, indexName string) error {
	fqIndex := pgx.Identifier{schema, indexName}.Sanitize()
	_, err := d.client.Pool.Exec(ctx, fmt.Sprintf("DROP INDEX %s", fqIndex))
	if err != nil {
		return fmt.Errorf("failed to drop index: %w", err)
	}
	return nil
}

func (d *Dao) ExecuteQuery(ctx context.Context, query string) ([]database.Row, []database.ColumnInfo, error) {
	rows, err := d.client.Pool.Query(ctx, query)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	fieldDescs := rows.FieldDescriptions()
	var colInfos []database.ColumnInfo
	for i, fd := range fieldDescs {
		colInfos = append(colInfos, database.ColumnInfo{
			Name:    fd.Name,
			Ordinal: i + 1,
		})
	}

	var result []database.Row
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to scan row: %w", err)
		}
		row := make(database.Row, len(fieldDescs))
		for i, fd := range fieldDescs {
			row[fd.Name] = values[i]
		}
		result = append(result, row)
	}

	return result, colInfos, rows.Err()
}

func (d *Dao) ExecuteStatement(ctx context.Context, stmt string) (int64, error) {
	result, err := d.client.Pool.Exec(ctx, stmt)
	if err != nil {
		return 0, fmt.Errorf("failed to execute statement: %w", err)
	}
	return result.RowsAffected(), nil
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

// buildPKWhere creates WHERE clause parts and args from a PrimaryKey.
func buildPKWhere(pk database.PrimaryKey) ([]string, []any) {
	parts := make([]string, 0, len(pk.Columns))
	args := make([]any, 0, len(pk.Columns))
	i := 1
	for col, val := range pk.Columns {
		parts = append(parts, fmt.Sprintf("%s = $%d", pgx.Identifier{col}.Sanitize(), i))
		args = append(args, val)
		i++
	}
	return parts, args
}
