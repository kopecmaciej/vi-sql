package gaussdbgo_test

import (
	"context"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbconn"
	"github.com/stretchr/testify/require"
)

func TestPipelineWithoutPreparedOrDescribedStatements(t *testing.T) {
	t.Parallel()

	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		pipeline := conn.GaussdbConn().StartPipeline(ctx)

		eqb := gaussdbgo.ExtendedQueryBuilder{}

		err := eqb.Build(conn.TypeMap(), nil, []any{1, 2})
		require.NoError(t, err)
		pipeline.SendQueryParams(`select $1::bigint + $2::bigint`, eqb.ParamValues, nil, eqb.ParamFormats, eqb.ResultFormats)

		err = eqb.Build(conn.TypeMap(), nil, []any{3, 4, 5})
		require.NoError(t, err)
		pipeline.SendQueryParams(`select $1::bigint + $2::bigint + $3::bigint`, eqb.ParamValues, nil, eqb.ParamFormats, eqb.ResultFormats)

		err = pipeline.Sync()
		require.NoError(t, err)

		results, err := pipeline.GetResults()
		require.NoError(t, err)
		rr, ok := results.(*gaussdbconn.ResultReader)
		require.True(t, ok)
		rows := gaussdbgo.RowsFromResultReader(conn.TypeMap(), rr)

		rowCount := 0
		var n int64
		for rows.Next() {
			err = rows.Scan(&n)
			require.NoError(t, err)
			rowCount++
		}
		require.NoError(t, rows.Err())
		require.Equal(t, 1, rowCount)
		require.Equal(t, "SELECT 1", rows.CommandTag().String())
		require.EqualValues(t, 3, n)

		results, err = pipeline.GetResults()
		require.NoError(t, err)
		rr, ok = results.(*gaussdbconn.ResultReader)
		require.True(t, ok)
		rows = gaussdbgo.RowsFromResultReader(conn.TypeMap(), rr)

		rowCount = 0
		n = 0
		for rows.Next() {
			err = rows.Scan(&n)
			require.NoError(t, err)
			rowCount++
		}
		require.NoError(t, rows.Err())
		require.Equal(t, 1, rowCount)
		require.Equal(t, "SELECT 1", rows.CommandTag().String())
		require.EqualValues(t, 12, n)

		results, err = pipeline.GetResults()
		require.NoError(t, err)
		_, ok = results.(*gaussdbconn.PipelineSync)
		require.True(t, ok)

		results, err = pipeline.GetResults()
		require.NoError(t, err)
		require.Nil(t, results)

		err = pipeline.Close()
		require.NoError(t, err)
	})
}
