package cockroachdb

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/kopecmaciej/vi-sql/internal/database"
	pgdriver "github.com/kopecmaciej/vi-sql/internal/driver/postgres"
	"github.com/rs/zerolog/log"
)

var _ database.Driver = (*Dao)(nil)

// Dao implements database.Driver for CockroachDB by embedding the PostgreSQL Dao
// and overriding methods where CockroachDB's SQL dialect differs.
type Dao struct {
	*pgdriver.Dao
	client *pgdriver.Client
}

func NewDao(client *pgdriver.Client) *Dao {
	return &Dao{Dao: pgdriver.NewDao(client), client: client}
}

// ListSchemas excludes CockroachDB-specific internal schemas in addition to the
// standard PostgreSQL ones.
func (d *Dao) ListSchemas(ctx context.Context, nameFilter string) ([]database.Schema, error) {
	query := `
		SELECT s.schema_name, COALESCE(array_agg(t.table_name ORDER BY t.table_name) FILTER (WHERE t.table_name IS NOT NULL), '{}')
		FROM information_schema.schemata s
		LEFT JOIN information_schema.tables t ON s.schema_name = t.table_schema AND t.table_type = 'BASE TABLE'
		WHERE s.schema_name NOT IN ('information_schema', 'pg_catalog', 'pg_toast', 'crdb_internal', 'pg_extension')
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
		result = append(result, database.Schema{Schema: schema, Tables: tables})
	}

	return result, rows.Err()
}

// GetServerInfo uses CockroachDB-specific views where PostgreSQL system catalogs
// are unavailable (pg_stat_activity, pg_postmaster_start_time, pg_stat_ssl, etc.).
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
		Host:      host,
		Port:      port,
		CurrentDB: d.client.Config.Database,
		Extra:     make(map[string]string),
	}

	var version string
	if err := d.client.Pool.QueryRow(ctx, "SELECT version()").Scan(&version); err != nil {
		return nil, fmt.Errorf("failed to get server version: %w", err)
	}
	info.Version = version

	var sessions int64
	if err := d.client.Pool.QueryRow(ctx,
		"SELECT count(*) FROM crdb_internal.node_sessions").Scan(&sessions); err != nil {
		log.Warn().Err(err).Msg("Failed to get active sessions")
	} else {
		info.ActiveSessions = sessions
	}

	var dbSize string
	if err := d.client.Pool.QueryRow(ctx,
		"SELECT pg_size_pretty(pg_database_size(current_database()))").Scan(&dbSize); err != nil {
		log.Warn().Err(err).Msg("Failed to get database size")
	} else {
		info.DatabaseSize = dbSize
	}

	if sslMode := d.client.Config.SSLMode; sslMode != "" && sslMode != "disable" {
		info.TLS = sslMode
	}

	return info, nil
}

// GetActiveSessions uses crdb_internal.node_sessions instead of pg_stat_activity.
func (d *Dao) GetActiveSessions(ctx context.Context) (int64, error) {
	var count int64
	err := d.client.Pool.QueryRow(ctx,
		"SELECT count(*) FROM crdb_internal.node_sessions").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get active sessions: %w", err)
	}
	return count, nil
}

// GetEstimatedRowCount performs an exact COUNT(*) because CockroachDB does not
// maintain pg_class.reltuples statistics reliably.
func (d *Dao) GetEstimatedRowCount(ctx context.Context, schema, table string) (int64, bool, error) {
	fqTable := pgx.Identifier{schema, table}.Sanitize()
	var count int64
	err := d.client.Pool.QueryRow(ctx, fmt.Sprintf("SELECT count(*) FROM %s", fqTable)).Scan(&count)
	return count, false, err
}

// GetIndexes uses SHOW INDEXES which is the reliable CockroachDB-native way to
// inspect indexes. pg_index / pg_am / pg_relation_size / pg_get_indexdef are
// either absent or inaccurate in CockroachDB.
func (d *Dao) GetIndexes(ctx context.Context, schema, table string) ([]database.IndexInfo, error) {
	fqTable := pgx.Identifier{schema, table}.Sanitize()
	query := fmt.Sprintf(`
		SELECT index_name, non_unique, seq_in_index, column_name, storing, implicit
		FROM [SHOW INDEXES FROM %s]
		ORDER BY index_name, seq_in_index
	`, fqTable)

	rows, err := d.client.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get indexes: %w", err)
	}
	defer rows.Close()

	type rawRow struct {
		indexName  string
		nonUnique  bool
		seqInIndex int
		columnName string
		storing    bool
		implicit   bool
	}

	var raws []rawRow
	for rows.Next() {
		var r rawRow
		if err := rows.Scan(&r.indexName, &r.nonUnique, &r.seqInIndex, &r.columnName, &r.storing, &r.implicit); err != nil {
			return nil, fmt.Errorf("failed to scan index row: %w", err)
		}
		raws = append(raws, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Group by index name; skip storing and implicit (hidden) columns.
	seen := map[string]int{}
	var indexes []database.IndexInfo
	for _, r := range raws {
		if r.storing || r.implicit {
			continue
		}
		if i, ok := seen[r.indexName]; ok {
			indexes[i].Columns = append(indexes[i].Columns, r.columnName)
		} else {
			seen[r.indexName] = len(indexes)
			indexes = append(indexes, database.IndexInfo{
				Name:      r.indexName,
				Columns:   []string{r.columnName},
				IsUnique:  !r.nonUnique,
				IsPrimary: strings.EqualFold(r.indexName, "primary"),
				Type:      "btree",
			})
		}
	}

	return indexes, nil
}

// ExplainAnalyze uses CockroachDB's EXPLAIN ANALYZE syntax rather than the
// PostgreSQL EXPLAIN (ANALYZE, ...) form.
func (d *Dao) ExplainAnalyze(ctx context.Context, sql string) (string, error) {
	resultRows, _, err := d.ExecuteQuery(ctx, "EXPLAIN ANALYZE (FORMAT JSON) "+sql)
	if err != nil {
		return "", err
	}
	var parts []string
	for _, row := range resultRows {
		for _, v := range row {
			if v != nil {
				parts = append(parts, v.(string))
			}
		}
	}
	return strings.Join(parts, ""), nil
}
