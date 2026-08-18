package gaussdbtype_test

import (
	"context"
	"testing"

	gaussdbx "github.com/HuaweiCloudDeveloper/gaussdb-go"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
	"github.com/stretchr/testify/require"
)

func TestRangeCodecTranscode(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "int4range", []gaussdbxtest.ValueRoundTripTest{
		{
			gaussdbtype.Range[gaussdbtype.Int4]{LowerType: gaussdbtype.Empty, UpperType: gaussdbtype.Empty, Valid: true},
			new(gaussdbtype.Range[gaussdbtype.Int4]),
			isExpectedEq(gaussdbtype.Range[gaussdbtype.Int4]{LowerType: gaussdbtype.Empty, UpperType: gaussdbtype.Empty, Valid: true}),
		},
		{
			gaussdbtype.Range[gaussdbtype.Int4]{
				LowerType: gaussdbtype.Inclusive,
				Lower:     gaussdbtype.Int4{Int32: 1, Valid: true},
				Upper:     gaussdbtype.Int4{Int32: 5, Valid: true},
				UpperType: gaussdbtype.Exclusive, Valid: true,
			},
			new(gaussdbtype.Range[gaussdbtype.Int4]),
			isExpectedEq(gaussdbtype.Range[gaussdbtype.Int4]{
				LowerType: gaussdbtype.Inclusive,
				Lower:     gaussdbtype.Int4{Int32: 1, Valid: true},
				Upper:     gaussdbtype.Int4{Int32: 5, Valid: true},
				UpperType: gaussdbtype.Exclusive, Valid: true,
			}),
		},
		{gaussdbtype.Range[gaussdbtype.Int4]{}, new(gaussdbtype.Range[gaussdbtype.Int4]), isExpectedEq(gaussdbtype.Range[gaussdbtype.Int4]{})},
		{nil, new(gaussdbtype.Range[gaussdbtype.Int4]), isExpectedEq(gaussdbtype.Range[gaussdbtype.Int4]{})},
	})
}

func TestRangeCodecTranscodeCompatibleRangeElementTypes(t *testing.T) {
	ctr := defaultConnTestRunner
	ctr.AfterConnect = func(ctx context.Context, t testing.TB, conn *gaussdbx.Conn) {}

	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, ctr, nil, "numrange", []gaussdbxtest.ValueRoundTripTest{
		{
			gaussdbtype.Range[gaussdbtype.Float8]{LowerType: gaussdbtype.Empty, UpperType: gaussdbtype.Empty, Valid: true},
			new(gaussdbtype.Range[gaussdbtype.Float8]),
			isExpectedEq(gaussdbtype.Range[gaussdbtype.Float8]{LowerType: gaussdbtype.Empty, UpperType: gaussdbtype.Empty, Valid: true}),
		},
		{
			gaussdbtype.Range[gaussdbtype.Float8]{
				LowerType: gaussdbtype.Inclusive,
				Lower:     gaussdbtype.Float8{Float64: 1, Valid: true},
				Upper:     gaussdbtype.Float8{Float64: 5, Valid: true},
				UpperType: gaussdbtype.Exclusive, Valid: true,
			},
			new(gaussdbtype.Range[gaussdbtype.Float8]),
			isExpectedEq(gaussdbtype.Range[gaussdbtype.Float8]{
				LowerType: gaussdbtype.Inclusive,
				Lower:     gaussdbtype.Float8{Float64: 1, Valid: true},
				Upper:     gaussdbtype.Float8{Float64: 5, Valid: true},
				UpperType: gaussdbtype.Exclusive, Valid: true,
			}),
		},
		{gaussdbtype.Range[gaussdbtype.Float8]{}, new(gaussdbtype.Range[gaussdbtype.Float8]), isExpectedEq(gaussdbtype.Range[gaussdbtype.Float8]{})},
		{nil, new(gaussdbtype.Range[gaussdbtype.Float8]), isExpectedEq(gaussdbtype.Range[gaussdbtype.Float8]{})},
	})
}

func TestRangeCodecScanRangeTwiceWithUnbounded(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *gaussdbx.Conn) {

		var r gaussdbtype.Range[gaussdbtype.Int4]

		err := conn.QueryRow(context.Background(), `select '[1,5)'::int4range`).Scan(&r)
		require.NoError(t, err)

		require.Equal(
			t,
			gaussdbtype.Range[gaussdbtype.Int4]{
				Lower:     gaussdbtype.Int4{Int32: 1, Valid: true},
				Upper:     gaussdbtype.Int4{Int32: 5, Valid: true},
				LowerType: gaussdbtype.Inclusive,
				UpperType: gaussdbtype.Exclusive,
				Valid:     true,
			},
			r,
		)

		err = conn.QueryRow(ctx, `select '[1,)'::int4range`).Scan(&r)
		require.NoError(t, err)

		require.Equal(
			t,
			gaussdbtype.Range[gaussdbtype.Int4]{
				Lower:     gaussdbtype.Int4{Int32: 1, Valid: true},
				Upper:     gaussdbtype.Int4{},
				LowerType: gaussdbtype.Inclusive,
				UpperType: gaussdbtype.Unbounded,
				Valid:     true,
			},
			r,
		)

		err = conn.QueryRow(ctx, `select 'empty'::int4range`).Scan(&r)
		require.NoError(t, err)

		require.Equal(
			t,
			gaussdbtype.Range[gaussdbtype.Int4]{
				Lower:     gaussdbtype.Int4{},
				Upper:     gaussdbtype.Int4{},
				LowerType: gaussdbtype.Empty,
				UpperType: gaussdbtype.Empty,
				Valid:     true,
			},
			r,
		)
	})
}

func TestRangeCodecDecodeValue(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, _ testing.TB, conn *gaussdbx.Conn) {

		for _, tt := range []struct {
			sql      string
			expected any
		}{
			{
				sql: `select '[1,5)'::int4range`,
				expected: gaussdbtype.Range[any]{
					Lower:     int32(1),
					Upper:     int32(5),
					LowerType: gaussdbtype.Inclusive,
					UpperType: gaussdbtype.Exclusive,
					Valid:     true,
				},
			},
		} {
			t.Run(tt.sql, func(t *testing.T) {
				rows, err := conn.Query(ctx, tt.sql)
				require.NoError(t, err)

				for rows.Next() {
					values, err := rows.Values()
					require.NoError(t, err)
					require.Len(t, values, 1)
					require.Equal(t, tt.expected, values[0])
				}

				require.NoError(t, rows.Err())
			})
		}
	})
}
