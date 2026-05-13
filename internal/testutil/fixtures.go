package testutil

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// RepoRoot returns the absolute path to the module root by walking up from
// the testutil package source file's location.
func RepoRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	// testutil is at internal/testutil/, walk up: testutil → internal → repo root
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func SampleSQLitePath() string {
	return filepath.Join(RepoRoot(), "sample-sql", "sample.sqlite.sql")
}

func SamplePostgresPath() string {
	return filepath.Join(RepoRoot(), "sample-sql", "sample.postgres.sql")
}

func SampleMySQLPath() string {
	return filepath.Join(RepoRoot(), "sample-sql", "sample.mysql.sql")
}

func SampleSQLServerPath() string {
	return filepath.Join(RepoRoot(), "sample-sql", "sample.mssql.sql")
}

func SampleMariaDBPath() string {
	return filepath.Join(RepoRoot(), "sample-sql", "sample.mariadb.sql")
}

func SampleOraclePath() string {
	return filepath.Join(RepoRoot(), "sample-sql", "sample.oracle.sql")
}

var (
	seedOnce sync.Once
	seedDir  string
	seedErr  error
)

// EnsureSeed guarantees that the test seed dir contains small CSV files
// (SEED_SIZE=small), running the generator if absent. Returns the seed dir.
// Panics on failure so callers in TestMain don't have to thread errors.
//
// Tests use /tmp/vi-sql-test-seed/ (separate from /tmp/vi-sql-seed/ which
// is used for manual testing with full data).
func EnsureSeed() string {
	seedOnce.Do(func() {
		dir := testSeedDir()
		matches, _ := filepath.Glob(filepath.Join(dir, "*.csv"))
		if len(matches) == 0 {
			cmd := exec.Command("go", "run", "./sample-sql/seed")
			cmd.Dir = RepoRoot()
			cmd.Env = append(os.Environ(), "SEED_SIZE=small", "SEED_DIR="+dir)
			out, err := cmd.CombinedOutput()
			if err != nil {
				seedErr = fmt.Errorf("run seed generator: %w\n%s", err, out)
				return
			}
		}
		seedDir = dir
	})
	if seedErr != nil {
		panic("testutil.EnsureSeed: " + seedErr.Error())
	}
	return seedDir
}

func testSeedDir() string {
	if d := os.Getenv("SEED_DIR"); d != "" {
		return d
	}
	return "/tmp/vi-sql-test-seed"
}

// LoadSQLiteFixture loads sqlPath into db.
// Lines starting with .import are handled by opening the CSV from csvDir and
// inserting rows directly; all other lines are executed as SQL via ExecContext.
//
// All operations run on a single dedicated connection so that the in-memory
// database (which is per-connection) stays consistent across the whole load.
func LoadSQLiteFixture(ctx context.Context, db *sql.DB, sqlPath, csvDir string) error {
	// A single connection ensures all operations see the same in-memory database.
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Close()

	content, err := os.ReadFile(sqlPath)
	if err != nil {
		return fmt.Errorf("read fixture: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024) // large rows in seed data
	var chunk strings.Builder

	flushChunk := func() error {
		s := strings.TrimSpace(chunk.String())
		chunk.Reset()
		if s == "" {
			return nil
		}
		if _, err := conn.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("exec SQL: %w", err)
		}
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip non-.import dot commands (e.g. .mode csv) — they are SQLite CLI
		// metacommands that ExecContext cannot handle.
		if strings.HasPrefix(trimmed, ".") && !strings.HasPrefix(trimmed, ".import") {
			continue
		}

		if !strings.HasPrefix(trimmed, ".import") {
			chunk.WriteString(line + "\n")
			continue
		}

		if err := flushChunk(); err != nil {
			return err
		}
		// Parse: .import --skip 1 <filepath> <table>
		parts := strings.Fields(line)
		if len(parts) < 5 {
			return fmt.Errorf("unexpected .import line: %q", line)
		}
		csvFile := filepath.Join(csvDir, filepath.Base(parts[3]))
		table := parts[4]
		if err := csvInsertIntoConn(ctx, conn, csvFile, table); err != nil {
			return fmt.Errorf("import %s into %s: %w", csvFile, table, err)
		}
	}
	return flushChunk()
}

// csvInsertIntoConn reads a CSV and bulk-inserts all rows into the named SQLite table.
// Rows are batched into multi-value INSERTs (capped at SQLite's 999-variable limit)
// inside a single transaction. This minimises both round-trips and race-detector overhead.
func csvInsertIntoConn(ctx context.Context, conn *sql.Conn, csvPath, table string) error {
	f, err := os.Open(csvPath)
	if err != nil {
		return err
	}
	defer f.Close()

	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return fmt.Errorf("read header: %w", err)
	}

	numCols := len(header)
	batchRows := 999 / numCols
	if batchRows < 1 {
		batchRows = 1
	}
	cols := strings.Join(header, ",")
	rowPH := "(" + strings.TrimSuffix(strings.Repeat("?,", numCols), ",") + ")"

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx for %s: %w", table, err)
	}
	defer tx.Rollback()

	// stmts caches prepared statements keyed by batch size to avoid recompiling
	// the same shape on every flush (the final batch may be smaller).
	stmts := make(map[int]*sql.Stmt)
	defer func() {
		for _, s := range stmts {
			s.Close()
		}
	}()
	prepareFor := func(n int) (*sql.Stmt, error) {
		if s, ok := stmts[n]; ok {
			return s, nil
		}
		ph := strings.TrimSuffix(strings.Repeat(rowPH+",", n), ",")
		s, err := tx.PrepareContext(ctx, fmt.Sprintf("INSERT INTO %s (%s) VALUES %s", table, cols, ph))
		if err != nil {
			return nil, err
		}
		stmts[n] = s
		return s, nil
	}

	var (
		batch  [][]string
		rowNum int
	)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		stmt, err := prepareFor(len(batch))
		if err != nil {
			return fmt.Errorf("prepare %d-row batch for %s: %w", len(batch), table, err)
		}
		args := make([]any, 0, len(batch)*numCols)
		for _, row := range batch {
			for _, v := range row {
				args = append(args, v)
			}
		}
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	for rowNum = 1; ; rowNum++ {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read row %d: %w", rowNum, err)
		}
		batch = append(batch, rec)
		if len(batch) == batchRows {
			if err := flush(); err != nil {
				return fmt.Errorf("flush batch ending at row %d into %s: %w", rowNum, table, err)
			}
		}
	}
	if err := flush(); err != nil {
		return fmt.Errorf("flush final batch into %s: %w", table, err)
	}
	return tx.Commit()
}
