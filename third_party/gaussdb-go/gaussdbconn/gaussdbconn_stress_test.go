package gaussdbconn_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbconn"

	"github.com/stretchr/testify/require"
)

func TestConnStress(t *testing.T) {
	gaussdbConn, err := gaussdbconn.Connect(context.Background(), os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
	require.NoError(t, err)
	defer closeConn(t, gaussdbConn)

	actionCount := 10000
	if s := os.Getenv(gaussdbgo.EnvGaussdbTestStressFactor); s != "" {
		stressFactor, err := strconv.ParseInt(s, 10, 64)
		require.Nil(t, err, fmt.Sprintf("Failed to parse %s", gaussdbgo.EnvGaussdbTestStressFactor))
		actionCount *= int(stressFactor)
	}

	setupStressDB(t, gaussdbConn)

	actions := []struct {
		name string
		fn   func(*gaussdbconn.GaussdbConn) error
	}{
		{"Exec Select", stressExecSelect},
		{"ExecParams Select", stressExecParamsSelect},
		{"Batch", stressBatch},
	}

	for i := 0; i < actionCount; i++ {
		action := actions[rand.Intn(len(actions))]
		err := action.fn(gaussdbConn)
		require.Nilf(t, err, "%d: %s", i, action.name)
	}

	// Each call with a context starts a goroutine. Ensure they are cleaned up when context is not canceled.
	numGoroutine := runtime.NumGoroutine()
	require.Truef(t, numGoroutine < 1000, "goroutines appear to be orphaned: %d in process", numGoroutine)
}

func setupStressDB(t *testing.T, gaussdbConn *gaussdbconn.GaussdbConn) {
	_, err := gaussdbConn.Exec(context.Background(), `
		drop table if exists widgets;
		create table widgets(
			id serial primary key,
			name varchar not null,
			description text,
			creation_time timestamptz default now()
		);

		insert into widgets(name, description) values
			('Foo', 'bar'),
			('baz', 'Something really long Something really long Something really long Something really long Something really long'),
			('a', 'b')`).ReadAll()
	require.NoError(t, err)
}

func stressExecSelect(gaussdbConn *gaussdbconn.GaussdbConn) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := gaussdbConn.Exec(ctx, "select * from widgets").ReadAll()
	return err
}

func stressExecParamsSelect(gaussdbConn *gaussdbconn.GaussdbConn) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := gaussdbConn.ExecParams(ctx, "select * from widgets where id < $1", [][]byte{[]byte("10")}, nil, nil, nil).Read()
	return result.Err
}

func stressBatch(gaussdbConn *gaussdbconn.GaussdbConn) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	batch := &gaussdbconn.Batch{}

	batch.ExecParams("select * from widgets", nil, nil, nil, nil)
	batch.ExecParams("select * from widgets where id < $1", [][]byte{[]byte("10")}, nil, nil, nil)
	_, err := gaussdbConn.ExecBatch(ctx, batch).ReadAll()
	return err
}
