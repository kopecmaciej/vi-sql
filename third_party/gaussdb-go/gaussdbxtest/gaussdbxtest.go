// Package gaussdbxtest provides utilities for testing gaussdbgo and packages that integrate with gaussdbgo.
package gaussdbxtest

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go"
)

var AllQueryExecModes = []gaussdbgo.QueryExecMode{
	gaussdbgo.QueryExecModeCacheStatement,
	gaussdbgo.QueryExecModeCacheDescribe,
	gaussdbgo.QueryExecModeDescribeExec,
	gaussdbgo.QueryExecModeExec,
	gaussdbgo.QueryExecModeSimpleProtocol,
}

// KnownOIDQueryExecModes is a slice of all query exec modes where the param and result OIDs are known before sending the query.
var KnownOIDQueryExecModes = []gaussdbgo.QueryExecMode{
	gaussdbgo.QueryExecModeCacheStatement,
	gaussdbgo.QueryExecModeCacheDescribe,
	gaussdbgo.QueryExecModeDescribeExec,
}

// ConnTestRunner controls how a *gaussdbgo.Conn is created and closed by tests. All fields are required. Use DefaultConnTestRunner to get a
// ConnTestRunner with reasonable default values.
type ConnTestRunner struct {
	// CreateConfig returns a *gaussdbgo.ConnConfig suitable for use with gaussdbgo.ConnectConfig.
	CreateConfig func(ctx context.Context, t testing.TB) *gaussdbgo.ConnConfig

	// AfterConnect is called after conn is established. It allows for arbitrary connection setup before a test begins.
	AfterConnect func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn)

	// AfterTest is called after the test is run. It allows for validating the state of the connection before it is closed.
	AfterTest func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn)

	// CloseConn closes conn.
	CloseConn func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn)
}

// DefaultConnTestRunner returns a new ConnTestRunner with all fields set to reasonable default values.
func DefaultConnTestRunner() ConnTestRunner {
	return ConnTestRunner{
		CreateConfig: func(ctx context.Context, t testing.TB) *gaussdbgo.ConnConfig {
			config, err := gaussdbgo.ParseConfig("")
			if err != nil {
				t.Fatalf("ParseConfig failed: %v", err)
			}
			return config
		},
		AfterConnect: func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {},
		AfterTest:    func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {},
		CloseConn: func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
			err := conn.Close(ctx)
			if err != nil {
				t.Errorf("Close failed: %v", err)
			}
		},
	}
}

func (ctr *ConnTestRunner) RunTest(ctx context.Context, t testing.TB, f func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn)) {
	t.Helper()

	config := ctr.CreateConfig(ctx, t)
	conn, err := gaussdbgo.ConnectConfig(ctx, config)
	if err != nil {
		t.Fatalf("ConnectConfig failed: %v", err)
	}
	defer ctr.CloseConn(ctx, t, conn)

	ctr.AfterConnect(ctx, t, conn)
	f(ctx, t, conn)
	ctr.AfterTest(ctx, t, conn)
}

// RunWithQueryExecModes runs a f in a new test for each element of modes with a new connection created using connector.
// If modes is nil all gaussdbgo.QueryExecModes are tested.
func RunWithQueryExecModes(ctx context.Context, t *testing.T, ctr ConnTestRunner, modes []gaussdbgo.QueryExecMode, f func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn)) {
	if modes == nil {
		modes = AllQueryExecModes
	}

	for _, mode := range modes {
		ctrWithMode := ctr
		ctrWithMode.CreateConfig = func(ctx context.Context, t testing.TB) *gaussdbgo.ConnConfig {
			config := ctr.CreateConfig(ctx, t)
			config.DefaultQueryExecMode = mode
			return config
		}

		t.Run(mode.String(),
			func(t *testing.T) {
				ctrWithMode.RunTest(ctx, t, f)
			},
		)
	}
}

type ValueRoundTripTest struct {
	Param  any
	Result any
	Test   func(any) bool
}

func RunValueRoundTripTests(
	ctx context.Context,
	t testing.TB,
	ctr ConnTestRunner,
	modes []gaussdbgo.QueryExecMode,
	gaussdbTypeName string,
	tests []ValueRoundTripTest,
) {
	t.Helper()

	if modes == nil {
		modes = AllQueryExecModes
	}

	ctr.RunTest(ctx, t, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		t.Helper()

		sql := fmt.Sprintf("select $1::%s", gaussdbTypeName)

		for i, tt := range tests {
			for _, mode := range modes {
				err := conn.QueryRow(ctx, sql, mode, tt.Param).Scan(tt.Result)
				if err != nil {
					t.Errorf("%d. %v: %v", i, mode, err)
				}

				result := reflect.ValueOf(tt.Result)
				if result.Kind() == reflect.Ptr {
					result = result.Elem()
				}

				if !tt.Test(result.Interface()) {
					t.Errorf("%d. %v: unexpected result for %v: %v", i, mode, tt.Param, result.Interface())
				}
			}
		}
	})
}
