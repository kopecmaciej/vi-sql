package gaussdbtype_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	gaussdbx "github.com/HuaweiCloudDeveloper/gaussdb-go"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimestampCodec(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "timestamp", []gaussdbxtest.ValueRoundTripTest{
		{time.Date(-100, 1, 1, 0, 0, 0, 0, time.UTC), new(time.Time), isExpectedEqTime(time.Date(-100, 1, 1, 0, 0, 0, 0, time.UTC))},
		{time.Date(-1, 1, 1, 0, 0, 0, 0, time.UTC), new(time.Time), isExpectedEqTime(time.Date(-1, 1, 1, 0, 0, 0, 0, time.UTC))},
		{time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC), new(time.Time), isExpectedEqTime(time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC))},
		{time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), new(time.Time), isExpectedEqTime(time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC))},

		{time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC), new(time.Time), isExpectedEqTime(time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC))},
		{time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC), new(time.Time), isExpectedEqTime(time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC))},
		{time.Date(1999, 12, 31, 0, 0, 0, 0, time.UTC), new(time.Time), isExpectedEqTime(time.Date(1999, 12, 31, 0, 0, 0, 0, time.UTC))},
		{time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), new(time.Time), isExpectedEqTime(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))},
		{time.Date(2000, 1, 2, 0, 0, 0, 0, time.UTC), new(time.Time), isExpectedEqTime(time.Date(2000, 1, 2, 0, 0, 0, 0, time.UTC))},
		{time.Date(2200, 1, 1, 0, 0, 0, 0, time.UTC), new(time.Time), isExpectedEqTime(time.Date(2200, 1, 1, 0, 0, 0, 0, time.UTC))},

		// Nanosecond truncation
		{time.Date(2020, 1, 1, 0, 0, 0, 999999999, time.UTC), new(time.Time), isExpectedEqTime(time.Date(2020, 1, 1, 0, 0, 0, 999999000, time.UTC))},
		{time.Date(2020, 1, 1, 0, 0, 0, 999999001, time.UTC), new(time.Time), isExpectedEqTime(time.Date(2020, 1, 1, 0, 0, 0, 999999000, time.UTC))},

		{gaussdbtype.Timestamp{InfinityModifier: gaussdbtype.Infinity, Valid: true}, new(gaussdbtype.Timestamp), isExpectedEq(gaussdbtype.Timestamp{InfinityModifier: gaussdbtype.Infinity, Valid: true})},
		{gaussdbtype.Timestamp{InfinityModifier: gaussdbtype.NegativeInfinity, Valid: true}, new(gaussdbtype.Timestamp), isExpectedEq(gaussdbtype.Timestamp{InfinityModifier: gaussdbtype.NegativeInfinity, Valid: true})},
		{gaussdbtype.Timestamp{}, new(gaussdbtype.Timestamp), isExpectedEq(gaussdbtype.Timestamp{})},
		{nil, new(*time.Time), isExpectedEq((*time.Time)(nil))},
	})
}

func TestTimestampCodecWithScanLocationUTC(t *testing.T) {
	connTestRunner := defaultConnTestRunner
	connTestRunner.AfterConnect = func(ctx context.Context, t testing.TB, conn *gaussdbx.Conn) {
		conn.TypeMap().RegisterType(&gaussdbtype.Type{
			Name:  "timestamp",
			OID:   gaussdbtype.TimestampOID,
			Codec: &gaussdbtype.TimestampCodec{ScanLocation: time.UTC},
		})
	}

	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, connTestRunner, nil, "timestamp", []gaussdbxtest.ValueRoundTripTest{
		// Have to use gaussdbtype.Timestamp instead of time.Time as source because otherwise the simple and exec query exec
		// modes will encode the time for timestamptz. That is, they will convert it from local time zone.
		{gaussdbtype.Timestamp{Time: time.Date(2000, 1, 1, 0, 0, 0, 0, time.Local), Valid: true}, new(time.Time), isExpectedEq(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))},
	})
}

func TestTimestampCodecWithScanLocationLocal(t *testing.T) {
	connTestRunner := defaultConnTestRunner
	connTestRunner.AfterConnect = func(ctx context.Context, t testing.TB, conn *gaussdbx.Conn) {
		conn.TypeMap().RegisterType(&gaussdbtype.Type{
			Name:  "timestamp",
			OID:   gaussdbtype.TimestampOID,
			Codec: &gaussdbtype.TimestampCodec{ScanLocation: time.Local},
		})
	}

	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, connTestRunner, nil, "timestamp", []gaussdbxtest.ValueRoundTripTest{
		{time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), new(time.Time), isExpectedEq(time.Date(2000, 1, 1, 0, 0, 0, 0, time.Local))},
	})
}

func TestTimestampTranscodeBigTimeBinary(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *gaussdbx.Conn) {
		in := &gaussdbtype.Timestamp{Time: time.Date(294276, 12, 31, 23, 59, 59, 999999000, time.UTC), Valid: true}
		var out gaussdbtype.Timestamp

		err := conn.QueryRow(ctx, "select $1::timestamp", in).Scan(&out)
		if err != nil {
			t.Fatal(err)
		}

		require.Equal(t, in.Valid, out.Valid)
		require.Truef(t, in.Time.Equal(out.Time), "expected %v got %v", in.Time, out.Time)
	})
}

func TestTimestampCodecDecodeTextInvalid(t *testing.T) {
	c := &gaussdbtype.TimestampCodec{}
	var ts gaussdbtype.Timestamp
	plan := c.PlanScan(nil, gaussdbtype.TimestampOID, gaussdbtype.TextFormatCode, &ts)
	err := plan.Scan([]byte(`eeeee`), &ts)
	require.Error(t, err)
}

func TestTimestampMarshalJSON(t *testing.T) {

	tsStruct := struct {
		TS gaussdbtype.Timestamp `json:"ts"`
	}{}

	tm := time.Date(2012, 3, 29, 10, 5, 45, 0, time.UTC)
	tsString := "\"" + tm.Format("2006-01-02T15:04:05") + "\"" //  `"2012-03-29T10:05:45"`
	var gaussdbType gaussdbtype.Timestamp
	_ = gaussdbType.Scan(tm)

	successfulTests := []struct {
		source gaussdbtype.Timestamp
		result string
	}{
		{source: gaussdbtype.Timestamp{}, result: "null"},
		{source: gaussdbtype.Timestamp{Time: tm, Valid: true}, result: tsString},
		{source: gaussdbType, result: tsString},
		{source: gaussdbtype.Timestamp{Time: tm.Add(time.Second * 555 / 1000), Valid: true}, result: `"2012-03-29T10:05:45.555"`},
		{source: gaussdbtype.Timestamp{InfinityModifier: gaussdbtype.Infinity, Valid: true}, result: "\"infinity\""},
		{source: gaussdbtype.Timestamp{InfinityModifier: gaussdbtype.NegativeInfinity, Valid: true}, result: "\"-infinity\""},
	}
	for i, tt := range successfulTests {
		r, err := tt.source.MarshalJSON()
		if err != nil {
			t.Errorf("%d: %v", i, err)
		}

		if !assert.Equal(t, tt.result, string(r)) {
			t.Errorf("%d: expected %v to convert to %v, but it was %v", i, tt.source, tt.result, string(r))
		}
		tsStruct.TS = tt.source
		b, err := json.Marshal(tsStruct)
		assert.NoErrorf(t, err, "failed to marshal %v %s", tt.source, err)
		t2 := tsStruct
		t2.TS = gaussdbtype.Timestamp{} // Clear out the value so that we can compare after unmarshalling
		err = json.Unmarshal(b, &t2)
		assert.NoErrorf(t, err, "failed to unmarshal %v with %s", tt.source, err)
		assert.True(t, tsStruct.TS.Time.Unix() == t2.TS.Time.Unix())
	}
}

func TestTimestampUnmarshalJSONErrors(t *testing.T) {
	tsStruct := struct {
		TS gaussdbtype.Timestamp `json:"ts"`
	}{}
	goodJson1 := []byte(`{"ts":"2012-03-29T10:05:45"}`)
	assert.NoError(t, json.Unmarshal(goodJson1, &tsStruct))
	goodJson2 := []byte(`{"ts":"2012-03-29T10:05:45Z"}`)
	assert.NoError(t, json.Unmarshal(goodJson2, &tsStruct))
	badJson := []byte(`{"ts":"2012-03-29"}`)
	assert.Error(t, json.Unmarshal(badJson, &tsStruct))
}

func TestTimestampUnmarshalJSON(t *testing.T) {
	successfulTests := []struct {
		source string
		result gaussdbtype.Timestamp
	}{
		{source: "null", result: gaussdbtype.Timestamp{}},
		{source: "\"2012-03-29T10:05:45\"", result: gaussdbtype.Timestamp{Time: time.Date(2012, 3, 29, 10, 5, 45, 0, time.UTC), Valid: true}},
		{source: "\"2012-03-29T10:05:45.555\"", result: gaussdbtype.Timestamp{Time: time.Date(2012, 3, 29, 10, 5, 45, 555*1000*1000, time.UTC), Valid: true}},
		{source: "\"infinity\"", result: gaussdbtype.Timestamp{InfinityModifier: gaussdbtype.Infinity, Valid: true}},
		{source: "\"-infinity\"", result: gaussdbtype.Timestamp{InfinityModifier: gaussdbtype.NegativeInfinity, Valid: true}},
	}
	for i, tt := range successfulTests {
		var r gaussdbtype.Timestamp
		err := r.UnmarshalJSON([]byte(tt.source))
		if err != nil {
			t.Errorf("%d: %v", i, err)
		}

		if !r.Time.Equal(tt.result.Time) || r.Valid != tt.result.Valid || r.InfinityModifier != tt.result.InfinityModifier {
			t.Errorf("%d: expected %v to convert to %v, but it was %v", i, tt.source, tt.result, r)
		}
	}
}
