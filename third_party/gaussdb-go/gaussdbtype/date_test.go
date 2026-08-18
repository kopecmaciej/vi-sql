package gaussdbtype_test

import (
	"context"
	"testing"
	"time"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
	"github.com/stretchr/testify/assert"
)

func isExpectedEqTime(a any) func(any) bool {
	return func(v any) bool {
		at := a.(time.Time)
		vt := v.(time.Time)

		return at.Equal(vt)
	}
}

func TestDateCodec(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "date", []gaussdbxtest.ValueRoundTripTest{
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
		{time.Date(12200, 1, 2, 0, 0, 0, 0, time.UTC), new(time.Time), isExpectedEqTime(time.Date(12200, 1, 2, 0, 0, 0, 0, time.UTC))},
		{gaussdbtype.Date{InfinityModifier: gaussdbtype.Infinity, Valid: true}, new(gaussdbtype.Date), isExpectedEq(gaussdbtype.Date{InfinityModifier: gaussdbtype.Infinity, Valid: true})},
		{gaussdbtype.Date{InfinityModifier: gaussdbtype.NegativeInfinity, Valid: true}, new(gaussdbtype.Date), isExpectedEq(gaussdbtype.Date{InfinityModifier: gaussdbtype.NegativeInfinity, Valid: true})},
		{gaussdbtype.Date{}, new(gaussdbtype.Date), isExpectedEq(gaussdbtype.Date{})},
		{nil, new(*time.Time), isExpectedEq((*time.Time)(nil))},
	})
}

func TestDateCodecTextEncode(t *testing.T) {
	m := gaussdbtype.NewMap()

	successfulTests := []struct {
		source gaussdbtype.Date
		result string
	}{
		{source: gaussdbtype.Date{Time: time.Date(2012, 3, 29, 0, 0, 0, 0, time.UTC), Valid: true}, result: "2012-03-29"},
		{source: gaussdbtype.Date{Time: time.Date(2012, 3, 29, 10, 5, 45, 0, time.FixedZone("", -6*60*60)), Valid: true}, result: "2012-03-29"},
		{source: gaussdbtype.Date{Time: time.Date(2012, 3, 29, 10, 5, 45, 555*1000*1000, time.FixedZone("", -6*60*60)), Valid: true}, result: "2012-03-29"},
		{source: gaussdbtype.Date{Time: time.Date(789, 1, 2, 0, 0, 0, 0, time.UTC), Valid: true}, result: "0789-01-02"},
		{source: gaussdbtype.Date{Time: time.Date(89, 1, 2, 0, 0, 0, 0, time.UTC), Valid: true}, result: "0089-01-02"},
		{source: gaussdbtype.Date{Time: time.Date(9, 1, 2, 0, 0, 0, 0, time.UTC), Valid: true}, result: "0009-01-02"},
		{source: gaussdbtype.Date{Time: time.Date(12200, 1, 2, 0, 0, 0, 0, time.UTC), Valid: true}, result: "12200-01-02"},
		{source: gaussdbtype.Date{Time: time.Date(9999, 1, 2, 0, 0, 0, 0, time.UTC), Valid: true}, result: "9999-01-02"},
		{source: gaussdbtype.Date{InfinityModifier: gaussdbtype.Infinity, Valid: true}, result: "infinity"},
		{source: gaussdbtype.Date{InfinityModifier: gaussdbtype.NegativeInfinity, Valid: true}, result: "-infinity"},
	}
	for i, tt := range successfulTests {
		buf, err := m.Encode(gaussdbtype.DateOID, gaussdbtype.TextFormatCode, tt.source, nil)
		assert.NoErrorf(t, err, "%d", i)
		assert.Equalf(t, tt.result, string(buf), "%d", i)
	}
}

func TestDateMarshalJSON(t *testing.T) {
	successfulTests := []struct {
		source gaussdbtype.Date
		result string
	}{
		{source: gaussdbtype.Date{}, result: "null"},
		{source: gaussdbtype.Date{Time: time.Date(2012, 3, 29, 0, 0, 0, 0, time.UTC), Valid: true}, result: "\"2012-03-29\""},
		{source: gaussdbtype.Date{Time: time.Date(2012, 3, 29, 10, 5, 45, 0, time.FixedZone("", -6*60*60)), Valid: true}, result: "\"2012-03-29\""},
		{source: gaussdbtype.Date{Time: time.Date(2012, 3, 29, 10, 5, 45, 555*1000*1000, time.FixedZone("", -6*60*60)), Valid: true}, result: "\"2012-03-29\""},
		{source: gaussdbtype.Date{InfinityModifier: gaussdbtype.Infinity, Valid: true}, result: "\"infinity\""},
		{source: gaussdbtype.Date{InfinityModifier: gaussdbtype.NegativeInfinity, Valid: true}, result: "\"-infinity\""},
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

func TestDateUnmarshalJSON(t *testing.T) {
	successfulTests := []struct {
		source string
		result gaussdbtype.Date
	}{
		{source: "null", result: gaussdbtype.Date{}},
		{source: "\"2012-03-29\"", result: gaussdbtype.Date{Time: time.Date(2012, 3, 29, 0, 0, 0, 0, time.UTC), Valid: true}},
		{source: "\"2012-03-29\"", result: gaussdbtype.Date{Time: time.Date(2012, 3, 29, 10, 5, 45, 0, time.FixedZone("", -6*60*60)), Valid: true}},
		{source: "\"2012-03-29\"", result: gaussdbtype.Date{Time: time.Date(2012, 3, 29, 10, 5, 45, 555*1000*1000, time.FixedZone("", -6*60*60)), Valid: true}},
		{source: "\"infinity\"", result: gaussdbtype.Date{InfinityModifier: gaussdbtype.Infinity, Valid: true}},
		{source: "\"-infinity\"", result: gaussdbtype.Date{InfinityModifier: gaussdbtype.NegativeInfinity, Valid: true}},
	}
	for i, tt := range successfulTests {
		var r gaussdbtype.Date
		err := r.UnmarshalJSON([]byte(tt.source))
		if err != nil {
			t.Errorf("%d: %v", i, err)
		}

		if r.Time.Year() != tt.result.Time.Year() || r.Time.Month() != tt.result.Time.Month() || r.Time.Day() != tt.result.Time.Day() || r.Valid != tt.result.Valid || r.InfinityModifier != tt.result.InfinityModifier {
			t.Errorf("%d: expected %v to convert to %v, but it was %v", i, tt.source, tt.result, r)
		}
	}
}
