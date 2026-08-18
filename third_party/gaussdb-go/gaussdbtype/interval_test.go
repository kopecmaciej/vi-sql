package gaussdbtype_test

import (
	"context"
	"testing"
	"time"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
	"github.com/stretchr/testify/assert"
)

func TestIntervalCodec(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "interval", []gaussdbxtest.ValueRoundTripTest{
		{
			gaussdbtype.Interval{Microseconds: 1, Valid: true},
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Microseconds: 1, Valid: true}),
		},
		{
			gaussdbtype.Interval{Microseconds: 1000000, Valid: true},
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Microseconds: 1000000, Valid: true}),
		},
		{
			gaussdbtype.Interval{Microseconds: 1000001, Valid: true},
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Microseconds: 1000001, Valid: true}),
		},
		{
			gaussdbtype.Interval{Microseconds: 123202800000000, Valid: true},
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Microseconds: 123202800000000, Valid: true}),
		},
		{
			gaussdbtype.Interval{Days: 1, Valid: true},
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Days: 1, Valid: true}),
		},
		{
			gaussdbtype.Interval{Months: 1, Valid: true},
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Months: 1, Valid: true}),
		},
		{
			gaussdbtype.Interval{Months: 12, Valid: true},
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Months: 12, Valid: true}),
		},
		{
			gaussdbtype.Interval{Months: 13, Days: 15, Microseconds: 1000001, Valid: true},
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Months: 13, Days: 15, Microseconds: 1000001, Valid: true}),
		},
		{
			gaussdbtype.Interval{Microseconds: -1, Valid: true},
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Microseconds: -1, Valid: true}),
		},
		{
			gaussdbtype.Interval{Microseconds: -1000000, Valid: true},
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Microseconds: -1000000, Valid: true}),
		},
		{
			gaussdbtype.Interval{Microseconds: -1000001, Valid: true},
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Microseconds: -1000001, Valid: true}),
		},
		{
			gaussdbtype.Interval{Microseconds: -123202800000000, Valid: true},
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Microseconds: -123202800000000, Valid: true}),
		},
		{
			gaussdbtype.Interval{Days: -1, Valid: true},
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Days: -1, Valid: true}),
		},
		{
			gaussdbtype.Interval{Months: -1, Valid: true},
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Months: -1, Valid: true}),
		},
		{
			gaussdbtype.Interval{Months: -12, Valid: true},
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Months: -12, Valid: true}),
		},
		{
			gaussdbtype.Interval{Months: -13, Days: -15, Microseconds: -1000001, Valid: true},
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Months: -13, Days: -15, Microseconds: -1000001, Valid: true}),
		},
		{
			"1 second",
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Microseconds: 1000000, Valid: true}),
		},
		{
			"1.000001 second",
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Microseconds: 1000001, Valid: true}),
		},
		{
			"34223 hours",
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Microseconds: 123202800000000, Valid: true}),
		},
		{
			"1 day",
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Days: 1, Valid: true}),
		},
		{
			"1 month",
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Months: 1, Valid: true}),
		},
		{
			"1 year",
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Months: 12, Valid: true}),
		},
		{
			"-13 mon",
			new(gaussdbtype.Interval),
			isExpectedEq(gaussdbtype.Interval{Months: -13, Valid: true}),
		},
		{time.Hour, new(time.Duration), isExpectedEq(time.Hour)},
		{
			gaussdbtype.Interval{Months: 1, Days: 1, Valid: true},
			new(time.Duration),
			isExpectedEq(time.Duration(2678400000000000)),
		},
		{gaussdbtype.Interval{}, new(gaussdbtype.Interval), isExpectedEq(gaussdbtype.Interval{})},
		{nil, new(gaussdbtype.Interval), isExpectedEq(gaussdbtype.Interval{})},
	})
}

func TestIntervalTextEncode(t *testing.T) {
	m := gaussdbtype.NewMap()

	successfulTests := []struct {
		source gaussdbtype.Interval
		result string
	}{
		{source: gaussdbtype.Interval{Months: 2, Days: 1, Microseconds: 0, Valid: true}, result: "2 mon 1 day 00:00:00"},
		{source: gaussdbtype.Interval{Months: 0, Days: 0, Microseconds: 0, Valid: true}, result: "00:00:00"},
		{source: gaussdbtype.Interval{Months: 0, Days: 0, Microseconds: 6 * 60 * 1000000, Valid: true}, result: "00:06:00"},
		{source: gaussdbtype.Interval{Months: 0, Days: 1, Microseconds: 6*60*1000000 + 30, Valid: true}, result: "1 day 00:06:00.000030"},
	}
	for i, tt := range successfulTests {
		buf, err := m.Encode(gaussdbtype.DateOID, gaussdbtype.TextFormatCode, tt.source, nil)
		assert.NoErrorf(t, err, "%d", i)
		assert.Equalf(t, tt.result, string(buf), "%d", i)
	}
}
