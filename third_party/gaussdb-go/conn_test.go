package gaussdbgo_test

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/HuaweiCloudDeveloper/gaussdb-go"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbconn"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCrateDBConnect(t *testing.T) {
	t.Parallel()

	connString := os.Getenv(gaussdbgo.EnvGaussdbTestCratedbConnString)
	if connString == "" {
		t.Skipf("Skipping due to missing environment variable %v", gaussdbgo.EnvGaussdbTestCratedbConnString)
	}

	conn, err := gaussdbgo.Connect(context.Background(), connString)
	require.Nil(t, err)
	defer closeConn(t, conn)

	assert.Equal(t, connString, conn.Config().ConnString())

	var result int
	err = conn.QueryRow(context.Background(), "select 1 +1").Scan(&result)
	if err != nil {
		t.Fatalf("QueryRow Scan unexpectedly failed: %v", err)
	}
	if result != 2 {
		t.Errorf("bad result: %d", result)
	}
}

func TestConnect(t *testing.T) {
	t.Parallel()

	connString := os.Getenv(gaussdbgo.EnvGaussdbTestDatabase)
	config := mustParseConfig(t, connString)

	conn, err := gaussdbgo.ConnectConfig(context.Background(), config)
	if err != nil {
		t.Fatalf("Unable to establish connection: %v", err)
	}

	assertConfigsEqual(t, config, conn.Config(), "Conn.Config() returns original config")

	var currentDB string
	err = conn.QueryRow(context.Background(), "select current_database()").Scan(&currentDB)
	if err != nil {
		t.Fatalf("QueryRow Scan unexpectedly failed: %v", err)
	}
	if currentDB != config.Config.Database {
		t.Errorf("Did not connect to specified database (%v)", config.Config.Database)
	}

	var user string
	err = conn.QueryRow(context.Background(), "select current_user").Scan(&user)
	if err != nil {
		t.Fatalf("QueryRow Scan unexpectedly failed: %v", err)
	}
	if user != config.Config.User {
		t.Errorf("Did not connect as specified user (%v)", config.Config.User)
	}

	err = conn.Close(context.Background())
	if err != nil {
		t.Fatal("Unable to close connection")
	}
}

func TestConnectWithPreferSimpleProtocol(t *testing.T) {
	t.Parallel()

	connConfig := mustParseConfig(t, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
	connConfig.DefaultQueryExecMode = gaussdbgo.QueryExecModeSimpleProtocol

	conn := mustConnect(t, connConfig)
	defer closeConn(t, conn)

	// If simple protocol is used we should be able to correctly scan the result
	// into a gaussdbtype.Text as the integer will have been encoded in text.

	var s gaussdbtype.Text
	err := conn.QueryRow(context.Background(), "select $1::int4", 42).Scan(&s)
	require.NoError(t, err)
	require.Equal(t, gaussdbtype.Text{String: "42", Valid: true}, s)

	ensureConnValid(t, conn)
}

func TestConnectConfigRequiresConnConfigFromParseConfig(t *testing.T) {
	config := &gaussdbgo.ConnConfig{}
	require.PanicsWithValue(t, "config must be created by ParseConfig", func() {
		gaussdbgo.ConnectConfig(context.Background(), config)
	})
}

func TestConfigContainsConnStr(t *testing.T) {
	connStr := os.Getenv(gaussdbgo.EnvGaussdbTestDatabase)
	config, err := gaussdbgo.ParseConfig(connStr)
	require.NoError(t, err)
	assert.Equal(t, connStr, config.ConnString())
}

func TestConfigCopyReturnsEqualConfig(t *testing.T) {
	connString := "gaussdb://jack:secret@localhost:5432/mydb?application_name=gaussdbxtest&search_path=myschema&connect_timeout=5"
	original, err := gaussdbgo.ParseConfig(connString)
	require.NoError(t, err)

	copied := original.Copy()
	assertConfigsEqual(t, original, copied, t.Name())
}

func TestConfigCopyCanBeUsedToConnect(t *testing.T) {
	connString := os.Getenv(gaussdbgo.EnvGaussdbTestDatabase)
	original, err := gaussdbgo.ParseConfig(connString)
	require.NoError(t, err)

	copied := original.Copy()
	assert.NotPanics(t, func() {
		_, err = gaussdbgo.ConnectConfig(context.Background(), copied)
	})
	assert.NoError(t, err)
}

func TestParseConfigExtractsStatementCacheOptions(t *testing.T) {
	t.Parallel()

	config, err := gaussdbgo.ParseConfig("statement_cache_capacity=0")
	require.NoError(t, err)
	require.EqualValues(t, 0, config.StatementCacheCapacity)

	config, err = gaussdbgo.ParseConfig("statement_cache_capacity=42")
	require.NoError(t, err)
	require.EqualValues(t, 42, config.StatementCacheCapacity)

	config, err = gaussdbgo.ParseConfig("description_cache_capacity=0")
	require.NoError(t, err)
	require.EqualValues(t, 0, config.DescriptionCacheCapacity)

	config, err = gaussdbgo.ParseConfig("description_cache_capacity=42")
	require.NoError(t, err)
	require.EqualValues(t, 42, config.DescriptionCacheCapacity)

	//	default_query_exec_mode
	//		Possible values: "cache_statement", "cache_describe", "describe_exec", "exec", and "simple_protocol". See

	config, err = gaussdbgo.ParseConfig("default_query_exec_mode=cache_statement")
	require.NoError(t, err)
	require.Equal(t, gaussdbgo.QueryExecModeCacheStatement, config.DefaultQueryExecMode)

	config, err = gaussdbgo.ParseConfig("default_query_exec_mode=cache_describe")
	require.NoError(t, err)
	require.Equal(t, gaussdbgo.QueryExecModeCacheDescribe, config.DefaultQueryExecMode)

	config, err = gaussdbgo.ParseConfig("default_query_exec_mode=describe_exec")
	require.NoError(t, err)
	require.Equal(t, gaussdbgo.QueryExecModeDescribeExec, config.DefaultQueryExecMode)

	config, err = gaussdbgo.ParseConfig("default_query_exec_mode=exec")
	require.NoError(t, err)
	require.Equal(t, gaussdbgo.QueryExecModeExec, config.DefaultQueryExecMode)

	config, err = gaussdbgo.ParseConfig("default_query_exec_mode=simple_protocol")
	require.NoError(t, err)
	require.Equal(t, gaussdbgo.QueryExecModeSimpleProtocol, config.DefaultQueryExecMode)
}

func TestParseConfigExtractsDefaultQueryExecMode(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		connString           string
		defaultQueryExecMode gaussdbgo.QueryExecMode
	}{
		{"", gaussdbgo.QueryExecModeCacheStatement},
		{"default_query_exec_mode=cache_statement", gaussdbgo.QueryExecModeCacheStatement},
		{"default_query_exec_mode=cache_describe", gaussdbgo.QueryExecModeCacheDescribe},
		{"default_query_exec_mode=describe_exec", gaussdbgo.QueryExecModeDescribeExec},
		{"default_query_exec_mode=exec", gaussdbgo.QueryExecModeExec},
		{"default_query_exec_mode=simple_protocol", gaussdbgo.QueryExecModeSimpleProtocol},
	} {
		config, err := gaussdbgo.ParseConfig(tt.connString)
		require.NoError(t, err)
		require.Equalf(t, tt.defaultQueryExecMode, config.DefaultQueryExecMode, "connString: `%s`", tt.connString)
		require.Empty(t, config.RuntimeParams["default_query_exec_mode"])
	}
}

func TestParseConfigErrors(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		connString           string
		expectedErrSubstring string
	}{
		{"default_query_exec_mode=does_not_exist", "does_not_exist"},
	} {
		config, err := gaussdbgo.ParseConfig(tt.connString)
		require.Nil(t, config)
		require.ErrorContains(t, err, tt.expectedErrSubstring)
	}
}

func TestExec(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		if results := mustExec(t, conn, "create temporary table foo(id integer primary key);"); results.String() != "CREATE TABLE" {
			t.Error("Unexpected results from Exec")
		}

		// Accept parameters
		if results := mustExec(t, conn, "insert into foo(id) values($1)", 1); results.String() != "INSERT 0 1" {
			t.Errorf("Unexpected results from Exec: %v", results)
		}

		if results := mustExec(t, conn, "drop table foo;"); results.String() != "DROP TABLE" {
			t.Error("Unexpected results from Exec")
		}

		// Multiple statements can be executed -- last command tag is returned
		// todo GaussDB 暂时不支持 临时表Serial自增序列
		/*if results := mustExec(t, conn, "create temporary table foo(id serial primary key); drop table foo;"); results.String() != "DROP TABLE" {
			t.Error("Unexpected results from Exec")
		}*/

		// Can execute longer SQL strings than sharedBufferSize
		if results := mustExec(t, conn, strings.Repeat("select 42; ", 1000)); results.String() != "SELECT 1" {
			t.Errorf("Unexpected results from Exec: %v", results)
		}

		// Exec no-op which does not return a command tag
		if results := mustExec(t, conn, "--;"); results.String() != "" {
			t.Errorf("Unexpected results from Exec: %v", results)
		}
	})
}

type testQueryRewriter struct {
	sql  string
	args []any
}

func (qr *testQueryRewriter) RewriteQuery(ctx context.Context, conn *gaussdbgo.Conn, sql string, args []any) (newSQL string, newArgs []any, err error) {
	return qr.sql, qr.args, nil
}

func TestExecWithQueryRewriter(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		qr := testQueryRewriter{sql: "select $1::int", args: []any{42}}
		_, err := conn.Exec(ctx, "should be replaced", &qr)
		require.NoError(t, err)
	})
}

func TestExecFailure(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		if _, err := conn.Exec(context.Background(), "selct;"); err == nil {
			t.Fatal("Expected SQL syntax error")
		}

		rows, _ := conn.Query(context.Background(), "select 1")
		rows.Close()
		if rows.Err() != nil {
			t.Fatalf("Exec failure appears to have broken connection: %v", rows.Err())
		}
	})
}

func TestExecFailureWithArguments(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		_, err := conn.Exec(context.Background(), "selct $1;", 1)
		if err == nil {
			t.Fatal("Expected SQL syntax error")
		}
		assert.False(t, gaussdbconn.SafeToRetry(err))

		_, err = conn.Exec(context.Background(), "select $1::varchar(1);", "1", "2")
		require.Error(t, err)
	})
}

func TestExecContextWithoutCancelation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		ctx, cancelFunc := context.WithCancel(ctx)
		defer cancelFunc()

		commandTag, err := conn.Exec(ctx, "create temporary table foo(id integer primary key);")
		if err != nil {
			t.Fatal(err)
		}
		if commandTag.String() != "CREATE TABLE" {
			t.Fatalf("Unexpected results from Exec: %v", commandTag)
		}
		assert.False(t, gaussdbconn.SafeToRetry(err))
	})
}

func TestExecContextFailureWithoutCancelation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		ctx, cancelFunc := context.WithCancel(ctx)
		defer cancelFunc()

		_, err := conn.Exec(ctx, "selct;")
		if err == nil {
			t.Fatal("Expected SQL syntax error")
		}
		assert.False(t, gaussdbconn.SafeToRetry(err))

		rows, _ := conn.Query(context.Background(), "select 1")
		rows.Close()
		if rows.Err() != nil {
			t.Fatalf("ExecEx failure appears to have broken connection: %v", rows.Err())
		}
		assert.False(t, gaussdbconn.SafeToRetry(err))
	})
}

func TestExecContextFailureWithoutCancelationWithArguments(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		ctx, cancelFunc := context.WithCancel(ctx)
		defer cancelFunc()

		_, err := conn.Exec(ctx, "selct $1;", 1)
		if err == nil {
			t.Fatal("Expected SQL syntax error")
		}
		assert.False(t, gaussdbconn.SafeToRetry(err))
	})
}

func TestExecFailureCloseBefore(t *testing.T) {
	t.Parallel()

	conn := mustConnectString(t, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
	closeConn(t, conn)

	_, err := conn.Exec(context.Background(), "select 1")
	require.Error(t, err)
	assert.True(t, gaussdbconn.SafeToRetry(err))
}

func TestExecPerQuerySimpleProtocol(t *testing.T) {
	t.Parallel()

	conn := mustConnectString(t, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
	defer closeConn(t, conn)

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	commandTag, err := conn.Exec(ctx, "create temporary table foo(name varchar primary key);")
	if err != nil {
		t.Fatal(err)
	}
	if commandTag.String() != "CREATE TABLE" {
		t.Fatalf("Unexpected results from Exec: %v", commandTag)
	}

	commandTag, err = conn.Exec(ctx,
		"insert into foo(name) values($1);",
		gaussdbgo.QueryExecModeSimpleProtocol,
		"bar'; drop table foo;--",
	)
	if err != nil {
		t.Fatal(err)
	}
	if commandTag.String() != "INSERT 0 1" {
		t.Fatalf("Unexpected results from Exec: %v", commandTag)
	}

}

func TestPrepare(t *testing.T) {
	t.Parallel()

	conn := mustConnectString(t, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
	defer closeConn(t, conn)

	_, err := conn.Prepare(context.Background(), "test", "select $1::varchar")
	if err != nil {
		t.Errorf("Unable to prepare statement: %v", err)
		return
	}

	var s string
	err = conn.QueryRow(context.Background(), "test", "hello").Scan(&s)
	if err != nil {
		t.Errorf("Executing prepared statement failed: %v", err)
	}

	if s != "hello" {
		t.Errorf("Prepared statement did not return expected value: %v", s)
	}

	err = conn.Deallocate(context.Background(), "test")
	if err != nil {
		t.Errorf("conn.Deallocate failed: %v", err)
	}

	// Create another prepared statement to ensure Deallocate left the connection
	// in a working state and that we can reuse the prepared statement name.

	_, err = conn.Prepare(context.Background(), "test", "select $1::integer")
	if err != nil {
		t.Errorf("Unable to prepare statement: %v", err)
		return
	}

	var n int32
	err = conn.QueryRow(context.Background(), "test", int32(1)).Scan(&n)
	if err != nil {
		t.Errorf("Executing prepared statement failed: %v", err)
	}

	if n != 1 {
		t.Errorf("Prepared statement did not return expected value: %v", s)
	}

	err = conn.DeallocateAll(context.Background())
	if err != nil {
		t.Errorf("conn.Deallocate failed: %v", err)
	}
}

func TestPrepareBadSQLFailure(t *testing.T) {
	t.Parallel()

	conn := mustConnectString(t, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
	defer closeConn(t, conn)

	if _, err := conn.Prepare(context.Background(), "badSQL", "select foo"); err == nil {
		t.Fatal("Prepare should have failed with syntax error")
	}

	ensureConnValid(t, conn)
}

func TestPrepareIdempotency(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		for i := 0; i < 2; i++ {
			_, err := conn.Prepare(context.Background(), "test", "select 42::integer")
			if err != nil {
				t.Fatalf("%d. Unable to prepare statement: %v", i, err)
			}

			var n int32
			err = conn.QueryRow(context.Background(), "test").Scan(&n)
			if err != nil {
				t.Errorf("%d. Executing prepared statement failed: %v", i, err)
			}

			if n != int32(42) {
				t.Errorf("%d. Prepared statement did not return expected value: %v", i, n)
			}
		}

		_, err := conn.Prepare(context.Background(), "test", "select 'fail'::varchar")
		if err == nil {
			t.Fatalf("Prepare statement with same name but different SQL should have failed but it didn't")
			return
		}
	})
}

func TestPrepareStatementCacheModes(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		_, err := conn.Prepare(context.Background(), "test", "select $1::text")
		require.NoError(t, err)

		var s string
		err = conn.QueryRow(context.Background(), "test", "hello").Scan(&s)
		require.NoError(t, err)
		require.Equal(t, "hello", s)
	})
}

func TestPrepareWithDigestedName(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		sql := "select $1::text"
		sd, err := conn.Prepare(ctx, sql, sql)
		require.NoError(t, err)
		require.Equal(t, "stmt_2510cc7db17de3f42758a2a29c8b9ef8305d007b997ebdd6", sd.Name)

		var s string
		err = conn.QueryRow(ctx, sql, "hello").Scan(&s)
		require.NoError(t, err)
		require.Equal(t, "hello", s)

		err = conn.Deallocate(ctx, sql)
		require.NoError(t, err)
	})
}

func TestDeallocateInAbortedTransaction(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		tx, err := conn.Begin(ctx)
		require.NoError(t, err)

		sql := "select $1::text"
		sd, err := tx.Prepare(ctx, sql, sql)
		require.NoError(t, err)
		require.Equal(t, "stmt_2510cc7db17de3f42758a2a29c8b9ef8305d007b997ebdd6", sd.Name)

		var s string
		err = tx.QueryRow(ctx, sql, "hello").Scan(&s)
		require.NoError(t, err)
		require.Equal(t, "hello", s)

		_, err = tx.Exec(ctx, "select 1/0") // abort transaction with divide by zero error
		require.Error(t, err)

		err = conn.Deallocate(ctx, sql)
		require.NoError(t, err)

		err = tx.Rollback(ctx)
		require.NoError(t, err)

		sd, err = conn.Prepare(ctx, sql, sql)
		require.NoError(t, err)
		require.Equal(t, "stmt_2510cc7db17de3f42758a2a29c8b9ef8305d007b997ebdd6", sd.Name)
	})
}

func TestDeallocateMissingPreparedStatementStillClearsFromPreparedStatementMap(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		_, err := conn.Prepare(ctx, "ps", "select $1::text")
		require.NoError(t, err)

		_, err = conn.Exec(ctx, "deallocate ps")
		require.NoError(t, err)

		err = conn.Deallocate(ctx, "ps")
		require.NoError(t, err)

		_, err = conn.Prepare(ctx, "ps", "select $1::text, $2::text")
		require.NoError(t, err)

		var s1, s2 string
		err = conn.QueryRow(ctx, "ps", "hello", "world").Scan(&s1, &s2)
		require.NoError(t, err)
		require.Equal(t, "hello", s1)
		require.Equal(t, "world", s2)
	})
}

// todo GaussDB 暂时不支持 LISTEN statement、NOFITY statement
/*func TestListenNotify(t *testing.T) {
	t.Parallel()

	listener := mustConnectString(t, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
	defer closeConn(t, listener)

	if listener.GaussdbConn().ParameterStatus("crdb_version") != "" {
		t.Skip("Server does not support LISTEN / NOTIFY (https://github.com/cockroachdb/cockroach/issues/41522)")
	}

	mustExec(t, listener, "listen chat")

	notifier := mustConnectString(t, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
	defer closeConn(t, notifier)

	mustExec(t, notifier, "notify chat")

	// when notification is waiting on the socket to be read
	notification, err := listener.WaitForNotification(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "chat", notification.Channel)

	// when notification has already been read during previous query
	mustExec(t, notifier, "notify chat")
	rows, _ := listener.Query(context.Background(), "select 1")
	rows.Close()
	require.NoError(t, rows.Err())

	ctx, cancelFn := context.WithCancel(context.Background())
	cancelFn()
	notification, err = listener.WaitForNotification(ctx)
	require.NoError(t, err)
	assert.Equal(t, "chat", notification.Channel)

	// when timeout occurs
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	notification, err = listener.WaitForNotification(ctx)
	assert.True(t, gaussdbconn.Timeout(err))
	assert.Nil(t, notification)

	// listener can listen again after a timeout
	mustExec(t, notifier, "notify chat")
	notification, err = listener.WaitForNotification(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "chat", notification.Channel)
}*/

// todo GaussDB 暂时不支持 LISTEN statement、NOFITY statement
/*func TestListenNotifyWhileBusyIsSafe(t *testing.T) {
	t.Parallel()

	func() {
		conn := mustConnectString(t, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
		defer closeConn(t, conn)
	}()

	listenerDone := make(chan bool)
	notifierDone := make(chan bool)
	listening := make(chan bool)
	go func() {
		conn := mustConnectString(t, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
		defer closeConn(t, conn)
		defer func() {
			listenerDone <- true
		}()

		mustExec(t, conn, "listen busysafe")
		listening <- true

		for i := 0; i < 5000; i++ {
			var sum int32
			var rowCount int32

			rows, err := conn.Query(context.Background(), "select generate_series(1,$1)", 100)
			if err != nil {
				t.Errorf("conn.Query failed: %v", err)
				return
			}

			for rows.Next() {
				var n int32
				if err := rows.Scan(&n); err != nil {
					t.Errorf("Row scan failed: %v", err)
					return
				}
				sum += n
				rowCount++
			}

			if rows.Err() != nil {
				t.Errorf("conn.Query failed: %v", rows.Err())
				return
			}

			if sum != 5050 {
				t.Errorf("Wrong rows sum: %v", sum)
				return
			}

			if rowCount != 100 {
				t.Errorf("Wrong number of rows: %v", rowCount)
				return
			}
		}
	}()

	go func() {
		conn := mustConnectString(t, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
		defer closeConn(t, conn)
		defer func() {
			notifierDone <- true
		}()

		<-listening

		for i := 0; i < 100000; i++ {
			mustExec(t, conn, "notify busysafe, 'hello'")
		}
	}()

	<-listenerDone
	<-notifierDone
}*/

// todo: LISTEN statement is not yet supported. (SQLSTATE 0A000)
//func TestListenNotifySelfNotification(t *testing.T) {
//	t.Parallel()
//
//	conn := mustConnectString(t, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
//	defer closeConn(t, conn)
//
//	mustExec(t, conn, "listen self")
//
//	// Notify self and WaitForNotification immediately
//	mustExec(t, conn, "notify self")
//
//	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
//	defer cancel()
//	notification, err := conn.WaitForNotification(ctx)
//	require.NoError(t, err)
//	assert.Equal(t, "self", notification.Channel)
//
//	// Notify self and do something else before WaitForNotification
//	mustExec(t, conn, "notify self")
//
//	rows, _ := conn.Query(context.Background(), "select 1")
//	rows.Close()
//	if rows.Err() != nil {
//		t.Fatalf("Unexpected error on Query: %v", rows.Err())
//	}
//
//	ctx, cncl := context.WithTimeout(context.Background(), time.Second)
//	defer cncl()
//	notification, err = conn.WaitForNotification(ctx)
//	require.NoError(t, err)
//	assert.Equal(t, "self", notification.Channel)
//}

// todo: conn.GaussdbConn().PID() not return the right pid.
//func TestFatalRxError(t *testing.T) {
//	t.Parallel()
//	envVar := os.Getenv(gaussdbgo.EnvGaussdbTestDatabase)
//
//	conn := mustConnectString(t, envVar)
//	defer closeConn(t, conn)
//
//	var wg sync.WaitGroup
//	wg.Add(1)
//	go func() {
//		defer wg.Done()
//		var n int32
//		var s string
//		err := conn.QueryRow(context.Background(), "select 1::int4, pg_sleep(10)::varchar").Scan(&n, &s)
//		gaussdbErr, ok := err.(*gaussdbconn.GaussdbError)
//		if !(ok && gaussdbErr.Severity == "FATAL") {
//			t.Errorf("Expected QueryRow Scan to return fatal GaussdbError, but instead received %v", err)
//			return
//		}
//	}()
//
//	otherConn := mustConnectString(t, envVar)
//	defer otherConn.Close(context.Background())
//
//	if _, err := otherConn.Exec(context.Background(), "select pg_terminate_backend($1)", conn.GaussdbConn().PID()); err != nil {
//		t.Fatalf("Unable to kill backend GaussDB process: %v", err)
//	}
//
//	wg.Wait()
//
//	if !conn.IsClosed() {
//		t.Fatal("Connection should be closed")
//	}
//}

// todo: conn.GaussdbConn().PID() not return the right pid.
//func TestFatalTxError(t *testing.T) {
//	t.Parallel()
//
//	// Run timing sensitive test many times
//	for i := 0; i < 50; i++ {
//		func() {
//			conn := mustConnectString(t, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
//			defer closeConn(t, conn)
//
//			otherConn := mustConnectString(t, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
//			defer otherConn.Close(context.Background())
//
//			_, err := otherConn.Exec(context.Background(), "select pg_terminate_backend($1)", conn.GaussdbConn().PID())
//			if err != nil {
//				t.Fatalf("Unable to kill backend GaussDB process: %v", err)
//			}
//
//			err = conn.QueryRow(context.Background(), "select 1").Scan(nil)
//			if err == nil {
//				t.Fatal("Expected error but none occurred")
//			}
//
//			if !conn.IsClosed() {
//				t.Fatalf("Connection should be closed but isn't. Previous Query err: %v", err)
//			}
//		}()
//	}
//}

func TestInsertBoolArray(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		if results := mustExec(t, conn, "create temporary table foo(spice bool[]);"); results.String() != "CREATE TABLE" {
			t.Error("Unexpected results from Exec")
		}

		// Accept parameters
		if results := mustExec(t, conn, "insert into foo(spice) values($1)", []bool{true, false, true}); results.String() != "INSERT 0 1" {
			t.Errorf("Unexpected results from Exec: %v", results)
		}
	})
}

func TestInsertTimestampArray(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		if results := mustExec(t, conn, "create temporary table foo(spice timestamp[]);"); results.String() != "CREATE TABLE" {
			t.Error("Unexpected results from Exec")
		}

		// Accept parameters
		if results := mustExec(t, conn, "insert into foo(spice) values($1)", []time.Time{time.Unix(1419143667, 0), time.Unix(1419143672, 0)}); results.String() != "INSERT 0 1" {
			t.Errorf("Unexpected results from Exec: %v", results)
		}
	})
}

func TestIdentifierSanitize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ident    gaussdbgo.Identifier
		expected string
	}{
		{
			ident:    gaussdbgo.Identifier{`foo`},
			expected: `"foo"`,
		},
		{
			ident:    gaussdbgo.Identifier{`select`},
			expected: `"select"`,
		},
		{
			ident:    gaussdbgo.Identifier{`foo`, `bar`},
			expected: `"foo"."bar"`,
		},
		{
			ident:    gaussdbgo.Identifier{`you should " not do this`},
			expected: `"you should "" not do this"`,
		},
		{
			ident:    gaussdbgo.Identifier{`you should " not do this`, `please don't`},
			expected: `"you should "" not do this"."please don't"`,
		},
		{
			ident:    gaussdbgo.Identifier{`you should ` + string([]byte{0}) + `not do this`},
			expected: `"you should not do this"`,
		},
	}

	for i, tt := range tests {
		qval := tt.ident.Sanitize()
		if qval != tt.expected {
			t.Errorf("%d. Expected Sanitize %v to return %v but it was %v", i, tt.ident, tt.expected, qval)
		}
	}
}

func TestConnInitTypeMap(t *testing.T) {
	conn := mustConnectString(t, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
	defer closeConn(t, conn)

	// spot check that the standard gaussdb type names aren't qualified
	nameOIDs := map[string]uint32{
		"_int8": gaussdbtype.Int8ArrayOID,
		"int8":  gaussdbtype.Int8OID,
		"json":  gaussdbtype.JSONOID,
		"text":  gaussdbtype.TextOID,
	}
	for name, oid := range nameOIDs {
		dtByName, ok := conn.TypeMap().TypeForName(name)
		if !ok {
			t.Fatalf("Expected type named %v to be present", name)
		}
		dtByOID, ok := conn.TypeMap().TypeForOID(oid)
		if !ok {
			t.Fatalf("Expected type OID %v to be present", oid)
		}
		if dtByName != dtByOID {
			t.Fatalf("Expected type named %v to be the same as type OID %v", name, oid)
		}
	}

	ensureConnValid(t, conn)
}

// todo GaussDB 暂时不支持 Domain域类型
//func TestUnregisteredTypeUsableAsStringArgumentAndBaseResult(t *testing.T) {
//	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
//	defer cancel()
//
//	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
//
//		var n uint64
//		err := conn.QueryRow(context.Background(), "select $1::uint64", "42").Scan(&n)
//		if err != nil {
//			t.Fatal(err)
//		}
//
//		if n != 42 {
//			t.Fatalf("Expected n to be 42, but was %v", n)
//		}
//	})
//}

// todo GaussDB 暂时不支持 Domain域类型
/*func TestDomainType(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {

		// Domain type uint64 is a GaussDB domain of underlying type numeric.

		// In the extended protocol preparing "select $1::uint64" appears to create a statement that expects a param OID of
		// uint64 but a result OID of the underlying numeric.

		var s string
		err := conn.QueryRow(ctx, "select $1::uint64", "24").Scan(&s)
		require.NoError(t, err)
		require.Equal(t, "24", s)

		// Register type
		uint64Type, err := conn.LoadType(ctx, "uint64")
		require.NoError(t, err)
		conn.TypeMap().RegisterType(uint64Type)

		var n uint64
		err = conn.QueryRow(ctx, "select $1::uint64", uint64(24)).Scan(&n)
		require.NoError(t, err)

		// String is still an acceptable argument after registration
		err = conn.QueryRow(ctx, "select $1::uint64", "7").Scan(&n)
		if err != nil {
			t.Fatal(err)
		}
		if n != 7 {
			t.Fatalf("Expected n to be 7, but was %v", n)
		}
	})
}*/

func TestLoadTypeSameNameInDifferentSchemas(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {

		tx, err := conn.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx, `create schema gaussdbgo_a;
create type gaussdbgo_a.point as (a text, b text);
create schema gaussdbgo_b;
create type gaussdbgo_b.point as (c text);
`)
		require.NoError(t, err)

		// Register types
		for _, typename := range []string{"gaussdbgo_a.point", "gaussdbgo_b.point"} {
			// Obviously using conn while a tx is in use and registering a type after the connection has been established are
			// really bad practices, but for the sake of convenience we do it in the test here.
			dt, err := conn.LoadType(ctx, typename)
			require.NoError(t, err)
			conn.TypeMap().RegisterType(dt)
		}

		type aPoint struct {
			A string
			B string
		}

		type bPoint struct {
			C string
		}

		var a aPoint
		var b bPoint
		err = tx.QueryRow(ctx, `select '(foo,bar)'::gaussdbgo_a.point, '(baz)'::gaussdbgo_b.point`).Scan(&a, &b)
		require.NoError(t, err)
		require.Equal(t, aPoint{"foo", "bar"}, a)
		require.Equal(t, bPoint{"baz"}, b)
	})
}

func TestLoadCompositeType(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {

		tx, err := conn.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx, "create type compositetype as (attr1 int, attr2 int)")
		require.NoError(t, err)

		_, err = tx.Exec(ctx, "alter type compositetype drop attribute attr1")
		require.NoError(t, err)

		_, err = conn.LoadType(ctx, "compositetype")
		require.NoError(t, err)
	})
}

func TestLoadRangeType(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {

		tx, err := conn.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx, "create type examplefloatrange as range (subtype=float8, subtype_diff=float8mi)")
		require.NoError(t, err)

		// Register types
		newRangeType, err := conn.LoadType(ctx, "examplefloatrange")
		require.NoError(t, err)
		conn.TypeMap().RegisterType(newRangeType)
		conn.TypeMap().RegisterDefaultGaussdbType(gaussdbtype.Range[float64]{}, "examplefloatrange")

		var inputRangeType = gaussdbtype.Range[float64]{
			Lower:     1.0,
			Upper:     2.0,
			LowerType: gaussdbtype.Inclusive,
			UpperType: gaussdbtype.Inclusive,
			Valid:     true,
		}
		var outputRangeType gaussdbtype.Range[float64]
		err = tx.QueryRow(ctx, "SELECT $1::examplefloatrange", inputRangeType).Scan(&outputRangeType)
		require.NoError(t, err)
		require.Equal(t, inputRangeType, outputRangeType)
	})
}

// todo GaussDB 暂时不支持 MultiRangeType多范围类型
/*func TestLoadMultiRangeType(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {

		tx, err := conn.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		_, err = tx.Exec(ctx, "create type examplefloatrange as range (subtype=float8, subtype_diff=float8mi, multirange_type_name=examplefloatmultirange)")
		require.NoError(t, err)

		// Register types
		newRangeType, err := conn.LoadType(ctx, "examplefloatrange")
		require.NoError(t, err)
		conn.TypeMap().RegisterType(newRangeType)
		conn.TypeMap().RegisterDefaultGaussdbType(gaussdbtype.Range[float64]{}, "examplefloatrange")

		newMultiRangeType, err := conn.LoadType(ctx, "examplefloatmultirange")
		require.NoError(t, err)
		conn.TypeMap().RegisterType(newMultiRangeType)
		conn.TypeMap().RegisterDefaultGaussdbType(gaussdbtype.Multirange[gaussdbtype.Range[float64]]{}, "examplefloatmultirange")

		var inputMultiRangeType = gaussdbtype.Multirange[gaussdbtype.Range[float64]]{
			{
				Lower:     1.0,
				Upper:     2.0,
				LowerType: gaussdbtype.Inclusive,
				UpperType: gaussdbtype.Inclusive,
				Valid:     true,
			},
			{
				Lower:     3.0,
				Upper:     4.0,
				LowerType: gaussdbtype.Exclusive,
				UpperType: gaussdbtype.Exclusive,
				Valid:     true,
			},
		}
		var outputMultiRangeType gaussdbtype.Multirange[gaussdbtype.Range[float64]]
		err = tx.QueryRow(ctx, "SELECT $1::examplefloatmultirange", inputMultiRangeType).Scan(&outputMultiRangeType)
		require.NoError(t, err)
		require.Equal(t, inputMultiRangeType, outputMultiRangeType)
	})
}*/

func TestStmtCacheInvalidationConn(t *testing.T) {
	ctx := context.Background()

	conn := mustConnectString(t, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
	defer closeConn(t, conn)

	// create a table and fill it with some data
	_, err := conn.Exec(ctx, `
        DROP TABLE IF EXISTS drop_cols;
        CREATE TABLE drop_cols (
            id SERIAL PRIMARY KEY NOT NULL,
            f1 int NOT NULL,
            f2 int NOT NULL
        );
    `)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "INSERT INTO drop_cols (f1, f2) VALUES (1, 2)")
	require.NoError(t, err)

	getSQL := "SELECT * FROM drop_cols WHERE id = $1"

	// This query will populate the statement cache. We don't care about the result.
	rows, err := conn.Query(ctx, getSQL, 1)
	require.NoError(t, err)
	rows.Close()
	require.NoError(t, rows.Err())

	// Now, change the schema of the table out from under the statement, making it invalid.
	_, err = conn.Exec(ctx, "ALTER TABLE drop_cols DROP COLUMN f1")
	require.NoError(t, err)

	// We must get an error the first time we try to re-execute a bad statement.
	// It is up to the application to determine if it wants to try again. We punt to
	// the application because there is no clear recovery path in the case of failed transactions
	// or batch operations and because automatic retry is tricky and we don't want to get
	// it wrong at such an importaint layer of the stack.
	rows, err = conn.Query(ctx, getSQL, 1)
	require.NoError(t, err)
	rows.Next()
	nextErr := rows.Err()
	rows.Close()
	for _, err := range []error{nextErr, rows.Err()} {
		// same as TestStmtCacheInvalidationTx
		if err == nil {
			t.Fatal(`expected "Cached plan must not change result type": no error`)
		}
		if !strings.Contains(err.Error(), lowerFirstLetterInError("Cached plan must not change result type")) {
			t.Fatalf(`expected "Cached plan must not change result type", got: "%s"`, err.Error())
		}
	}

	// On retry, the statement should have been flushed from the cache.
	rows, err = conn.Query(ctx, getSQL, 1)
	require.NoError(t, err)
	rows.Next()
	err = rows.Err()
	require.NoError(t, err)
	rows.Close()
	require.NoError(t, rows.Err())

	ensureConnValid(t, conn)
}

func TestStmtCacheInvalidationTx(t *testing.T) {
	ctx := context.Background()

	conn := mustConnectString(t, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
	defer closeConn(t, conn)

	// create a table and fill it with some data
	_, err := conn.Exec(ctx, `
        DROP TABLE IF EXISTS drop_cols;
        CREATE TABLE drop_cols (
            id SERIAL PRIMARY KEY NOT NULL,
            f1 int NOT NULL,
            f2 int NOT NULL
        );
    `)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "INSERT INTO drop_cols (f1, f2) VALUES (1, 2)")
	require.NoError(t, err)

	tx, err := conn.Begin(ctx)
	require.NoError(t, err)

	getSQL := "SELECT * FROM drop_cols WHERE id = $1"

	// This query will populate the statement cache. We don't care about the result.
	rows, err := tx.Query(ctx, getSQL, 1)
	require.NoError(t, err)
	rows.Close()
	require.NoError(t, rows.Err())

	// Now, change the schema of the table out from under the statement, making it invalid.
	_, err = tx.Exec(ctx, "ALTER TABLE drop_cols DROP COLUMN f1")
	require.NoError(t, err)

	// We must get an error the first time we try to re-execute a bad statement.
	// It is up to the application to determine if it wants to try again. We punt to
	// the application because there is no clear recovery path in the case of failed transactions
	// or batch operations and because automatic retry is tricky and we don't want to get
	// it wrong at such an importaint layer of the stack.
	rows, err = tx.Query(ctx, getSQL, 1)
	require.NoError(t, err)
	rows.Next()
	nextErr := rows.Err()
	rows.Close()
	// opengauss always return "cached ...", gaussdb return "Cached ..."
	for _, err := range []error{nextErr, rows.Err()} {
		if err == nil {
			t.Fatal(`expected "Cached plan must not change result type": no error`)
		}
		if !strings.Contains(err.Error(), lowerFirstLetterInError("Cached plan must not change result type")) {
			t.Fatalf(`expected "Cached plan must not change result type", got: "%s"`, err.Error())
		}
	}

	rows, _ = tx.Query(ctx, getSQL, 1)
	rows.Close()
	err = rows.Err()
	// Retries within the same transaction are errors (really anything except a rollback
	// will be an error in this transaction).
	require.Error(t, err)
	rows.Close()

	err = tx.Rollback(ctx)
	require.NoError(t, err)

	// once we've rolled back, retries will work
	rows, err = conn.Query(ctx, getSQL, 1)
	require.NoError(t, err)
	rows.Next()
	err = rows.Err()
	require.NoError(t, err)
	rows.Close()

	ensureConnValid(t, conn)
}

func TestInsertDurationInterval(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		_, err := conn.Exec(context.Background(), "create temporary table t(duration INTERVAL(0) NOT NULL)")
		require.NoError(t, err)

		result, err := conn.Exec(context.Background(), "insert into t(duration) values($1)", time.Minute)
		require.NoError(t, err)

		n := result.RowsAffected()
		require.EqualValues(t, 1, n)
	})
}

func TestRawValuesUnderlyingMemoryReused(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		var buf []byte

		rows, err := conn.Query(ctx, `select 1::int`)
		require.NoError(t, err)

		for rows.Next() {
			buf = rows.RawValues()[0]
		}

		require.NoError(t, rows.Err())

		original := make([]byte, len(buf))
		copy(original, buf)

		for i := 0; i < 1_000_000; i++ {
			rows, err := conn.Query(ctx, `select $1::int`, i)
			require.NoError(t, err)
			rows.Close()
			require.NoError(t, rows.Err())

			if !bytes.Equal(original, buf) {
				return
			}
		}

		t.Fatal("expected buffer from RawValues to be overwritten by subsequent queries but it was not")
	})
}

func TestConnDeallocateInvalidatedCachedStatementsWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	gaussdbxtest.RunWithQueryExecModes(ctx, t, defaultConnTestRunner, nil, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {

		var n int32
		err := conn.QueryRow(ctx, "select 1 / $1::int", 1).Scan(&n)
		require.NoError(t, err)
		require.EqualValues(t, 1, n)

		// Divide by zero causes an error. baseRows.Close() calls Invalidate on the statement cache whenever an error was
		// encountered by the query. Use this to purposely invalidate the query. If we had access to private fields of conn
		// we could call conn.statementCache.InvalidateAll() instead.
		err = conn.QueryRow(ctx, "select 1 / $1::int", 0).Scan(&n)
		require.Error(t, err)

		ctx2, cancel2 := context.WithCancel(ctx)
		cancel2()
		err = conn.QueryRow(ctx2, "select 1 / $1::int", 1).Scan(&n)
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)

		err = conn.QueryRow(ctx, "select 1 / $1::int", 1).Scan(&n)
		require.NoError(t, err)
		require.EqualValues(t, 1, n)
	})
}

func TestConnDeallocateInvalidatedCachedStatementsInTransactionWithBatch(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	connString := os.Getenv(gaussdbgo.EnvGaussdbTestDatabase)
	config := mustParseConfig(t, connString)
	config.DefaultQueryExecMode = gaussdbgo.QueryExecModeCacheStatement
	config.StatementCacheCapacity = 2

	conn, err := gaussdbgo.ConnectConfig(ctx, config)
	require.NoError(t, err)

	tx, err := conn.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, "select $1::int + 1", 1)
	require.NoError(t, err)

	_, err = tx.Exec(ctx, "select $1::int + 2", 1)
	require.NoError(t, err)

	// This should invalidate the first cached statement.
	_, err = tx.Exec(ctx, "select $1::int + 3", 1)
	require.NoError(t, err)

	batch := &gaussdbgo.Batch{}
	batch.Queue("select $1::int + 1", 1)
	err = tx.SendBatch(ctx, batch).Close()
	require.NoError(t, err)

	err = tx.Rollback(ctx)
	require.NoError(t, err)

	ensureConnValid(t, conn)
}

func TestErrNoRows(t *testing.T) {
	t.Parallel()

	// ensure we preserve old error message
	require.Equal(t, "no rows in result set", gaussdbgo.ErrNoRows.Error())

	require.ErrorIs(t, gaussdbgo.ErrNoRows, sql.ErrNoRows, "gaussdbgo.ErrNowRows must match sql.ErrNoRows")
}
