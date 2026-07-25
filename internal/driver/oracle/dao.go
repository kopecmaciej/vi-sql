package oracle

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

const quote = util.ANSIQuoter

// Dao implements database.Driver for Oracle.
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
		"SELECT BANNER, ORA_DATABASE_NAME FROM V$VERSION WHERE ROWNUM = 1").
		Scan(&version, &dbName); err != nil {
		// V$VERSION may be restricted; try simpler queries
		_ = d.client.DB.QueryRowContext(ctx, "SELECT BANNER FROM V$VERSION WHERE ROWNUM = 1").Scan(&version)
		_ = d.client.DB.QueryRowContext(ctx, "SELECT ORA_DATABASE_NAME FROM DUAL").Scan(&dbName)
	}
	info.Version = version
	info.CurrentDB = dbName

	var host string
	if err := d.client.DB.QueryRowContext(ctx,
		"SELECT HOST_NAME FROM V$INSTANCE").Scan(&host); err == nil {
		info.Host = host
	}

	var maxConns int64
	if err := d.client.DB.QueryRowContext(ctx,
		"SELECT VALUE FROM V$PARAMETER WHERE NAME = 'sessions'").Scan(&maxConns); err == nil {
		info.MaxConnections = maxConns
	}

	var sessions int64
	if err := d.client.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM V$SESSION WHERE TYPE = 'USER'").Scan(&sessions); err == nil {
		info.ActiveSessions = sessions
	}

	var dbSizeMB float64
	if err := d.client.DB.QueryRowContext(ctx, `
		SELECT SUM(bytes) / 1024 / 1024 FROM DBA_DATA_FILES`).Scan(&dbSizeMB); err == nil {
		info.DatabaseSize = fmt.Sprintf("%.2f MiB", dbSizeMB)
	}

	return info, nil
}

func (d *Dao) GetActiveSessions(ctx context.Context) (int64, error) {
	var count int64
	if err := d.client.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM V$SESSION WHERE TYPE = 'USER'").Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to get active sessions: %w", err)
	}
	return count, nil
}

func (d *Dao) ListSchemas(ctx context.Context, nameFilter string) ([]database.Schema, error) {
	query := `
		SELECT t.owner, t.table_name
		FROM ALL_TABLES t
		WHERE t.owner NOT IN (
			'SYS','SYSTEM','OUTLN','DBSNMP','APPQOSSYS','DBSFWUSER',
			'GGSYS','ANONYMOUS','CTXSYS','DVSYS','DVF','GSMADMIN_INTERNAL',
			'MDSYS','OLAPSYS','XDB','WMSYS','OJVMSYS','ORDSYS','ORDDATA',
			'ORDPLUGINS','SI_INFORMTN_SCHEMA','LBACSYS','APEX_PUBLIC_USER'
		)`
	args := []any{}
	if nameFilter != "" {
		query += " AND (UPPER(t.owner) LIKE UPPER(:1) OR UPPER(t.table_name) LIKE UPPER(:2))"
		args = append(args, "%"+nameFilter+"%", "%"+nameFilter+"%")
	}
	query += " ORDER BY t.owner, t.table_name"

	rows, err := d.client.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var result []database.Schema
	idx := make(map[string]int)
	for rows.Next() {
		var owner, tableName string
		if err := rows.Scan(&owner, &tableName); err != nil {
			return nil, err
		}
		i, ok := idx[owner]
		if !ok {
			i = len(result)
			idx[owner] = i
			result = append(result, database.Schema{Schema: owner})
		}
		result[i].Tables = append(result[i].Tables, tableName)
	}
	return result, rows.Err()
}

func (d *Dao) GetTableColumns(ctx context.Context, schema, table string) ([]database.ColumnInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT c.column_name,
		       c.data_type ||
		           CASE WHEN c.data_type IN ('VARCHAR2','NVARCHAR2','CHAR','NCHAR') THEN '(' || c.char_length || ')' ELSE '' END ||
		           CASE WHEN c.data_type IN ('NUMBER') AND c.data_precision IS NOT NULL THEN
		               '(' || c.data_precision || CASE WHEN c.data_scale IS NOT NULL THEN ',' || c.data_scale ELSE '' END || ')'
		               ELSE '' END AS column_type,
		       CASE c.nullable WHEN 'Y' THEN 1 ELSE 0 END AS is_nullable,
		       c.data_default,
		       CASE WHEN pk.column_name IS NOT NULL THEN 1 ELSE 0 END AS is_pk,
		       c.column_id,
		       c.identity_column
		FROM ALL_TAB_COLUMNS c
		LEFT JOIN (
		    SELECT ac.column_name
		    FROM ALL_CONSTRAINTS ac
		    JOIN ALL_CONS_COLUMNS acc ON ac.constraint_name = acc.constraint_name AND ac.owner = acc.owner
		    WHERE ac.owner = :1 AND acc.table_name = :2 AND ac.constraint_type = 'P'
		) pk ON pk.column_name = c.column_name
		WHERE c.owner = :3 AND c.table_name = :4
		ORDER BY c.column_id`,
		schema, table, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get table columns: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var columns []database.ColumnInfo
	for rows.Next() {
		var col database.ColumnInfo
		var isNullable, isPK int
		var defaultVal sql.NullString
		var identityCol string
		if err := rows.Scan(&col.Name, &col.DataType, &isNullable, &defaultVal, &isPK, &col.Ordinal, &identityCol); err != nil {
			return nil, err
		}
		col.IsNullable = isNullable == 1
		col.IsPK = isPK == 1
		col.IsAutoGenerated = strings.EqualFold(identityCol, "YES")
		if defaultVal.Valid {
			s := strings.TrimSpace(defaultVal.String)
			col.Default = &s
		}
		columns = append(columns, col)
	}
	return columns, rows.Err()
}

func (d *Dao) GetTableConstraints(ctx context.Context, schema, table string) ([]database.ConstraintInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT ac.constraint_name,
		       CASE ac.constraint_type
		           WHEN 'P' THEN 'PRIMARY KEY'
		           WHEN 'U' THEN 'UNIQUE'
		           WHEN 'C' THEN 'CHECK'
		           ELSE ac.constraint_type
		       END,
		       LISTAGG(acc.column_name, ',') WITHIN GROUP (ORDER BY acc.position),
		       ac.search_condition
		FROM ALL_CONSTRAINTS ac
		JOIN ALL_CONS_COLUMNS acc ON ac.constraint_name = acc.constraint_name AND ac.owner = acc.owner
		WHERE ac.owner = :1 AND ac.table_name = :2
		  AND ac.constraint_type IN ('P','U','C')
		GROUP BY ac.constraint_name, ac.constraint_type, ac.search_condition
		ORDER BY ac.constraint_type, ac.constraint_name`,
		schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get table constraints: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var constraints []database.ConstraintInfo
	for rows.Next() {
		var c database.ConstraintInfo
		var colStr string
		var searchCond sql.NullString
		if err := rows.Scan(&c.Name, &c.Type, &colStr, &searchCond); err != nil {
			return nil, err
		}
		if colStr != "" {
			c.Columns = strings.Split(colStr, ",")
		}
		if searchCond.Valid {
			c.Def = searchCond.String
		}
		constraints = append(constraints, c)
	}
	return constraints, rows.Err()
}

func (d *Dao) GetTableForeignKeys(ctx context.Context, schema, table string) ([]database.ForeignKeyInfo, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT ac.constraint_name,
		       LISTAGG(acc.column_name, ',') WITHIN GROUP (ORDER BY acc.position),
		       rc.owner AS ref_schema,
		       rc.table_name AS ref_table,
		       LISTAGG(rcc.column_name, ',') WITHIN GROUP (ORDER BY rcc.position),
		       ac.delete_rule
		FROM ALL_CONSTRAINTS ac
		JOIN ALL_CONS_COLUMNS acc  ON ac.constraint_name  = acc.constraint_name  AND ac.owner = acc.owner
		JOIN ALL_CONSTRAINTS rc    ON ac.r_constraint_name = rc.constraint_name  AND ac.r_owner = rc.owner
		JOIN ALL_CONS_COLUMNS rcc  ON rc.constraint_name  = rcc.constraint_name AND rc.owner  = rcc.owner
		WHERE ac.owner = :1 AND ac.table_name = :2 AND ac.constraint_type = 'R'
		GROUP BY ac.constraint_name, rc.owner, rc.table_name, ac.delete_rule
		ORDER BY ac.constraint_name`,
		schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get foreign keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var fks []database.ForeignKeyInfo
	for rows.Next() {
		var fk database.ForeignKeyInfo
		var colStr, refColStr string
		if err := rows.Scan(&fk.Name, &colStr, &fk.ReferencedSchema, &fk.ReferencedTable,
			&refColStr, &fk.OnDelete); err != nil {
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
		SELECT ac.owner,
		       ac.table_name,
		       LISTAGG(acc.column_name, ',') WITHIN GROUP (ORDER BY acc.position),
		       LISTAGG(rcc.column_name, ',') WITHIN GROUP (ORDER BY rcc.position)
		FROM ALL_CONSTRAINTS ac
		JOIN ALL_CONS_COLUMNS acc  ON ac.constraint_name   = acc.constraint_name  AND ac.owner  = acc.owner
		JOIN ALL_CONSTRAINTS rc    ON ac.r_constraint_name = rc.constraint_name   AND ac.r_owner = rc.owner
		JOIN ALL_CONS_COLUMNS rcc  ON rc.constraint_name   = rcc.constraint_name  AND rc.owner  = rcc.owner
		WHERE rc.owner = :1 AND rc.table_name = :2 AND ac.constraint_type = 'R'
		GROUP BY ac.owner, ac.table_name, ac.constraint_name
		ORDER BY ac.owner, ac.table_name`,
		schema, table)
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
	var count int64
	err := d.client.DB.QueryRowContext(ctx, `
		SELECT NUM_ROWS FROM ALL_TABLES WHERE OWNER = :1 AND TABLE_NAME = :2`,
		schema, table).Scan(&count)
	if err != nil {
		// Fall back to exact count when statistics are unavailable.
		if err2 := d.client.DB.QueryRowContext(ctx,
			fmt.Sprintf("SELECT COUNT(*) FROM %s", quote.Table(schema, table))).Scan(&count); err2 != nil {
			return 0, false, err2
		}
		return count, false, nil
	}
	return count, true, nil
}

func (d *Dao) FetchTableRows(ctx context.Context, state *database.TableState, where, orderBy string) (string, []database.Row, error) {
	inner := fmt.Sprintf("SELECT * FROM %s", quote.Table(state.Schema, state.Table))
	args := []any{}
	argIdx := 1

	if where != "" {
		if err := database.SanitizeWhereClause(where); err != nil {
			return "", nil, err
		}
		inner += " WHERE " + where
	}
	if orderBy != "" {
		inner += " ORDER BY " + orderBy
	}

	offset := int64(state.RowCount())
	// Oracle 12c+ OFFSET/FETCH syntax.
	displayQuery := fmt.Sprintf("%s OFFSET %d ROWS FETCH NEXT %d ROWS ONLY", inner, offset, state.BatchSize)
	query := fmt.Sprintf("%s OFFSET :%d ROWS FETCH NEXT :%d ROWS ONLY", inner, argIdx, argIdx+1)
	args = append(args, offset, state.BatchSize)

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
		placeholders = append(placeholders, fmt.Sprintf(":%d", i))
		args = append(args, val)
		i++
	}

	if len(pkCols) == 1 {
		// Use RETURNING INTO to retrieve the generated PK.
		var pkVal any
		returningQuery := fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s) RETURNING %s INTO :%d",
			quote.Table(schema, table),
			strings.Join(cols, ", "),
			strings.Join(placeholders, ", "),
			quote.Ident(pkCols[0]),
			i,
		)
		args = append(args, sql.Out{Dest: &pkVal})
		if _, err := d.client.DB.ExecContext(ctx, returningQuery, args...); err != nil {
			return database.PrimaryKey{}, fmt.Errorf("failed to insert row: %w", err)
		}
		return database.PrimaryKey{Columns: map[string]any{pkCols[0]: fmt.Sprint(pkVal)}}, nil
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
			setClauses = append(setClauses, fmt.Sprintf("%s = :%d", quote.Ident(col), idx))
			args = append(args, newVal)
			idx++
		}
	}
	if len(setClauses) == 0 {
		return nil
	}

	whereParts := make([]string, 0, len(pk.Columns))
	for col, val := range pk.Columns {
		whereParts = append(whereParts, fmt.Sprintf("%s = :%d", quote.Ident(col), idx))
		args = append(args, val)
		idx++
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
		whereParts := make([]string, 0, len(pk.Columns))
		args := make([]any, 0, len(pk.Columns))
		idx := 1
		for col, val := range pk.Columns {
			whereParts = append(whereParts, fmt.Sprintf("%s = :%d", quote.Ident(col), idx))
			args = append(args, val)
			idx++
		}
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
		"NUMBER",
		"NUMBER(10,2)",
		"NUMBER(19,0)",
		"FLOAT",
		"BINARY_FLOAT",
		"BINARY_DOUBLE",
		"VARCHAR2(255)",
		"VARCHAR2(4000)",
		"NVARCHAR2(255)",
		"CHAR(1)",
		"CLOB",
		"NCLOB",
		"BLOB",
		"DATE",
		"TIMESTAMP",
		"TIMESTAMP WITH TIME ZONE",
		"TIMESTAMP WITH LOCAL TIME ZONE",
		"INTERVAL YEAR TO MONTH",
		"INTERVAL DAY TO SECOND",
		"RAW(16)",
	}
}

func (d *Dao) DefaultPKType() string { return "NUMBER" }

func (d *Dao) DefaultCreateTableDDL(schema, tableName string) string {
	return fmt.Sprintf(
		"CREATE TABLE %s (id NUMBER GENERATED ALWAYS AS IDENTITY PRIMARY KEY)",
		quote.Table(schema, tableName),
	)
}

func (d *Dao) GetViewDDL(ctx context.Context, schema, view string) (string, error) {
	var text string
	err := d.client.DB.QueryRowContext(ctx,
		"SELECT TEXT FROM ALL_VIEWS WHERE OWNER = :1 AND VIEW_NAME = :2",
		schema, view).Scan(&text)
	if err != nil {
		return "", fmt.Errorf("failed to get view DDL: %w", err)
	}
	return fmt.Sprintf("CREATE OR REPLACE VIEW %s AS\n%s",
		quote.Table(schema, view), strings.TrimSpace(text)), nil
}

func (d *Dao) GetTableDDL(ctx context.Context, schema, table string) (string, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT c.column_name,
		       c.data_type ||
		           CASE WHEN c.data_type IN ('VARCHAR2','NVARCHAR2','CHAR','NCHAR') THEN '(' || c.char_length || ')' ELSE '' END ||
		           CASE WHEN c.data_type IN ('NUMBER') AND c.data_precision IS NOT NULL THEN
		               '(' || c.data_precision || CASE WHEN c.data_scale IS NOT NULL THEN ',' || c.data_scale ELSE '' END || ')'
		               ELSE '' END AS column_type,
		       c.nullable,
		       c.identity_column
		FROM ALL_TAB_COLUMNS c
		WHERE c.owner = :1 AND c.table_name = :2
		ORDER BY c.column_id`,
		schema, table)
	if err != nil {
		return "", fmt.Errorf("failed to get table DDL: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sb strings.Builder
	fmt.Fprintf(&sb, "CREATE TABLE %s (\n", quote.Table(schema, table))

	first := true
	for rows.Next() {
		var name, typeName, nullable, identityCol string
		if err := rows.Scan(&name, &typeName, &nullable, &identityCol); err != nil {
			return "", err
		}
		if !first {
			sb.WriteString(",\n")
		}
		first = false

		nullStr := " NOT NULL"
		if nullable == "Y" {
			nullStr = ""
		}
		identStr := ""
		if strings.EqualFold(identityCol, "YES") {
			identStr = " GENERATED ALWAYS AS IDENTITY"
		}
		fmt.Fprintf(&sb, "    %s %s%s%s", quote.Ident(name), typeName, identStr, nullStr)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	// Append PRIMARY KEY constraint.
	pkRows, err := d.client.DB.QueryContext(ctx, `
		SELECT ac.constraint_name,
		       LISTAGG(acc.column_name, ', ') WITHIN GROUP (ORDER BY acc.position)
		FROM ALL_CONSTRAINTS ac
		JOIN ALL_CONS_COLUMNS acc ON ac.constraint_name = acc.constraint_name AND ac.owner = acc.owner
		WHERE ac.owner = :1 AND ac.table_name = :2 AND ac.constraint_type = 'P'
		GROUP BY ac.constraint_name`,
		schema, table)
	if err == nil {
		defer func() { _ = pkRows.Close() }()
		if pkRows.Next() {
			var pkName, pkCols string
			if pkRows.Scan(&pkName, &pkCols) == nil {
				quotedCols := make([]string, 0)
				for _, col := range strings.Split(pkCols, ", ") {
					quotedCols = append(quotedCols, quote.Ident(strings.TrimSpace(col)))
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
		fmt.Sprintf("DROP TABLE %s", quote.Table(schema, table)))
	if err != nil {
		return fmt.Errorf("failed to drop table: %w", err)
	}
	log.Info().Str("schema", schema).Str("table", table).Msg("Table dropped")
	return nil
}

func (d *Dao) RenameTable(ctx context.Context, schema, old, newName string) error {
	// Oracle RENAME works within the same schema only; no cross-schema renames.
	_, err := d.client.DB.ExecContext(ctx,
		fmt.Sprintf("RENAME %s TO %s", quote.Ident(old), quote.Ident(newName)))
	if err != nil {
		return fmt.Errorf("failed to rename table: %w", err)
	}
	log.Info().Str("schema", schema).Str("old", old).Str("new", newName).Msg("Table renamed")
	return nil
}

func (d *Dao) RenameColumn(ctx context.Context, schema, table, old, newName string) error {
	_, err := d.client.DB.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s",
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
		SELECT ai.index_name,
		       ai.uniqueness,
		       CASE WHEN ac.constraint_type = 'P' THEN 1 ELSE 0 END AS is_primary,
		       ai.index_type,
		       LISTAGG(aic.column_name, ',') WITHIN GROUP (ORDER BY aic.column_position)
		FROM ALL_INDEXES ai
		JOIN ALL_IND_COLUMNS aic ON ai.index_name = aic.index_name AND ai.owner = aic.index_owner
		LEFT JOIN ALL_CONSTRAINTS ac ON ac.index_name = ai.index_name AND ac.owner = ai.owner AND ac.constraint_type = 'P'
		WHERE ai.owner = :1 AND ai.table_name = :2
		GROUP BY ai.index_name, ai.uniqueness, ac.constraint_type, ai.index_type
		ORDER BY is_primary DESC, ai.index_name`,
		schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get indexes: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var indexes []database.IndexInfo
	for rows.Next() {
		var name, uniqueness, indexType, colStr string
		var isPrimary int
		if err := rows.Scan(&name, &uniqueness, &isPrimary, &indexType, &colStr); err != nil {
			return nil, err
		}
		indexes = append(indexes, database.IndexInfo{
			Name:      name,
			Columns:   strings.Split(colStr, ","),
			IsUnique:  uniqueness == "UNIQUE",
			IsPrimary: isPrimary == 1,
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
	_, err := d.client.DB.ExecContext(ctx,
		fmt.Sprintf("DROP INDEX %s", quote.Ident(indexName)))
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
		// Oracle 12c+ OFFSET/FETCH for pagination.
		query = fmt.Sprintf(
			"SELECT * FROM (%s) OFFSET %d ROWS FETCH NEXT %d ROWS ONLY",
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

func (d *Dao) ExplainPlan(ctx context.Context, sqlStr string) (string, error) {
	conn, err := d.client.DB.Conn(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get connection for plan: %w", err)
	}
	defer func() { _ = conn.Close() }()

	stmtID := fmt.Sprintf("vi-sql-%d", time.Now().UnixNano())

	if _, err := conn.ExecContext(ctx,
		fmt.Sprintf("EXPLAIN PLAN SET STATEMENT_ID = '%s' FOR %s", stmtID, sqlStr)); err != nil {
		return "", fmt.Errorf("failed to explain query: %w", err)
	}

	rows, err := conn.QueryContext(ctx,
		"SELECT PLAN_TABLE_OUTPUT FROM TABLE(DBMS_XPLAN.DISPLAY('PLAN_TABLE', :1, 'ALL'))", stmtID)
	if err != nil {
		return "", fmt.Errorf("failed to read explain plan: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return "", err
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return strings.Join(lines, "\n"), nil
}

// ExplainAnalyze delegates to ExplainPlan; Oracle's actual-execution stats
// require V$SQL_PLAN_STATISTICS and are complex to surface generically.
func (d *Dao) ExplainAnalyze(ctx context.Context, sqlStr string) (string, error) {
	return d.ExplainPlan(ctx, sqlStr)
}

func (d *Dao) GetTableColumnNames(ctx context.Context, schema, table string) ([]string, error) {
	rows, err := d.client.DB.QueryContext(ctx, `
		SELECT column_name FROM ALL_TAB_COLUMNS
		WHERE owner = :1 AND table_name = :2
		ORDER BY column_id`,
		schema, table)
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
		SELECT acc.column_name
		FROM ALL_CONSTRAINTS ac
		JOIN ALL_CONS_COLUMNS acc ON ac.constraint_name = acc.constraint_name AND ac.owner = acc.owner
		WHERE ac.owner = :1 AND ac.table_name = :2 AND ac.constraint_type = 'P'
		ORDER BY acc.position`,
		schema, table)
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
