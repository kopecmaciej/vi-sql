package gaussdbgo_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/HuaweiCloudDeveloper/gaussdb-go"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbconn"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// todo: GaussD8 暂时不支持 临时表Serial自增序列
//func TestConnSendBatch(t *testing.T) {
//	t.Parallel()
//
//	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
//	defer cancel()
//
//	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
//
//		sql := `create temporary table ledger(
//	  id serial primary key,
//	  description varchar not null,
//	  amount int not null
//	);`
//		mustExec(t, conn, sql)
//
//		batch := &gaussdbgo.Batch{}
//		batch.Queue("insert into ledger(description, amount) values($1, $2)", "q1", 1)
//		batch.Queue("insert into ledger(description, amount) values($1, $2)", "q2", 2)
//		batch.Queue("insert into ledger(description, amount) values($1, $2)", "q3", 3)
//		batch.Queue("select id, description, amount from ledger order by id")
//		batch.Queue("select id, description, amount from ledger order by id")
//		batch.Queue("select * from ledger where false")
//		batch.Queue("select sum(amount) from ledger")
//
//		br := conn.SendBatch(ctx, batch)
//
//		ct, err := br.Exec()
//		if err != nil {
//			t.Error(err)
//		}
//		if ct.RowsAffected() != 1 {
//			t.Errorf("ct.RowsAffected() => %v, want %v", ct.RowsAffected(), 1)
//		}
//
//		ct, err = br.Exec()
//		if err != nil {
//			t.Error(err)
//		}
//		if ct.RowsAffected() != 1 {
//			t.Errorf("ct.RowsAffected() => %v, want %v", ct.RowsAffected(), 1)
//		}
//
//		ct, err = br.Exec()
//		if err != nil {
//			t.Error(err)
//		}
//		if ct.RowsAffected() != 1 {
//			t.Errorf("ct.RowsAffected() => %v, want %v", ct.RowsAffected(), 1)
//		}
//
//		selectFromLedgerExpectedRows := []struct {
//			id          int32
//			description string
//			amount      int32
//		}{
//			{1, "q1", 1},
//			{2, "q2", 2},
//			{3, "q3", 3},
//		}
//
//		rows, err := br.Query()
//		if err != nil {
//			t.Error(err)
//		}
//
//		var id int32
//		var description string
//		var amount int32
//		rowCount := 0
//
//		for rows.Next() {
//			if rowCount >= len(selectFromLedgerExpectedRows) {
//				t.Fatalf("got too many rows: %d", rowCount)
//			}
//
//			if err := rows.Scan(&id, &description, &amount); err != nil {
//				t.Fatalf("row %d: %v", rowCount, err)
//			}
//
//			if id != selectFromLedgerExpectedRows[rowCount].id {
//				t.Errorf("id => %v, want %v", id, selectFromLedgerExpectedRows[rowCount].id)
//			}
//			if description != selectFromLedgerExpectedRows[rowCount].description {
//				t.Errorf("description => %v, want %v", description, selectFromLedgerExpectedRows[rowCount].description)
//			}
//			if amount != selectFromLedgerExpectedRows[rowCount].amount {
//				t.Errorf("amount => %v, want %v", amount, selectFromLedgerExpectedRows[rowCount].amount)
//			}
//
//			rowCount++
//		}
//
//		if rows.Err() != nil {
//			t.Fatal(rows.Err())
//		}
//
//		rowCount = 0
//		rows, _ = br.Query()
//		_, err = gaussdbgo.ForEachRow(rows, []any{&id, &description, &amount}, func() error {
//			if id != selectFromLedgerExpectedRows[rowCount].id {
//				t.Errorf("id => %v, want %v", id, selectFromLedgerExpectedRows[rowCount].id)
//			}
//			if description != selectFromLedgerExpectedRows[rowCount].description {
//				t.Errorf("description => %v, want %v", description, selectFromLedgerExpectedRows[rowCount].description)
//			}
//			if amount != selectFromLedgerExpectedRows[rowCount].amount {
//				t.Errorf("amount => %v, want %v", amount, selectFromLedgerExpectedRows[rowCount].amount)
//			}
//
//			rowCount++
//
//			return nil
//		})
//		if err != nil {
//			t.Error(err)
//		}
//
//		err = br.QueryRow().Scan(&id, &description, &amount)
//		if !errors.Is(err, gaussdbgo.ErrNoRows) {
//			t.Errorf("expected gaussdbgo.ErrNoRows but got: %v", err)
//		}
//
//		err = br.QueryRow().Scan(&amount)
//		if err != nil {
//			t.Error(err)
//		}
//		if amount != 6 {
//			t.Errorf("amount => %v, want %v", amount, 6)
//		}
//
//		err = br.Close()
//		if err != nil {
//			t.Fatal(err)
//		}
//	})
//}

// todo: GaussD8 暂时不支持 临时表Serial自增序列
//func TestConnSendBatchQueuedQuery(t *testing.T) {
//	t.Parallel()
//
//	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
//	defer cancel()
//
//	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
//
//		sql := `create temporary table ledger(
//	  id serial primary key,
//	  description varchar not null,
//	  amount int not null
//	);`
//		mustExec(t, conn, sql)
//
//		batch := &gaussdbgo.Batch{}
//
//		batch.Queue("insert into ledger(description, amount) values($1, $2)", "q1", 1).Exec(func(ct gaussdbconn.CommandTag) error {
//			assert.EqualValues(t, 1, ct.RowsAffected())
//			return nil
//		})
//
//		batch.Queue("insert into ledger(description, amount) values($1, $2)", "q2", 2).Exec(func(ct gaussdbconn.CommandTag) error {
//			assert.EqualValues(t, 1, ct.RowsAffected())
//			return nil
//		})
//
//		batch.Queue("insert into ledger(description, amount) values($1, $2)", "q3", 3).Exec(func(ct gaussdbconn.CommandTag) error {
//			assert.EqualValues(t, 1, ct.RowsAffected())
//			return nil
//		})
//
//		selectFromLedgerExpectedRows := []struct {
//			id          int32
//			description string
//			amount      int32
//		}{
//			{1, "q1", 1},
//			{2, "q2", 2},
//			{3, "q3", 3},
//		}
//
//		batch.Queue("select id, description, amount from ledger order by id").Query(func(rows gaussdbgo.Rows) error {
//			rowCount := 0
//			var id int32
//			var description string
//			var amount int32
//			_, err := gaussdbgo.ForEachRow(rows, []any{&id, &description, &amount}, func() error {
//				assert.Equal(t, selectFromLedgerExpectedRows[rowCount].id, id)
//				assert.Equal(t, selectFromLedgerExpectedRows[rowCount].description, description)
//				assert.Equal(t, selectFromLedgerExpectedRows[rowCount].amount, amount)
//				rowCount++
//
//				return nil
//			})
//			assert.NoError(t, err)
//			return nil
//		})
//
//		batch.Queue("select id, description, amount from ledger order by id").Query(func(rows gaussdbgo.Rows) error {
//			rowCount := 0
//			var id int32
//			var description string
//			var amount int32
//			_, err := gaussdbgo.ForEachRow(rows, []any{&id, &description, &amount}, func() error {
//				assert.Equal(t, selectFromLedgerExpectedRows[rowCount].id, id)
//				assert.Equal(t, selectFromLedgerExpectedRows[rowCount].description, description)
//				assert.Equal(t, selectFromLedgerExpectedRows[rowCount].amount, amount)
//				rowCount++
//
//				return nil
//			})
//			assert.NoError(t, err)
//			return nil
//		})
//
//		batch.Queue("select * from ledger where false").QueryRow(func(row gaussdbgo.Row) error {
//			err := row.Scan(nil, nil, nil)
//			assert.ErrorIs(t, err, gaussdbgo.ErrNoRows)
//			return nil
//		})
//
//		batch.Queue("select sum(amount) from ledger").QueryRow(func(row gaussdbgo.Row) error {
//			var sumAmount int32
//			err := row.Scan(&sumAmount)
//			assert.NoError(t, err)
//			assert.EqualValues(t, 6, sumAmount)
//			return nil
//		})
//
//		err := conn.SendBatch(ctx, batch).Close()
//		assert.NoError(t, err)
//	})
//}

// todo: GaussD8 暂时不支持 临时表Serial自增序列
//func TestConnSendBatchMany(t *testing.T) {
//	t.Parallel()
//
//	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
//	defer cancel()
//
//	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
//		sql := `create temporary table ledger(
//	  id serial primary key,
//	  description varchar not null,
//	  amount int not null
//	);`
//		mustExec(t, conn, sql)
//
//		batch := &gaussdbgo.Batch{}
//
//		numInserts := 1000
//
//		for i := 0; i < numInserts; i++ {
//			batch.Queue("insert into ledger(description, amount) values($1, $2)", "q1", 1)
//		}
//		batch.Queue("select count(*) from ledger")
//
//		br := conn.SendBatch(ctx, batch)
//
//		for i := 0; i < numInserts; i++ {
//			ct, err := br.Exec()
//			assert.NoError(t, err)
//			assert.EqualValues(t, 1, ct.RowsAffected())
//		}
//
//		var actualInserts int
//		err := br.QueryRow().Scan(&actualInserts)
//		assert.NoError(t, err)
//		assert.EqualValues(t, numInserts, actualInserts)
//
//		err = br.Close()
//		require.NoError(t, err)
//	})
//}

func TestConnSendBatchReadResultsWhenNothingQueued(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		batch := &gaussdbgo.Batch{}
		br := conn.SendBatch(ctx, batch)
		commandTag, err := br.Exec()
		require.Equal(t, "", commandTag.String())
		require.EqualError(t, err, "no more results in batch")
		err = br.Close()
		require.NoError(t, err)
	})
}

func TestConnSendBatchReadMoreResultsThanQueriesSent(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		batch := &gaussdbgo.Batch{}
		batch.Queue("select 1")
		br := conn.SendBatch(ctx, batch)
		commandTag, err := br.Exec()
		require.Equal(t, "SELECT 1", commandTag.String())
		require.NoError(t, err)
		commandTag, err = br.Exec()
		require.Equal(t, "", commandTag.String())
		require.EqualError(t, err, "no more results in batch")
		err = br.Close()
		require.NoError(t, err)
	})
}

func TestConnSendBatchWithPreparedStatement(t *testing.T) {
	t.Parallel()

	modes := []gaussdbgo.QueryExecMode{
		gaussdbgo.QueryExecModeCacheStatement,
		gaussdbgo.QueryExecModeCacheDescribe,
		gaussdbgo.QueryExecModeDescribeExec,
		gaussdbgo.QueryExecModeExec,
		// Don't test simple mode with prepared statements.
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, modes, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		_, err := conn.Prepare(ctx, "ps1", "select n from generate_series(0,$1::int) n")
		if err != nil {
			t.Fatal(err)
		}

		batch := &gaussdbgo.Batch{}

		queryCount := 3
		for i := 0; i < queryCount; i++ {
			batch.Queue("ps1", 5)
		}

		br := conn.SendBatch(ctx, batch)

		for i := 0; i < queryCount; i++ {
			rows, err := br.Query()
			if err != nil {
				t.Fatal(err)
			}

			for k := 0; rows.Next(); k++ {
				var n int
				if err := rows.Scan(&n); err != nil {
					t.Fatal(err)
				}
				if n != k {
					t.Fatalf("n => %v, want %v", n, k)
				}
			}

			if rows.Err() != nil {
				t.Fatal(rows.Err())
			}
		}

		err = br.Close()
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestConnSendBatchWithQueryRewriter(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		batch := &gaussdbgo.Batch{}
		batch.Queue("something to be replaced", &testQueryRewriter{sql: "select $1::int", args: []any{1}})
		batch.Queue("something else to be replaced", &testQueryRewriter{sql: "select $1::text", args: []any{"hello"}})
		batch.Queue("more to be replaced", &testQueryRewriter{sql: "select $1::int", args: []any{3}})

		br := conn.SendBatch(ctx, batch)

		var n int32
		err := br.QueryRow().Scan(&n)
		require.NoError(t, err)
		require.EqualValues(t, 1, n)

		var s string
		err = br.QueryRow().Scan(&s)
		require.NoError(t, err)
		require.Equal(t, "hello", s)

		err = br.QueryRow().Scan(&n)
		require.NoError(t, err)
		require.EqualValues(t, 3, n)

		err = br.Close()
		require.NoError(t, err)
	})
}

func TestConnSendBatchWithPreparedStatementAndStatementCacheDisabled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	config, err := gaussdbgo.ParseConfig(os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
	require.NoError(t, err)

	config.DefaultQueryExecMode = gaussdbgo.QueryExecModeDescribeExec
	config.StatementCacheCapacity = 0
	config.DescriptionCacheCapacity = 0

	conn := mustConnect(t, config)
	defer closeConn(t, conn)

	_, err = conn.Prepare(ctx, "ps1", "select n from generate_series(0,$1::int) n")
	if err != nil {
		t.Fatal(err)
	}

	batch := &gaussdbgo.Batch{}

	queryCount := 3
	for i := 0; i < queryCount; i++ {
		batch.Queue("ps1", 5)
	}

	br := conn.SendBatch(ctx, batch)

	for i := 0; i < queryCount; i++ {
		rows, err := br.Query()
		if err != nil {
			t.Fatal(err)
		}

		for k := 0; rows.Next(); k++ {
			var n int
			if err := rows.Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != k {
				t.Fatalf("n => %v, want %v", n, k)
			}
		}

		if rows.Err() != nil {
			t.Fatal(rows.Err())
		}
	}

	err = br.Close()
	if err != nil {
		t.Fatal(err)
	}

	ensureConnValid(t, conn)
}

func TestConnSendBatchCloseRowsPartiallyRead(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {

		batch := &gaussdbgo.Batch{}
		batch.Queue("select n from generate_series(0,5) n")
		batch.Queue("select n from generate_series(0,5) n")

		br := conn.SendBatch(ctx, batch)

		rows, err := br.Query()
		if err != nil {
			t.Error(err)
		}

		for i := 0; i < 3; i++ {
			if !rows.Next() {
				t.Error("expected a row to be available")
			}

			var n int
			if err := rows.Scan(&n); err != nil {
				t.Error(err)
			}
			if n != i {
				t.Errorf("n => %v, want %v", n, i)
			}
		}

		rows.Close()

		rows, err = br.Query()
		if err != nil {
			t.Error(err)
		}

		for i := 0; rows.Next(); i++ {
			var n int
			if err := rows.Scan(&n); err != nil {
				t.Error(err)
			}
			if n != i {
				t.Errorf("n => %v, want %v", n, i)
			}
		}

		if rows.Err() != nil {
			t.Error(rows.Err())
		}

		err = br.Close()
		if err != nil {
			t.Fatal(err)
		}

	})
}

func TestConnSendBatchQueryError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {

		batch := &gaussdbgo.Batch{}
		batch.Queue("select n from generate_series(0,5) n where 100/(5-n) > 0")
		batch.Queue("select n from generate_series(0,5) n")

		br := conn.SendBatch(ctx, batch)

		rows, err := br.Query()
		if err != nil {
			t.Error(err)
		}

		for i := 0; rows.Next(); i++ {
			var n int
			if err := rows.Scan(&n); err != nil {
				t.Error(err)
			}
			if n != i {
				t.Errorf("n => %v, want %v", n, i)
			}
		}

		if gaussdbError, ok := rows.Err().(*gaussdbconn.GaussdbError); !(ok && gaussdbError.Code == "22012") {
			t.Errorf("rows.Err() => %v, want error code %v", rows.Err(), 22012)
		}

		err = br.Close()
		if gaussdbError, ok := err.(*gaussdbconn.GaussdbError); !(ok && gaussdbError.Code == "22012") {
			t.Errorf("br.Close() => %v, want error code %v", err, 22012)
		}

	})
}

func TestConnSendBatchQuerySyntaxError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {

		batch := &gaussdbgo.Batch{}
		batch.Queue("select 1 1")

		br := conn.SendBatch(ctx, batch)

		var n int32
		err := br.QueryRow().Scan(&n)
		if gaussdbError, ok := err.(*gaussdbconn.GaussdbError); !(ok && gaussdbError.Code == "42601") {
			t.Errorf("rows.Err() => %v, want error code %v", err, 42601)
		}

		err = br.Close()
		if err == nil {
			t.Error("Expected error")
		}

	})
}

// todo: GaussD8 暂时不支持 临时表Serial自增序列
//func TestConnSendBatchQueryRowInsert(t *testing.T) {
//	t.Parallel()
//
//	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
//	defer cancel()
//
//	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
//
//		sql := `create temporary table ledger(
//	  id serial primary key,
//	  description varchar not null,
//	  amount int not null
//	);`
//		mustExec(t, conn, sql)
//
//		batch := &gaussdbgo.Batch{}
//		batch.Queue("select 1")
//		batch.Queue("insert into ledger(description, amount) values($1, $2),($1, $2)", "q1", 1)
//
//		br := conn.SendBatch(ctx, batch)
//
//		var value int
//		err := br.QueryRow().Scan(&value)
//		if err != nil {
//			t.Error(err)
//		}
//
//		ct, err := br.Exec()
//		if err != nil {
//			t.Error(err)
//		}
//		if ct.RowsAffected() != 2 {
//			t.Errorf("ct.RowsAffected() => %v, want %v", ct.RowsAffected(), 2)
//		}
//
//		br.Close()
//
//	})
//}

// todo: GaussD8 暂时不支持 临时表Serial自增序列
//func TestConnSendBatchQueryPartialReadInsert(t *testing.T) {
//	t.Parallel()
//
//	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
//	defer cancel()
//
//	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
//
//		sql := `create temporary table ledger(
//	  id serial primary key,
//	  description varchar not null,
//	  amount int not null
//	);`
//		mustExec(t, conn, sql)
//
//		batch := &gaussdbgo.Batch{}
//		batch.Queue("select 1 union all select 2 union all select 3")
//		batch.Queue("insert into ledger(description, amount) values($1, $2),($1, $2)", "q1", 1)
//
//		br := conn.SendBatch(ctx, batch)
//
//		rows, err := br.Query()
//		if err != nil {
//			t.Error(err)
//		}
//		rows.Close()
//
//		ct, err := br.Exec()
//		if err != nil {
//			t.Error(err)
//		}
//		if ct.RowsAffected() != 2 {
//			t.Errorf("ct.RowsAffected() => %v, want %v", ct.RowsAffected(), 2)
//		}
//
//		br.Close()
//
//	})
//}

func TestTxSendBatch(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {

		sql := `drop table if exists ledger1;
				create table ledger1(
				  id serial primary key,
				  description varchar not null
				);`
		mustExec(t, conn, sql)

		sql = `create temporary table ledger2(
				  id int primary key,
				  amount int not null
				);`
		mustExec(t, conn, sql)

		tx, _ := conn.Begin(ctx)
		batch := &gaussdbgo.Batch{}
		batch.Queue("insert into ledger1(description) values($1) returning id", "q1")

		br := tx.SendBatch(context.Background(), batch)

		var id int
		err := br.QueryRow().Scan(&id)
		if err != nil {
			t.Error(err)
		}
		br.Close()

		batch = &gaussdbgo.Batch{}
		batch.Queue("insert into ledger2(id,amount) values($1, $2)", id, 2)
		batch.Queue("select amount from ledger2 where id = $1", id)

		br = tx.SendBatch(ctx, batch)

		ct, err := br.Exec()
		if err != nil {
			t.Error(err)
		}
		if ct.RowsAffected() != 1 {
			t.Errorf("ct.RowsAffected() => %v, want %v", ct.RowsAffected(), 1)
		}

		var amount int
		err = br.QueryRow().Scan(&amount)
		if err != nil {
			t.Error(err)
		}

		br.Close()
		tx.Commit(ctx)

		var count int
		conn.QueryRow(ctx, "select count(1) from ledger1 where id = $1", id).Scan(&count)
		if count != 1 {
			t.Errorf("count => %v, want %v", count, 1)
		}

		err = br.Close()
		if err != nil {
			t.Fatal(err)
		}

	})
}

// todo GaussDB 暂时不支持 临时表Serial自增序列
/*func TestTxSendBatchRollback(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {

		sql := `create temporary table ledger1(
	  id serial primary key,
	  description varchar not null
	);`
		mustExec(t, conn, sql)

		tx, _ := conn.Begin(ctx)
		batch := &gaussdbgo.Batch{}
		batch.Queue("insert into ledger1(description) values($1) returning id", "q1")

		br := tx.SendBatch(ctx, batch)

		var id int
		err := br.QueryRow().Scan(&id)
		if err != nil {
			t.Error(err)
		}
		br.Close()
		tx.Rollback(ctx)

		row := conn.QueryRow(ctx, "select count(1) from ledger1 where id = $1", id)
		var count int
		row.Scan(&count)
		if count != 0 {
			t.Errorf("count => %v, want %v", count, 0)
		}

	})
}*/

func TestSendBatchErrorWhileReadingResultsWithoutCallback(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		batch := &gaussdbgo.Batch{}
		batch.Queue("select 4 / $1::int", 0)

		batchResult := conn.SendBatch(ctx, batch)

		_, execErr := batchResult.Exec()
		require.Error(t, execErr)

		closeErr := batchResult.Close()
		require.Equal(t, execErr, closeErr)

		// Try to use the connection.
		_, err := conn.Exec(ctx, "select 1")
		require.NoError(t, err)
	})
}

func TestSendBatchErrorWhileReadingResultsWithExecWhereSomeRowsAreReturned(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		batch := &gaussdbgo.Batch{}
		batch.Queue("select 4 / n from generate_series(-2, 2) n")

		batchResult := conn.SendBatch(ctx, batch)

		_, execErr := batchResult.Exec()
		require.Error(t, execErr)

		closeErr := batchResult.Close()
		require.Equal(t, execErr, closeErr)

		// Try to use the connection.
		_, err := conn.Exec(ctx, "select 1")
		require.NoError(t, err)
	})
}

func TestConnBeginBatchDeferredError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {

		mustExec(t, conn, `create temporary table t (
		id text primary key,
		n int not null,
		unique (n) deferrable initially deferred
	);

	insert into t (id, n) values ('a', 1), ('b', 2), ('c', 3);`)

		batch := &gaussdbgo.Batch{}

		batch.Queue(`update t set n=n+1 where id='b' returning *`)

		br := conn.SendBatch(ctx, batch)

		rows, err := br.Query()
		if err != nil {
			t.Error(err)
		}

		for rows.Next() {
			var id string
			var n int32
			err = rows.Scan(&id, &n)
			if err != nil {
				t.Fatal(err)
			}
		}

		err = br.Close()
		if err == nil {
			t.Fatal("expected error 23505 but got none")
		}

		if err, ok := err.(*gaussdbconn.GaussdbError); !ok || err.Code != "23505" {
			t.Fatalf("expected error 23505, got %v", err)
		}

	})
}

func TestConnSendBatchNoStatementCache(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	config := mustParseConfig(t, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
	config.DefaultQueryExecMode = gaussdbgo.QueryExecModeDescribeExec
	config.StatementCacheCapacity = 0
	config.DescriptionCacheCapacity = 0

	conn := mustConnect(t, config)
	defer closeConn(t, conn)

	testConnSendBatch(t, ctx, conn, 3)
}

func TestConnSendBatchPrepareStatementCache(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	config := mustParseConfig(t, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
	config.DefaultQueryExecMode = gaussdbgo.QueryExecModeCacheStatement
	config.StatementCacheCapacity = 32

	conn := mustConnect(t, config)
	defer closeConn(t, conn)

	testConnSendBatch(t, ctx, conn, 3)
}

func TestConnSendBatchDescribeStatementCache(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	config := mustParseConfig(t, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
	config.DefaultQueryExecMode = gaussdbgo.QueryExecModeCacheDescribe
	config.DescriptionCacheCapacity = 32

	conn := mustConnect(t, config)
	defer closeConn(t, conn)

	testConnSendBatch(t, ctx, conn, 3)
}

func testConnSendBatch(t *testing.T, ctx context.Context, conn *gaussdbgo.Conn, queryCount int) {
	batch := &gaussdbgo.Batch{}
	for j := 0; j < queryCount; j++ {
		batch.Queue("select n from generate_series(0,5) n")
	}

	br := conn.SendBatch(ctx, batch)

	for j := 0; j < queryCount; j++ {
		rows, err := br.Query()
		require.NoError(t, err)

		for k := 0; rows.Next(); k++ {
			var n int
			err := rows.Scan(&n)
			require.NoError(t, err)
			require.Equal(t, k, n)
		}

		require.NoError(t, rows.Err())
	}

	err := br.Close()
	require.NoError(t, err)
}

func TestSendBatchSimpleProtocol(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	config := mustParseConfig(t, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
	config.DefaultQueryExecMode = gaussdbgo.QueryExecModeSimpleProtocol

	conn := mustConnect(t, config)
	defer closeConn(t, conn)

	var batch gaussdbgo.Batch
	batch.Queue("SELECT 1::int")
	batch.Queue("SELECT 2::int; SELECT $1::int", 3)
	results := conn.SendBatch(ctx, &batch)
	rows, err := results.Query()
	assert.NoError(t, err)
	assert.True(t, rows.Next())
	values, err := rows.Values()
	assert.NoError(t, err)
	assert.EqualValues(t, 1, values[0])
	assert.False(t, rows.Next())

	rows, err = results.Query()
	assert.NoError(t, err)
	assert.True(t, rows.Next())
	values, err = rows.Values()
	assert.NoError(t, err)
	assert.EqualValues(t, 2, values[0])
	assert.False(t, rows.Next())

	rows, err = results.Query()
	assert.NoError(t, err)
	assert.True(t, rows.Next())
	values, err = rows.Values()
	assert.NoError(t, err)
	assert.EqualValues(t, 3, values[0])
	assert.False(t, rows.Next())
}

func TestConnSendBatchErrorDoesNotLeaveOrphanedPreparedStatement(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {

		mustExec(t, conn, `create temporary table foo(col1 text primary key);`)

		batch := &gaussdbgo.Batch{}
		batch.Queue("select col1 from foo")
		batch.Queue("select col1 from baz")
		err := conn.SendBatch(ctx, batch).Close()
		// opengauss, gaussdb return different error.
		require.Contains(t, err.Error(), lowerFirstLetterInError(`ERROR: Relation "baz" does not exist on`))
		var gaussdbConnErr *gaussdbconn.GaussdbError

		ok := errors.As(err, &gaussdbConnErr)
		require.True(t, ok)
		require.EqualValues(t, gaussdbConnErr.Code, "42P01")
		mustExec(t, conn, `create temporary table baz(col1 text primary key);`)

		// Since table baz now exists, the batch should succeed.

		batch = &gaussdbgo.Batch{}
		batch.Queue("select col1 from foo")
		batch.Queue("select col1 from baz")
		err = conn.SendBatch(ctx, batch).Close()
		require.NoError(t, err)
	})
}

func ExampleConn_SendBatch() {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	conn, err := gaussdbgo.Connect(ctx, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
	if err != nil {
		fmt.Printf("Unable to establish connection: %v", err)
		return
	}

	batch := &gaussdbgo.Batch{}
	batch.Queue("select 1 + 1").QueryRow(func(row gaussdbgo.Row) error {
		var n int32
		err := row.Scan(&n)
		if err != nil {
			return err
		}

		fmt.Println(n)

		return err
	})

	batch.Queue("select 1 + 2").QueryRow(func(row gaussdbgo.Row) error {
		var n int32
		err := row.Scan(&n)
		if err != nil {
			return err
		}

		fmt.Println(n)

		return err
	})

	batch.Queue("select 2 + 3").QueryRow(func(row gaussdbgo.Row) error {
		var n int32
		err := row.Scan(&n)
		if err != nil {
			return err
		}

		fmt.Println(n)

		return err
	})

	err = conn.SendBatch(ctx, batch).Close()
	if err != nil {
		fmt.Printf("SendBatch error: %v", err)
		return
	}

	// Output:
	// 2
	// 3
	// 5
}
