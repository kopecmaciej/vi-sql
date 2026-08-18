package gaussdbtype_test

import (
	"context"
	"testing"
	"time"

	gaussdbx "github.com/HuaweiCloudDeveloper/gaussdb-go"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
	"github.com/stretchr/testify/require"
)

func TestTimestamptzCodec(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "timestamptz", []gaussdbxtest.ValueRoundTripTest{
		{time.Date(-100, 1, 1, 0, 0, 0, 0, time.Local), new(time.Time), isExpectedEqTime(time.Date(-100, 1, 1, 0, 0, 0, 0, time.Local))},
		{time.Date(-1, 1, 1, 0, 0, 0, 0, time.Local), new(time.Time), isExpectedEqTime(time.Date(-1, 1, 1, 0, 0, 0, 0, time.Local))},
		{time.Date(0, 1, 1, 0, 0, 0, 0, time.Local), new(time.Time), isExpectedEqTime(time.Date(0, 1, 1, 0, 0, 0, 0, time.Local))},
		{time.Date(1, 1, 1, 0, 0, 0, 0, time.Local), new(time.Time), isExpectedEqTime(time.Date(1, 1, 1, 0, 0, 0, 0, time.Local))},

		{time.Date(1900, 1, 1, 0, 0, 0, 0, time.Local), new(time.Time), isExpectedEqTime(time.Date(1900, 1, 1, 0, 0, 0, 0, time.Local))},
		{time.Date(1970, 1, 1, 0, 0, 0, 0, time.Local), new(time.Time), isExpectedEqTime(time.Date(1970, 1, 1, 0, 0, 0, 0, time.Local))},
		{time.Date(1999, 12, 31, 0, 0, 0, 0, time.Local), new(time.Time), isExpectedEqTime(time.Date(1999, 12, 31, 0, 0, 0, 0, time.Local))},
		{time.Date(2000, 1, 1, 0, 0, 0, 0, time.Local), new(time.Time), isExpectedEqTime(time.Date(2000, 1, 1, 0, 0, 0, 0, time.Local))},
		{time.Date(2000, 1, 2, 0, 0, 0, 0, time.Local), new(time.Time), isExpectedEqTime(time.Date(2000, 1, 2, 0, 0, 0, 0, time.Local))},
		{time.Date(2200, 1, 1, 0, 0, 0, 0, time.Local), new(time.Time), isExpectedEqTime(time.Date(2200, 1, 1, 0, 0, 0, 0, time.Local))},

		// Nanosecond truncation
		{time.Date(2020, 1, 1, 0, 0, 0, 999999999, time.Local), new(time.Time), isExpectedEqTime(time.Date(2020, 1, 1, 0, 0, 0, 999999000, time.Local))},
		{time.Date(2020, 1, 1, 0, 0, 0, 999999001, time.Local), new(time.Time), isExpectedEqTime(time.Date(2020, 1, 1, 0, 0, 0, 999999000, time.Local))},

		{gaussdbtype.Timestamptz{InfinityModifier: gaussdbtype.Infinity, Valid: true}, new(gaussdbtype.Timestamptz), isExpectedEq(gaussdbtype.Timestamptz{InfinityModifier: gaussdbtype.Infinity, Valid: true})},
		{gaussdbtype.Timestamptz{InfinityModifier: gaussdbtype.NegativeInfinity, Valid: true}, new(gaussdbtype.Timestamptz), isExpectedEq(gaussdbtype.Timestamptz{InfinityModifier: gaussdbtype.NegativeInfinity, Valid: true})},
		{gaussdbtype.Timestamptz{}, new(gaussdbtype.Timestamptz), isExpectedEq(gaussdbtype.Timestamptz{})},
		{nil, new(*time.Time), isExpectedEq((*time.Time)(nil))},
	})
}

func TestTimestamptzCodecWithLocationUTC(t *testing.T) {
	connTestRunner := defaultConnTestRunner
	connTestRunner.AfterConnect = func(ctx context.Context, t testing.TB, conn *gaussdbx.Conn) {
		conn.TypeMap().RegisterType(&gaussdbtype.Type{
			Name:  "timestamptz",
			OID:   gaussdbtype.TimestamptzOID,
			Codec: &gaussdbtype.TimestamptzCodec{ScanLocation: time.UTC},
		})
	}

	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, connTestRunner, nil, "timestamptz", []gaussdbxtest.ValueRoundTripTest{
		{time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), new(time.Time), isExpectedEq(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))},
	})
}

func TestTimestamptzCodecWithLocationLocal(t *testing.T) {
	connTestRunner := defaultConnTestRunner
	connTestRunner.AfterConnect = func(ctx context.Context, t testing.TB, conn *gaussdbx.Conn) {
		conn.TypeMap().RegisterType(&gaussdbtype.Type{
			Name:  "timestamptz",
			OID:   gaussdbtype.TimestamptzOID,
			Codec: &gaussdbtype.TimestamptzCodec{ScanLocation: time.Local},
		})
	}

	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, connTestRunner, nil, "timestamptz", []gaussdbxtest.ValueRoundTripTest{
		{time.Date(2000, 1, 1, 0, 0, 0, 0, time.Local), new(time.Time), isExpectedEq(time.Date(2000, 1, 1, 0, 0, 0, 0, time.Local))},
	})
}

func TestTimestamptzTranscodeBigTimeBinary(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *gaussdbx.Conn) {
		in := &gaussdbtype.Timestamptz{Time: time.Date(294276, 12, 31, 23, 59, 59, 999999000, time.UTC), Valid: true}
		var out gaussdbtype.Timestamptz

		err := conn.QueryRow(ctx, "select $1::timestamptz", in).Scan(&out)
		if err != nil {
			t.Fatal(err)
		}

		require.Equal(t, in.Valid, out.Valid)
		require.Truef(t, in.Time.Equal(out.Time), "expected %v got %v", in.Time, out.Time)
	})
}

func TestTimestamptzDecodeTextInvalid(t *testing.T) {
	c := &gaussdbtype.TimestamptzCodec{}
	var tstz gaussdbtype.Timestamptz
	plan := c.PlanScan(nil, gaussdbtype.TimestamptzOID, gaussdbtype.TextFormatCode, &tstz)
	err := plan.Scan([]byte(`eeeee`), &tstz)
	require.Error(t, err)
}

func TestTimestamptzMarshalJSON(t *testing.T) {
	successfulTests := []struct {
		source gaussdbtype.Timestamptz
		result string
	}{
		{source: gaussdbtype.Timestamptz{}, result: "null"},
		{source: gaussdbtype.Timestamptz{Time: time.Date(2012, 3, 29, 10, 5, 45, 0, time.FixedZone("", -6*60*60)), Valid: true}, result: "\"2012-03-29T10:05:45-06:00\""},
		{source: gaussdbtype.Timestamptz{Time: time.Date(2012, 3, 29, 10, 5, 45, 555*1000*1000, time.FixedZone("", -6*60*60)), Valid: true}, result: "\"2012-03-29T10:05:45.555-06:00\""},
		{source: gaussdbtype.Timestamptz{InfinityModifier: gaussdbtype.Infinity, Valid: true}, result: "\"infinity\""},
		{source: gaussdbtype.Timestamptz{InfinityModifier: gaussdbtype.NegativeInfinity, Valid: true}, result: "\"-infinity\""},
	}
	for i, tt := range successfulTests {
		r, err := tt.source.MarshalJSON()
		if err != nil {
			t.Errorf("%d: %v", i, err)
		}

		if string(r) != tt.result {
			t.Errorf("%d: expected %v to convert to %v, but it was %v", i, tt.source, tt.result, string(r))
		}
	}
}

func TestTimestamptzUnmarshalJSON(t *testing.T) {
	successfulTests := []struct {
		source string
		result gaussdbtype.Timestamptz
	}{
		{source: "null", result: gaussdbtype.Timestamptz{}},
		{source: "\"2012-03-29T10:05:45-06:00\"", result: gaussdbtype.Timestamptz{Time: time.Date(2012, 3, 29, 10, 5, 45, 0, time.FixedZone("", -6*60*60)), Valid: true}},
		{source: "\"2012-03-29T10:05:45.555-06:00\"", result: gaussdbtype.Timestamptz{Time: time.Date(2012, 3, 29, 10, 5, 45, 555*1000*1000, time.FixedZone("", -6*60*60)), Valid: true}},
		{source: "\"infinity\"", result: gaussdbtype.Timestamptz{InfinityModifier: gaussdbtype.Infinity, Valid: true}},
		{source: "\"-infinity\"", result: gaussdbtype.Timestamptz{InfinityModifier: gaussdbtype.NegativeInfinity, Valid: true}},
	}
	for i, tt := range successfulTests {
		var r gaussdbtype.Timestamptz
		err := r.UnmarshalJSON([]byte(tt.source))
		if err != nil {
			t.Errorf("%d: %v", i, err)
		}

		if !r.Time.Equal(tt.result.Time) || r.Valid != tt.result.Valid || r.InfinityModifier != tt.result.InfinityModifier {
			t.Errorf("%d: expected %v to convert to %v, but it was %v", i, tt.source, tt.result, r)
		}
	}
}
