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

// Quoter implements database.QuoterProvider: identifiers are backtick-quoted
// in MySQL-compatible mode and double-quoted (ANSI) otherwise.
func (d *Dao) Quoter() util.Quoter {
	if d.client.Capabilities.CompatMode == CompatMySQL {
		return util.BacktickQuoter
	}
	return util.ANSIQuoter
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
	maxConnsQuery := "SELECT setting::int FROM pg_settings WHERE name = 'max_connections'"
	if d.client.Capabilities.CompatMode == CompatMySQL {
		maxConnsQuery = "SELECT setting::int FROM gs_settings WHERE name = 'max_connections'"
	}
	err = d.client.DB.QueryRowContext(ctx, maxConnsQuery).Scan(&maxConns)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get max connections")
	} else {
		info.MaxConnections = maxConns
	}

	var dbSize string
	dbSizeQuery := "SELECT pg_size_pretty(pg_database_size(current_database()))"
	if d.client.Capabilities.CompatMode == CompatMySQL {
		dbSizeQuery = `SELECT pg_size_pretty(COALESCE(SUM(data_length + index_length), 0)) FROM information_schema.tables WHERE table_schema = current_database()`
	}
	err = d.client.DB.QueryRowContext(ctx, dbSizeQuery).Scan(&dbSize)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get database size")
	} else {
		info.DatabaseSize = dbSize
	}

	var cacheHit string
	cacheHitQuery := `
		SELECT to_char(round(blks_hit::numeric / nullif(blks_hit + blks_read, 0) * 100, 1), 'FM990.0') || '%'
		FROM pg_stat_database WHERE datname = current_database()`
	if d.client.Capabilities.CompatMode == CompatMySQL {
		cacheHitQuery = `
			SELECT to_char(round(blks_hit::numeric / nullif(blks_hit + blks_read, 0) * 100, 1), 'FM990.0') || '%'
			FROM gs_stat_database WHERE datname = current_database()`
	}
	err = d.client.DB.QueryRowContext(ctx, cacheHitQuery).Scan(&cacheHit)
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
	view := "pg_stat_activity"
	if d.client.Capabilities.CompatMode == CompatMySQL {
		view = "gs_stat_activity"
	}
	var count int64
	err := d.client.DB.QueryRowContext(ctx,
		"SELECT count(*) FROM "+view+" WHERE state IS NOT NULL").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get active sessions: %w", err)
	}
	return count, nil
}

func (d *Dao) ListSchemas(ctx context.Context, nameFilter string) ([]database.Schema, error) {
	// MySQL-compatible mode has no string_agg and exposes MySQL system schemas.
	// System views/tables are kept: information_schema.tables reports them with
	// table_type values like 'SYSTEM VIEW'/'SYSTEM TOAST TABLE' that a
	// 'BASE TABLE'/'VIEW' filter would drop, so classification matches any
	// *VIEW* type and everything else is a table.
	aggTables := "COALESCE(string_agg(CASE WHEN t.table_type NOT LIKE '%VIEW%' THEN t.table_name END, ',' ORDER BY t.table_name), '')"
	aggViews := "COALESCE(string_agg(CASE WHEN t.table_type LIKE '%VIEW%' THEN t.table_name END, ',' ORDER BY t.table_name), '')"
	systemSchemas := "'pg_toast'"
	switch d.client.Capabilities.CompatMode {
	case CompatMySQL:
		aggTables = "COALESCE(GROUP_CONCAT(CASE WHEN t.table_type NOT LIKE '%VIEW%' THEN t.table_name END ORDER BY t.table_name SEPARATOR ','), '')"
		aggViews = "COALESCE(GROUP_CONCAT(CASE WHEN t.table_type LIKE '%VIEW%' THEN t.table_name END ORDER BY t.table_name SEPARATOR ','), '')"
		systemSchemas = "'mysql', 'performance_schema', 'pg_toast'"
	}

	query := fmt.Sprintf(`
		SELECT s.schema_name,
		       %s,
		       %s
		FROM information_schema.schemata s
		LEFT JOIN information_schema.tables t
			ON s.schema_name = t.table_schema
		WHERE s.schema_name NOT IN (%s)
			AND s.schema_name NOT LIKE 'pg\_temp\_%%'
			AND s.schema_name NOT LIKE 'pg\_toast\_temp\_%%'
			AND s.schema_name NOT IN ('blockchain', 'db4ai', 'dbe_pldebugger', 'snapshot', 'sqladvisor', 'sys')
	`, aggTables, aggViews, systemSchemas)
	args := []any{}
	argIdx := 1

	if nameFilter != "" {
		query += fmt.Sprintf(` AND (LOWER(s.schema_name) LIKE LOWER($%d) OR LOWER(t.table_name) LIKE LOWER($%d))`, argIdx, argIdx)
		args = append(args, "%"+nameFilter+"%")
	}

	query += ` GROUP BY s.schema_name ORDER BY s.schema_name`

	// In MySQL-compatible mode GROUP_CONCAT truncates at 1024 bytes by default
	// (group_concat_max_len), silently dropping tables of large system schemas
	// like dbe_perf (~380 entries). Pin a single connection and raise the limit
	// before running the aggregation.
	var queryDB interface {
		QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	} = d.client.DB
	if d.client.Capabilities.CompatMode == CompatMySQL {
		pinned, err := d.client.DB.Conn(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to pin connection for schema listing: %w", err)
		}
		defer pinned.Close()
		if _, err := pinned.ExecContext(ctx, "SET SESSION group_concat_max_len = 1048576"); err != nil {
			return nil, fmt.Errorf("failed to raise group_concat_max_len: %w", err)
		}
		queryDB = pinned
	}

	rows, err := queryDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}
	defer rows.Close()

	var result []database.Schema
	for rows.Next() {
		var schema, tablesRaw, viewsRaw string
		if err := rows.Scan(&schema, &tablesRaw, &viewsRaw); err != nil {
			return nil, fmt.Errorf("failed to scan schema row: %w", err)
		}
		result = append(result, database.Schema{
			Schema: schema,
			Tables: splitNonEmpty(tablesRaw),
			Views:  splitNonEmpty(viewsRaw),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func splitNonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	tags := parts[:0]
	for _, p := range parts {
		if p != "" {
			tags = append(tags, p)
		}
	}
	return tags
}

func (d *Dao) GetViewDDL(ctx context.Context, schema, view string) (string, error) {
	viewName := schema + "." + view
	var def string
	err := d.client.DB.QueryRowContext(ctx,
		"SELECT pg_get_viewdef($1, true)", viewName).Scan(&def)
	if err != nil {
		return "", fmt.Errorf("failed to get view DDL: %w", err)
	}
	return fmt.Sprintf("CREATE VIEW %s.%s AS\n%s",
		d.Quoter().Ident(schema), d.Quoter().Ident(view),
		strings.TrimSpace(def)), nil
}

func (d *Dao) GetTableColumns(ctx context.Context, schema, table string) ([]database.ColumnInfo, error) {
	if d.client.Capabilities.CompatMode == CompatMySQL {
		return d.getTableColumnsM(ctx, schema, table)
	}
	return d.getTableColumnsA(ctx, schema, table)
}

func (d *Dao) getTableColumnsA(ctx context.Context, schema, table string) ([]database.ColumnInfo, error) {
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

// getTableColumnsM uses the MySQL-compatible information_schema, which
// reports the full type in column_type, key role in column_key and
// auto_increment in extra.
func (d *Dao) getTableColumnsM(ctx context.Context, schema, table string) ([]database.ColumnInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT column_name, column_type, COALESCE(is_nullable, 'NO') = 'YES',
		       column_default, COALESCE(column_key, '') = 'PRI', ordinal_position,
		       COALESCE(column_comment, ''), COALESCE(extra, '')
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get table columns: %w", err)
	}
	defer rows.Close()

	var columns []database.ColumnInfo
	for rows.Next() {
		var col database.ColumnInfo
		var isPK, isNullable bool
		var extra string
		if err := rows.Scan(&col.Name, &col.DataType, &isNullable,
			&col.Default, &isPK, &col.Ordinal, &col.Comment, &extra); err != nil {
			return nil, fmt.Errorf("failed to scan column: %w", err)
		}
		col.IsNullable = isNullable
		col.IsPK = isPK
		col.IsAutoGenerated = strings.Contains(extra, "auto_increment")
		columns = append(columns, col)
	}

	return columns, rows.Err()
}

func (d *Dao) GetTableConstraints(ctx context.Context, schema, table string) ([]database.ConstraintInfo, error) {
	if d.client.Capabilities.CompatMode == CompatMySQL {
		return d.getTableConstraintsM(ctx, schema, table)
	}
	return d.getTableConstraintsA(ctx, schema, table)
}

func (d *Dao) getTableConstraintsA(ctx context.Context, schema, table string) ([]database.ConstraintInfo, error) {
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

// getTableConstraintsM uses the MySQL-compatible information_schema.
// PRIMARY KEY column lists are taken from columns.column_key (the
// constraint_name on the PK is auto-generated and does not match
// key_column_usage); UNIQUE column lists come from key_column_usage.
func (d *Dao) getTableConstraintsM(ctx context.Context, schema, table string) ([]database.ConstraintInfo, error) {
	// 1) Constraint names and types.
	crows, err := d.client.DB.QueryContext(ctx, `
		SELECT constraint_name, constraint_type
		FROM information_schema.table_constraints
		WHERE table_schema = $1 AND table_name = $2
		  AND constraint_type IN ('PRIMARY KEY', 'UNIQUE')
		ORDER BY constraint_type, constraint_name`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get table constraints: %w", err)
	}
	defer crows.Close()

	var constraints []database.ConstraintInfo
	for crows.Next() {
		var c database.ConstraintInfo
		if err := crows.Scan(&c.Name, &c.Type); err != nil {
			return nil, fmt.Errorf("failed to scan constraint: %w", err)
		}
		constraints = append(constraints, c)
	}
	if err := crows.Err(); err != nil {
		return nil, err
	}

	// 2) PRIMARY KEY columns come from columns.column_key.
	var pkCols string
	err = d.client.DB.QueryRowContext(ctx, `
		SELECT GROUP_CONCAT(column_name ORDER BY ordinal_position SEPARATOR ',')
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2 AND column_key = 'PRI'`, schema, table).
		Scan(&pkCols)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get primary key columns: %w", err)
	}

	// 3) UNIQUE columns come from key_column_usage.
	urows, err := d.client.DB.QueryContext(ctx, `
		SELECT tc.constraint_name,
		       COALESCE(GROUP_CONCAT(kcu.column_name ORDER BY kcu.ordinal_position SEPARATOR ','), '')
		FROM information_schema.table_constraints tc
		LEFT JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
			AND tc.table_name = kcu.table_name
		WHERE tc.table_schema = $1 AND tc.table_name = $2
		  AND tc.constraint_type = 'UNIQUE'
		GROUP BY tc.constraint_name`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get unique constraint columns: %w", err)
	}
	defer urows.Close()

	uniqueCols := make(map[string][]string)
	for urows.Next() {
		var name, colsRaw string
		if err := urows.Scan(&name, &colsRaw); err != nil {
			return nil, fmt.Errorf("failed to scan unique constraint: %w", err)
		}
		if colsRaw != "" {
			uniqueCols[name] = strings.Split(colsRaw, ",")
		}
	}
	if err := urows.Err(); err != nil {
		return nil, err
	}

	for i := range constraints {
		switch constraints[i].Type {
		case "PRIMARY KEY":
			if pkCols != "" {
				constraints[i].Columns = strings.Split(pkCols, ",")
			}
		case "UNIQUE":
			if cols, ok := uniqueCols[constraints[i].Name]; ok {
				constraints[i].Columns = cols
			}
		}
	}
	return constraints, nil
}

func (d *Dao) GetTableForeignKeys(ctx context.Context, schema, table string) ([]database.ForeignKeyInfo, error) {
	if d.client.Capabilities.CompatMode == CompatMySQL {
		return d.getTableForeignKeysM(ctx, schema, table)
	}
	return d.getTableForeignKeysA(ctx, schema, table)
}

func (d *Dao) getTableForeignKeysA(ctx context.Context, schema, table string) ([]database.ForeignKeyInfo, error) {
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
		fk.Columns, err = resolveAttNames(ctx, d.client.DB, schema, table, conkeyRaw)
		if err != nil {
			return nil, err
		}
		fk.ReferencedCols, err = resolveAttNames(ctx, d.client.DB, fk.ReferencedSchema, fk.ReferencedTable, confkeyRaw)
		if err != nil {
			return nil, err
		}
		fks = append(fks, fk)
	}

	return fks, rows.Err()
}

// getTableForeignKeysM uses the MySQL-compatible information_schema which
// exposes foreign keys through key_column_usage and referential_constraints.
func (d *Dao) getTableForeignKeysM(ctx context.Context, schema, table string) ([]database.ForeignKeyInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT kcu.constraint_name,
		       COALESCE(GROUP_CONCAT(kcu.column_name ORDER BY kcu.ordinal_position SEPARATOR ','), ''),
		       kcu.referenced_table_schema,
		       kcu.referenced_table_name,
		       COALESCE(GROUP_CONCAT(kcu.referenced_column_name ORDER BY kcu.ordinal_position SEPARATOR ','), ''),
		       rc.update_rule, rc.delete_rule
		FROM information_schema.key_column_usage kcu
		JOIN information_schema.referential_constraints rc
			ON kcu.constraint_name = rc.constraint_name
			AND kcu.table_schema = rc.constraint_schema
			AND kcu.table_name = rc.table_name
		WHERE kcu.table_schema = $1 AND kcu.table_name = $2
		  AND kcu.referenced_table_name IS NOT NULL
		GROUP BY kcu.constraint_name, kcu.referenced_table_schema,
		         kcu.referenced_table_name, rc.update_rule, rc.delete_rule
		ORDER BY kcu.constraint_name`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get foreign keys: %w", err)
	}
	defer rows.Close()

	var fks []database.ForeignKeyInfo
	for rows.Next() {
		var fk database.ForeignKeyInfo
		var colStr, refColStr string
		if err := rows.Scan(&fk.Name, &colStr, &fk.ReferencedSchema,
			&fk.ReferencedTable, &refColStr, &fk.OnUpdate, &fk.OnDelete); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key: %w", err)
		}
		fk.Columns = strings.Split(colStr, ",")
		fk.ReferencedCols = strings.Split(refColStr, ",")
		fks = append(fks, fk)
	}
	return fks, rows.Err()
}

func (d *Dao) GetIncomingForeignKeys(ctx context.Context, schema, table string) ([]database.IncomingForeignKeyInfo, error) {
	if d.client.Capabilities.CompatMode == CompatMySQL {
		return d.getIncomingForeignKeysM(ctx, schema, table)
	}
	return d.getIncomingForeignKeysA(ctx, schema, table)
}

func (d *Dao) getIncomingForeignKeysA(ctx context.Context, schema, table string) ([]database.IncomingForeignKeyInfo, error) {
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
		fk.Columns, err = resolveAttNames(ctx, d.client.DB, fk.Schema, fk.Table, conkeyRaw)
		if err != nil {
			return nil, err
		}
		fk.ReferencedCols, err = resolveAttNames(ctx, d.client.DB, schema, table, confkeyRaw)
		if err != nil {
			return nil, err
		}
		fks = append(fks, fk)
	}

	return fks, rows.Err()
}

// getIncomingForeignKeysM finds tables that reference the given table via a
// foreign key, using MySQL-compatible information_schema.
func (d *Dao) getIncomingForeignKeysM(ctx context.Context, schema, table string) ([]database.IncomingForeignKeyInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT kcu.table_schema, kcu.table_name,
		       COALESCE(GROUP_CONCAT(kcu.column_name ORDER BY kcu.ordinal_position SEPARATOR ','), ''),
		       COALESCE(GROUP_CONCAT(kcu.referenced_column_name ORDER BY kcu.ordinal_position SEPARATOR ','), '')
		FROM information_schema.key_column_usage kcu
		WHERE kcu.referenced_table_schema = $1 AND kcu.referenced_table_name = $2
		  AND kcu.referenced_column_name IS NOT NULL
		GROUP BY kcu.table_schema, kcu.table_name, kcu.constraint_name
		ORDER BY kcu.table_schema, kcu.table_name`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get incoming foreign keys: %w", err)
	}
	defer rows.Close()

	var result []database.IncomingForeignKeyInfo
	for rows.Next() {
		var fk database.IncomingForeignKeyInfo
		var colStr, refColStr string
		if err := rows.Scan(&fk.Schema, &fk.Table, &colStr, &refColStr); err != nil {
			return nil, fmt.Errorf("failed to scan incoming foreign key: %w", err)
		}
		fk.Columns = strings.Split(colStr, ",")
		fk.ReferencedCols = strings.Split(refColStr, ",")
		result = append(result, fk)
	}
	return result, rows.Err()
}

func resolveAttNames(ctx context.Context, db *sql.DB, schema, table, arrStr string) ([]string, error) {
	arrStr = strings.Trim(arrStr, "{}")
	if arrStr == "" {
		return nil, nil
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
		return nil, fmt.Errorf("failed to resolve attribute names for %s.%s: %w", schema, table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var attnum int
		if err := rows.Scan(&name, &attnum); err != nil {
			return nil, fmt.Errorf("failed to scan attribute name: %w", err)
		}
		attMap[fmt.Sprintf("%d", attnum)] = name
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]string, len(parts))
	for i, p := range parts {
		if name, ok := attMap[p]; ok {
			result[i] = name
		} else {
			result[i] = p
		}
	}
	return result, nil
}

func (d *Dao) GetEstimatedRowCount(ctx context.Context, schema, table string) (int64, bool, error) {
	if d.client.Capabilities.CompatMode == CompatMySQL {
		var count *int64
		err := d.client.DB.QueryRowContext(ctx, `
			SELECT table_rows
			FROM information_schema.tables
			WHERE table_schema = $1 AND table_name = $2`, schema, table).Scan(&count)
		if err != nil {
			return 0, false, err
		}
		if count == nil {
			return 0, false, nil
		}
		return *count, true, nil
	}

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
	fqTable := d.Quoter().Ident(state.Schema) + "." + d.Quoter().Ident(state.Table)
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

	cols, err := rows.ColumnTypes()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get column types: %w", err)
	}

	colInfos := make([]database.ColumnInfo, len(cols))
	for i, ct := range cols {
		colInfos[i] = database.ColumnInfo{Name: ct.Name(), DataType: queryResultColumnType(ct, d.client.Capabilities.CompatMode), Ordinal: i + 1}
	}

	result, err := scanRows(rows)
	if err != nil {
		// Some OID/XID-family columns arrive with int8-sized payloads that
		// gaussdb-go's Uint32Codec cannot decode (e.g. gs_variable_info in
		// M-mode); retry with every column cast to ::text.
		if textResult, textErr := d.scanQueryRowsAsText(ctx, query, colInfos); textErr == nil {
			return query, textResult, nil
		}
		return "", nil, err
	}

	return query, result, nil
}

func (d *Dao) InsertRow(ctx context.Context, schema, table string, row database.Row) (database.PrimaryKey, error) {
	log.Info().Str("schema", schema).Str("table", table).Msg("Inserting row")
	fqTable := d.Quoter().Ident(schema) + "." + d.Quoter().Ident(table)

	cols := make([]string, 0, len(row))
	placeholders := make([]string, 0, len(row))
	args := make([]any, 0, len(row))
	i := 1
	for col, val := range row {
		cols = append(cols, d.Quoter().Ident(col))
		placeholders = append(placeholders, fmt.Sprintf("$%d", i))
		args = append(args, val)
		i++
	}

	pkCols, err := d.getPrimaryKeyColumns(ctx, schema, table)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get PK columns for generated-key lookup")
	}
	// PK values explicitly present in the inserted row identify the new row
	// even without server-side generated-key support (composite PKs and
	// non-auto single-column PKs in MySQL-compatible mode).
	providedPK := database.PrimaryKey{Columns: make(map[string]any)}
	for _, c := range pkCols {
		if v, ok := row[c]; ok && v != nil {
			providedPK.Columns[c] = v
		}
	}
	completeProvided := len(pkCols) > 0 && len(providedPK.Columns) == len(pkCols)

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		fqTable, strings.Join(cols, ", "), strings.Join(placeholders, ", "))

	if d.client.Capabilities.SupportsReturning && len(pkCols) > 0 {
		quotedPK := make([]string, len(pkCols))
		for j, c := range pkCols {
			quotedPK[j] = d.Quoter().Ident(c)
		}
		query += " RETURNING " + strings.Join(quotedPK, ", ")

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

	// LAST_INSERT_ID only helps when the single PK column is auto-generated
	// and not supplied by the caller; otherwise fall through and report the
	// provided values below.
	if d.client.Capabilities.SupportsLastInsertID && len(pkCols) == 1 && !completeProvided {
		// gaussdb-go reports no last insert id, so in M-mode the generated
		// id comes from LAST_INSERT_ID(). It is session-scoped, so Exec and
		// the lookup must run on the same pinned connection.
		conn, err := d.client.DB.Conn(ctx)
		if err != nil {
			return database.PrimaryKey{}, fmt.Errorf("failed to insert row: %w", err)
		}
		defer conn.Close()

		if _, err := conn.ExecContext(ctx, query, args...); err != nil {
			return database.PrimaryKey{}, fmt.Errorf("failed to insert row: %w", err)
		}

		// A non-AUTO_INCREMENT primary key has no meaningful LAST_INSERT_ID,
		// so only adopt the value when the pk column is auto-generated.
		var isAuto bool
		if err := conn.QueryRowContext(ctx, `
			SELECT (extra LIKE '%auto_increment%') FROM information_schema.columns
			WHERE table_schema = $1 AND table_name = $2 AND column_name = $3`,
			schema, table, pkCols[0]).Scan(&isAuto); err == nil && isAuto {
			var id int64
			if err := conn.QueryRowContext(ctx, "SELECT LAST_INSERT_ID()").Scan(&id); err == nil {
				return database.PrimaryKey{Columns: map[string]any{pkCols[0]: id}}, nil
			}
		}
		return database.PrimaryKey{}, nil
	}

	_, err = d.client.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return database.PrimaryKey{}, fmt.Errorf("failed to insert row: %w", err)
	}
	if completeProvided {
		// Composite PK or a caller-supplied single-column PK: the inserted
		// values themselves identify the new row.
		return providedPK, nil
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
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", d.Quoter().Ident(col), argIdx))
			args = append(args, newVal)
			argIdx++
		}
	}

	if len(setClauses) == 0 {
		return nil
	}

	fqTable := d.Quoter().Ident(schema) + "." + d.Quoter().Ident(table)
	whereParts := []string{}
	for col, val := range pk.Columns {
		if val == nil {
			// `= NULL` never matches; IS NULL is the only correct predicate.
			whereParts = append(whereParts, fmt.Sprintf("%s IS NULL", d.Quoter().Ident(col)))
			continue
		}
		whereParts = append(whereParts, fmt.Sprintf("%s = $%d", d.Quoter().Ident(col), argIdx))
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
	fqTable := d.Quoter().Ident(schema) + "." + d.Quoter().Ident(table)

	for _, pk := range pks {
		parts, args := d.Quoter().WhereEqIndexed(pk.Columns)
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
	if d.client.Capabilities.CompatMode == CompatMySQL {
		return []string{
			"TINYINT",
			"SMALLINT",
			"INT",
			"BIGINT",
			"DECIMAL(10,2)",
			"FLOAT",
			"DOUBLE",
			"VARCHAR(255)",
			"CHAR(36)",
			"TEXT",
			"DATE",
			"TIME",
			"DATETIME",
			"TIMESTAMP",
			"BLOB",
			"JSON",
		}
	}
	return commonDataTypesPG()
}

func commonDataTypesPG() []string {
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

func (d *Dao) DefaultPKType() string {
	if d.client.Capabilities.CompatMode == CompatMySQL {
		return "INT AUTO_INCREMENT"
	}
	return "SERIAL"
}

func (d *Dao) DefaultCreateTableDDL(schema, tableName string) string {
	return fmt.Sprintf("CREATE TABLE %s (id %s PRIMARY KEY)",
		d.Quoter().Ident(schema)+"."+d.Quoter().Ident(tableName), d.DefaultPKType())
}

func (d *Dao) GetTableDDL(ctx context.Context, schema, table string) (string, error) {
	tableName := schema + "." + table
	var ddl string
	err := d.client.DB.QueryRowContext(ctx,
		"SELECT pg_get_tabledef($1)", tableName).Scan(&ddl)
	if err != nil {
		return "", fmt.Errorf("failed to get table DDL: %w", err)
	}
	return ddl, nil
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
	fqTable := d.Quoter().Ident(schema) + "." + d.Quoter().Ident(table)
	_, err := d.client.DB.ExecContext(ctx, fmt.Sprintf("DROP TABLE %s", fqTable))
	if err != nil {
		return fmt.Errorf("failed to drop table: %w", err)
	}
	log.Info().Str("schema", schema).Str("table", table).Msg("Table dropped")
	return nil
}

func (d *Dao) RenameTable(ctx context.Context, schema, old, newName string) error {
	fqTable := d.Quoter().Ident(schema) + "." + d.Quoter().Ident(old)
	_, err := d.client.DB.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s RENAME TO %s", fqTable, d.Quoter().Ident(newName)))
	if err != nil {
		return fmt.Errorf("failed to rename table: %w", err)
	}
	log.Info().Str("schema", schema).Str("old", old).Str("new", newName).Msg("Table renamed")
	return nil
}

func (d *Dao) RenameColumn(ctx context.Context, schema, table, old, newName string) error {
	if d.client.Capabilities.SupportsChangeColumn {
		return d.renameColumnViaChange(ctx, schema, table, old, newName)
	}

	fqTable := d.Quoter().Ident(schema) + "." + d.Quoter().Ident(table)
	_, err := d.client.DB.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s",
			fqTable, d.Quoter().Ident(old), d.Quoter().Ident(newName)))
	if err != nil {
		return fmt.Errorf("failed to rename column: %w", err)
	}
	log.Info().Str("schema", schema).Str("table", table).Str("old", old).Str("new", newName).Msg("Column renamed")
	return nil
}

// renameColumnViaChange rewrites the column via CHANGE COLUMN, the only
// column-rename form the MySQL-compatible grammar accepts (both "RENAME
// COLUMN x TO y" and "RENAME x TO y" are rejected with SQLSTATE 42601).
// CHANGE requires the full replacement definition, so type, nullability,
// default, auto_increment, a non-default per-column collation and the comment
// are rebuilt from information_schema to keep the rename non-destructive.
func (d *Dao) renameColumnViaChange(ctx context.Context, schema, table, old, newName string) error {
	var colType, isNullable, colDefault, extra, comment, collation, tableCollation string
	err := d.client.DB.QueryRowContext(ctx, `
		SELECT c.column_type, c.is_nullable, COALESCE(c.column_default, ''),
		       COALESCE(c.extra, ''), COALESCE(c.column_comment, ''),
		       COALESCE(c.collation_name, ''),
		       COALESCE((SELECT t.table_collation FROM information_schema.tables t
		                 WHERE t.table_schema = c.table_schema AND t.table_name = c.table_name), '')
		FROM information_schema.columns c
		WHERE c.table_schema = $1 AND c.table_name = $2 AND c.column_name = $3`,
		schema, table, old).Scan(&colType, &isNullable, &colDefault, &extra, &comment, &collation, &tableCollation)
	if err != nil {
		return fmt.Errorf("failed to get column definition: %w", err)
	}

	def := colType
	if collation != "" && collation != tableCollation {
		def += " COLLATE " + collation
	}
	if isNullable == "NO" {
		def += " NOT NULL"
	}
	// In MySQL-compatible mode information_schema reports the literal string
	// "AUTO_INCREMENT" as the default of auto_increment columns; emitting it
	// would become a DEFAULT clause and fail, so it is skipped here.
	if strings.Contains(extra, "auto_increment") {
		def += " AUTO_INCREMENT"
	} else if colDefault != "" {
		def += " DEFAULT " + colDefault
	}
	// "ON UPDATE CURRENT_TIMESTAMP" also lives in extra; without carrying it
	// over the CHANGE COLUMN rewrite silently changes column behavior.
	if onUpdate := onUpdateClause(extra); onUpdate != "" {
		def += " " + onUpdate
	}
	if comment != "" {
		def += " COMMENT '" + strings.ReplaceAll(comment, "'", "''") + "'"
	}

	fqTable := d.Quoter().Ident(schema) + "." + d.Quoter().Ident(table)
	_, err = d.client.DB.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s CHANGE COLUMN %s %s %s",
		fqTable, d.Quoter().Ident(old), d.Quoter().Ident(newName), def))
	if err != nil {
		return fmt.Errorf("failed to rename column: %w", err)
	}
	log.Info().Str("schema", schema).Str("table", table).Str("old", old).Str("new", newName).Msg("Column renamed")
	return nil
}

func (d *Dao) TruncateTable(ctx context.Context, schema, table string) error {
	fqTable := d.Quoter().Ident(schema) + "." + d.Quoter().Ident(table)
	_, err := d.client.DB.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %s", fqTable))
	if err != nil {
		return fmt.Errorf("failed to truncate table: %w", err)
	}
	log.Info().Str("schema", schema).Str("table", table).Msg("Table truncated")
	return nil
}

func (d *Dao) GetIndexes(ctx context.Context, schema, table string) ([]database.IndexInfo, error) {
	if d.client.Capabilities.CompatMode == CompatMySQL {
		return d.getIndexesM(ctx, schema, table)
	}

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

// getIndexesM uses the MySQL-compatible information_schema.statistics view.
func (d *Dao) getIndexesM(ctx context.Context, schema, table string) ([]database.IndexInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT index_name, non_unique, index_type,
		       GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ',')
		FROM information_schema.statistics
		WHERE table_schema = $1 AND table_name = $2
		GROUP BY index_name, non_unique, index_type
		ORDER BY index_name`, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get indexes: %w", err)
	}
	defer rows.Close()

	var indexes []database.IndexInfo
	for rows.Next() {
		var idx database.IndexInfo
		var colsRaw string
		if err := rows.Scan(&idx.Name, &idx.IsUnique, &idx.Type, &colsRaw); err != nil {
			return nil, fmt.Errorf("failed to scan index: %w", err)
		}
		idx.IsUnique = !idx.IsUnique
		idx.IsPrimary = idx.Name == "PRIMARY"
		idx.Columns = strings.Split(colsRaw, ",")
		indexes = append(indexes, idx)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	d.enrichIndexSizeAndDefinition(ctx, schema, table, indexes)
	return indexes, nil
}

// enrichIndexSizeAndDefinition fills Size and Definition from the PG-flavored
// catalogs (pg_index/pg_class), which exist under both compatibility modes;
// information_schema.statistics carries neither. Best-effort: on failure the
// base columns from information_schema stay untouched.
func (d *Dao) enrichIndexSizeAndDefinition(ctx context.Context, schema, table string, indexes []database.IndexInfo) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT i.relname,
		       COALESCE(pg_relation_size(i.oid), 0),
		       pg_get_indexdef(ix.indexrelid)
		FROM pg_index ix
		JOIN pg_class i ON i.oid = ix.indexrelid
		JOIN pg_class t ON t.oid = ix.indrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = $1 AND t.relname = $2`, schema, table)
	if err != nil {
		log.Warn().Err(err).Str("schema", schema).Str("table", table).
			Msg("failed to read index size/definition")
		return
	}
	defer rows.Close()

	type indexMeta struct {
		size int64
		def  string
	}
	meta := make(map[string]indexMeta, len(indexes))
	for rows.Next() {
		var name, def string
		var size int64
		if err := rows.Scan(&name, &size, &def); err != nil {
			log.Warn().Err(err).Msg("failed to scan index size/definition")
			return
		}
		meta[name] = indexMeta{size: size, def: def}
	}
	if err := rows.Err(); err != nil {
		log.Warn().Err(err).Msg("failed to read index size/definition")
		return
	}
	for i := range indexes {
		if m, ok := meta[indexes[i].Name]; ok {
			indexes[i].Size = m.size
			indexes[i].Definition = m.def
		}
	}
}

func parseIndexColumns(definition string) []string {
	open := strings.IndexByte(definition, '(')
	if open < 0 {
		return nil
	}
	closeIdx := matchingParen(definition, open)
	if closeIdx < 0 {
		return nil
	}
	content := definition[open+1 : closeIdx]

	var result []string
	depth := 0
	start := 0
	inSingle, inDouble := false, false
	for i := 0; i < len(content); i++ {
		c := content[i]
		switch {
		case inSingle:
			if c == '\'' {
				inSingle = false
			}
		case inDouble:
			if c == '"' {
				inDouble = false
			}
		case c == '\'':
			inSingle = true
		case c == '"':
			inDouble = true
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == ',' && depth == 0:
			if col := strings.TrimSpace(content[start:i]); col != "" {
				result = append(result, col)
			}
			start = i + 1
		}
	}
	if col := strings.TrimSpace(content[start:]); col != "" {
		result = append(result, col)
	}
	return result
}

func matchingParen(s string, open int) int {
	depth := 0
	inSingle, inDouble := false, false
	for i := open; i < len(s); i++ {
		c := s[i]
		switch {
		case inSingle:
			if c == '\'' {
				inSingle = false
			}
		case inDouble:
			if c == '"' {
				inDouble = false
			}
		case c == '\'':
			inSingle = true
		case c == '"':
			inDouble = true
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func (d *Dao) CreateIndex(ctx context.Context, schema, table string, def database.IndexDefinition) error {
	uniqueStr := ""
	if def.IsUnique {
		uniqueStr = "UNIQUE "
	}

	quotedCols := make([]string, len(def.Columns))
	for i, c := range def.Columns {
		parts := strings.Fields(c)
		quotedCols[i] = d.Quoter().Ident(parts[0])
		if len(parts) > 1 {
			quotedCols[i] += " " + parts[1]
		}
	}

	fqTable := d.Quoter().Ident(schema) + "." + d.Quoter().Ident(table)
	indexName := d.Quoter().Ident(def.Name)

	// MySQL-compatible mode rejects USING entirely (verified: both USING btree
	// and USING hash fail with SQLSTATE 42601); GaussDB picks the access method
	// itself there, so only A-mode emits an explicit access method.
	usingClause := ""
	if d.client.Capabilities.CompatMode != CompatMySQL && def.Type != "" && def.Type != "btree" {
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
	fqIndex := d.Quoter().Ident(schema) + "." + d.Quoter().Ident(indexName)
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
		colInfos[i] = database.ColumnInfo{Name: ct.Name(), DataType: queryResultColumnType(ct, d.client.Capabilities.CompatMode), Ordinal: i + 1}
	}

	result, err := scanRows(rows)
	if err != nil {
		if !bypassSubquery {
			if textResult, textErr := d.scanQueryRowsAsText(ctx, paged, colInfos); textErr == nil {
				return paged, textResult, colInfos, nil
			}
		}
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
		colInfos[i] = database.ColumnInfo{Name: ct.Name(), DataType: queryResultColumnType(ct, d.client.Capabilities.CompatMode), Ordinal: i + 1}
	}

	result, err := scanRows(rows)
	if err != nil {
		if textResult, textErr := d.scanQueryRowsAsText(ctx, query, colInfos); textErr == nil {
			return textResult, colInfos, nil
		}
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
	if d.client.Capabilities.SupportsExplainJSON {
		return d.explainRaw(ctx, "EXPLAIN (FORMAT JSON) "+sql)
	}
	return d.explainRaw(ctx, "EXPLAIN "+sql)
}

func (d *Dao) ExplainAnalyze(ctx context.Context, sql string) (string, error) {
	if d.client.Capabilities.SupportsExplainPerf {
		return d.explainRaw(ctx, "EXPLAIN (ANALYZE, FORMAT JSON) "+sql)
	}
	return d.ExplainPlan(ctx, sql)
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

// scanQueryRowsAsText re-runs a query casting every column to ::text. GaussDB
// mislabels some OID/XID columns (e.g. pg_database.datfrozenxid64 in
// MySQL-compatible mode) as XID while sending int8-sized payloads that
// gaussdb-go's Uint32Codec cannot decode. Casting to text makes the server
// send text values, bypassing the broken codec path.
func (d *Dao) scanQueryRowsAsText(ctx context.Context, query string, colInfos []database.ColumnInfo) ([]database.Row, error) {
	names := make([]string, len(colInfos))
	for i, ci := range colInfos {
		names[i] = ci.Name
	}

	casts := make([]string, len(names))
	for i, n := range names {
		casts[i] = "t." + d.Quoter().Ident(n) + "::text"
	}

	textQuery := fmt.Sprintf("SELECT %s FROM (%s) AS t", strings.Join(casts, ", "), query)
	rows, err := d.client.DB.QueryContext(ctx, textQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRows(rows)
}

// queryResultColumnType returns the display type name for a query-result
// column. gaussdb-go's ColumnTypeDatabaseTypeName returns the PostgreSQL
// canonical name uppercased (e.g. "NUMERIC" for a decimal column), which is
// lowercased here to match the information_schema-derived table metadata. In
// MySQL-compatible mode the PostgreSQL names are additionally mapped to the
// MySQL dialect names so query results match what the MySQL-flavored
// information_schema reports for the same column (and what the MySQL driver
// shows).
func queryResultColumnType(ct *sql.ColumnType, compatMode CompatMode) string {
	return normalizeQueryResultTypeName(ct.DatabaseTypeName(), compatMode)
}

func normalizeQueryResultTypeName(name string, compatMode CompatMode) string {
	name = strings.ToLower(name)
	if compatMode == CompatMySQL {
		switch name {
		case "numeric":
			return "decimal"
		case "int2":
			return "smallint"
		case "int4":
			return "int"
		case "int8":
			return "bigint"
		case "bool":
			return "boolean"
		case "float4":
			return "float"
		case "float8":
			return "double"
		case "bpchar":
			return "char"
		}
	}
	return name
}

func scanRows(rows *sql.Rows) ([]database.Row, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

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

	return result, rows.Err()
}

func (d *Dao) getPrimaryKeyColumns(ctx context.Context, schema, table string) ([]string, error) {
	// M-mode exposes primary keys via information_schema.statistics; the
	// table_constraints/key_column_usage join returns nothing there.
	if d.client.Capabilities.CompatMode == CompatMySQL {
		return d.getPrimaryKeyColumnsM(ctx, schema, table)
	}

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

func (d *Dao) getPrimaryKeyColumnsM(ctx context.Context, schema, table string) ([]string, error) {
	query := `
		SELECT column_name
		FROM information_schema.statistics
		WHERE table_schema = $1 AND table_name = $2 AND index_name = 'PRIMARY'
		ORDER BY seq_in_index
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

// onUpdateClause extracts an "ON UPDATE <expr>" tail from a MySQL-style extra
// column attribute so CHANGE COLUMN keeps the behavior instead of silently
// dropping it.
func onUpdateClause(extra string) string {
	idx := strings.Index(strings.ToLower(extra), "on update")
	if idx < 0 {
		return ""
	}
	clause := strings.TrimSpace(extra[idx:])
	if strings.EqualFold(clause, "on update") {
		return ""
	}
	return clause
}
