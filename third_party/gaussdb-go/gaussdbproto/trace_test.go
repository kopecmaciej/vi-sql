package gaussdbproto_test

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	gaussdbgo "github.com/HuaweiCloudDeveloper/gaussdb-go"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbconn"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbproto"
	"github.com/stretchr/testify/require"
)

func TestTrace(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	conn, err := gaussdbconn.Connect(ctx, os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
	require.NoError(t, err)
	defer conn.Close(ctx)

	traceOutput := &bytes.Buffer{}
	conn.Frontend().Trace(traceOutput, gaussdbproto.TracerOptions{
		SuppressTimestamps: true,
		RegressMode:        true,
	})

	result := conn.ExecParams(ctx, "select n from generate_series(1,5) n", nil, nil, nil, nil).Read()
	require.NoError(t, result.Err)

	expected := `F	Parse	45	 "" "select n from generate_series(1,5) n" 0
F	Bind	13	 "" "" 0 0 0
F	Describe	7	 P ""
F	Execute	10	 "" 0
F	Sync	5
B	ParseComplete	5
B	BindComplete	5
B	RowDescription	27	 1 "n" 0 0 23 4 -1 0
B	DataRow	12	 1 1 '1'
B	DataRow	12	 1 1 '2'
B	DataRow	12	 1 1 '3'
B	DataRow	12	 1 1 '4'
B	DataRow	12	 1 1 '5'
B	CommandComplete	14	 "SELECT 5"
B	ReadyForQuery	6	 I
`

	require.Equal(t, expected, traceOutput.String())
}
