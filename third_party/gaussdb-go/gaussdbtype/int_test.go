// Code generated from gaussdbtype/int_test.go.erb. DO NOT EDIT.

package gaussdbtype_test

import (
	"context"
	"math"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
)

func TestInt2Codec(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "int2", []gaussdbxtest.ValueRoundTripTest{
		{int8(1), new(int16), isExpectedEq(int16(1))},
		{int16(1), new(int16), isExpectedEq(int16(1))},
		{int32(1), new(int16), isExpectedEq(int16(1))},
		{int64(1), new(int16), isExpectedEq(int16(1))},
		{uint8(1), new(int16), isExpectedEq(int16(1))},
		{uint16(1), new(int16), isExpectedEq(int16(1))},
		{uint32(1), new(int16), isExpectedEq(int16(1))},
		{uint64(1), new(int16), isExpectedEq(int16(1))},
		{int(1), new(int16), isExpectedEq(int16(1))},
		{uint(1), new(int16), isExpectedEq(int16(1))},
		{gaussdbtype.Int2{Int16: 1, Valid: true}, new(int16), isExpectedEq(int16(1))},
		{int32(-1), new(gaussdbtype.Int2), isExpectedEq(gaussdbtype.Int2{Int16: -1, Valid: true})},
		{1, new(int8), isExpectedEq(int8(1))},
		{1, new(int16), isExpectedEq(int16(1))},
		{1, new(int32), isExpectedEq(int32(1))},
		{1, new(int64), isExpectedEq(int64(1))},
		{1, new(uint8), isExpectedEq(uint8(1))},
		{1, new(uint16), isExpectedEq(uint16(1))},
		{1, new(uint32), isExpectedEq(uint32(1))},
		{1, new(uint64), isExpectedEq(uint64(1))},
		{1, new(int), isExpectedEq(int(1))},
		{1, new(uint), isExpectedEq(uint(1))},
		{-1, new(int8), isExpectedEq(int8(-1))},
		{-1, new(int16), isExpectedEq(int16(-1))},
		{-1, new(int32), isExpectedEq(int32(-1))},
		{-1, new(int64), isExpectedEq(int64(-1))},
		{-1, new(int), isExpectedEq(int(-1))},
		{math.MinInt16, new(int16), isExpectedEq(int16(math.MinInt16))},
		{-1, new(int16), isExpectedEq(int16(-1))},
		{0, new(int16), isExpectedEq(int16(0))},
		{1, new(int16), isExpectedEq(int16(1))},
		{math.MaxInt16, new(int16), isExpectedEq(int16(math.MaxInt16))},
		{1, new(gaussdbtype.Int2), isExpectedEq(gaussdbtype.Int2{Int16: 1, Valid: true})},
		{"1", new(string), isExpectedEq("1")},
		{gaussdbtype.Int2{}, new(gaussdbtype.Int2), isExpectedEq(gaussdbtype.Int2{})},
		{nil, new(*int16), isExpectedEq((*int16)(nil))},
	})
}

func TestInt2MarshalJSON(t *testing.T) {
	successfulTests := []struct {
		source gaussdbtype.Int2
		result string
	}{
		{source: gaussdbtype.Int2{Int16: 0}, result: "null"},
		{source: gaussdbtype.Int2{Int16: 1, Valid: true}, result: "1"},
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

func TestInt2UnmarshalJSON(t *testing.T) {
	successfulTests := []struct {
		source string
		result gaussdbtype.Int2
	}{
		{source: "null", result: gaussdbtype.Int2{Int16: 0}},
		{source: "1", result: gaussdbtype.Int2{Int16: 1, Valid: true}},
	}
	for i, tt := range successfulTests {
		var r gaussdbtype.Int2
		err := r.UnmarshalJSON([]byte(tt.source))
		if err != nil {
			t.Errorf("%d: %v", i, err)
		}

		if r != tt.result {
			t.Errorf("%d: expected %v to convert to %v, but it was %v", i, tt.source, tt.result, r)
		}
	}
}

func TestInt4Codec(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "int4", []gaussdbxtest.ValueRoundTripTest{
		{int8(1), new(int32), isExpectedEq(int32(1))},
		{int16(1), new(int32), isExpectedEq(int32(1))},
		{int32(1), new(int32), isExpectedEq(int32(1))},
		{int64(1), new(int32), isExpectedEq(int32(1))},
		{uint8(1), new(int32), isExpectedEq(int32(1))},
		{uint16(1), new(int32), isExpectedEq(int32(1))},
		{uint32(1), new(int32), isExpectedEq(int32(1))},
		{uint64(1), new(int32), isExpectedEq(int32(1))},
		{int(1), new(int32), isExpectedEq(int32(1))},
		{uint(1), new(int32), isExpectedEq(int32(1))},
		{gaussdbtype.Int4{Int32: 1, Valid: true}, new(int32), isExpectedEq(int32(1))},
		{int32(-1), new(gaussdbtype.Int4), isExpectedEq(gaussdbtype.Int4{Int32: -1, Valid: true})},
		{1, new(int8), isExpectedEq(int8(1))},
		{1, new(int16), isExpectedEq(int16(1))},
		{1, new(int32), isExpectedEq(int32(1))},
		{1, new(int64), isExpectedEq(int64(1))},
		{1, new(uint8), isExpectedEq(uint8(1))},
		{1, new(uint16), isExpectedEq(uint16(1))},
		{1, new(uint32), isExpectedEq(uint32(1))},
		{1, new(uint64), isExpectedEq(uint64(1))},
		{1, new(int), isExpectedEq(int(1))},
		{1, new(uint), isExpectedEq(uint(1))},
		{-1, new(int8), isExpectedEq(int8(-1))},
		{-1, new(int16), isExpectedEq(int16(-1))},
		{-1, new(int32), isExpectedEq(int32(-1))},
		{-1, new(int64), isExpectedEq(int64(-1))},
		{-1, new(int), isExpectedEq(int(-1))},
		{math.MinInt32, new(int32), isExpectedEq(int32(math.MinInt32))},
		{-1, new(int32), isExpectedEq(int32(-1))},
		{0, new(int32), isExpectedEq(int32(0))},
		{1, new(int32), isExpectedEq(int32(1))},
		{math.MaxInt32, new(int32), isExpectedEq(int32(math.MaxInt32))},
		{1, new(gaussdbtype.Int4), isExpectedEq(gaussdbtype.Int4{Int32: 1, Valid: true})},
		{"1", new(string), isExpectedEq("1")},
		{gaussdbtype.Int4{}, new(gaussdbtype.Int4), isExpectedEq(gaussdbtype.Int4{})},
		{nil, new(*int32), isExpectedEq((*int32)(nil))},
	})
}

func TestInt4MarshalJSON(t *testing.T) {
	successfulTests := []struct {
		source gaussdbtype.Int4
		result string
	}{
		{source: gaussdbtype.Int4{Int32: 0}, result: "null"},
		{source: gaussdbtype.Int4{Int32: 1, Valid: true}, result: "1"},
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

func TestInt4UnmarshalJSON(t *testing.T) {
	successfulTests := []struct {
		source string
		result gaussdbtype.Int4
	}{
		{source: "null", result: gaussdbtype.Int4{Int32: 0}},
		{source: "1", result: gaussdbtype.Int4{Int32: 1, Valid: true}},
	}
	for i, tt := range successfulTests {
		var r gaussdbtype.Int4
		err := r.UnmarshalJSON([]byte(tt.source))
		if err != nil {
			t.Errorf("%d: %v", i, err)
		}

		if r != tt.result {
			t.Errorf("%d: expected %v to convert to %v, but it was %v", i, tt.source, tt.result, r)
		}
	}
}

func TestInt8Codec(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "int8", []gaussdbxtest.ValueRoundTripTest{
		{int8(1), new(int64), isExpectedEq(int64(1))},
		{int16(1), new(int64), isExpectedEq(int64(1))},
		{int32(1), new(int64), isExpectedEq(int64(1))},
		{int64(1), new(int64), isExpectedEq(int64(1))},
		{uint8(1), new(int64), isExpectedEq(int64(1))},
		{uint16(1), new(int64), isExpectedEq(int64(1))},
		{uint32(1), new(int64), isExpectedEq(int64(1))},
		{uint64(1), new(int64), isExpectedEq(int64(1))},
		{int(1), new(int64), isExpectedEq(int64(1))},
		{uint(1), new(int64), isExpectedEq(int64(1))},
		{gaussdbtype.Int8{Int64: 1, Valid: true}, new(int64), isExpectedEq(int64(1))},
		{int32(-1), new(gaussdbtype.Int8), isExpectedEq(gaussdbtype.Int8{Int64: -1, Valid: true})},
		{1, new(int8), isExpectedEq(int8(1))},
		{1, new(int16), isExpectedEq(int16(1))},
		{1, new(int32), isExpectedEq(int32(1))},
		{1, new(int64), isExpectedEq(int64(1))},
		{1, new(uint8), isExpectedEq(uint8(1))},
		{1, new(uint16), isExpectedEq(uint16(1))},
		{1, new(uint32), isExpectedEq(uint32(1))},
		{1, new(uint64), isExpectedEq(uint64(1))},
		{1, new(int), isExpectedEq(int(1))},
		{1, new(uint), isExpectedEq(uint(1))},
		{-1, new(int8), isExpectedEq(int8(-1))},
		{-1, new(int16), isExpectedEq(int16(-1))},
		{-1, new(int32), isExpectedEq(int32(-1))},
		{-1, new(int64), isExpectedEq(int64(-1))},
		{-1, new(int), isExpectedEq(int(-1))},
		{math.MinInt64, new(int64), isExpectedEq(int64(math.MinInt64))},
		{-1, new(int64), isExpectedEq(int64(-1))},
		{0, new(int64), isExpectedEq(int64(0))},
		{1, new(int64), isExpectedEq(int64(1))},
		{math.MaxInt64, new(int64), isExpectedEq(int64(math.MaxInt64))},
		{1, new(gaussdbtype.Int8), isExpectedEq(gaussdbtype.Int8{Int64: 1, Valid: true})},
		{"1", new(string), isExpectedEq("1")},
		{gaussdbtype.Int8{}, new(gaussdbtype.Int8), isExpectedEq(gaussdbtype.Int8{})},
		{nil, new(*int64), isExpectedEq((*int64)(nil))},
	})
}

func TestInt8MarshalJSON(t *testing.T) {
	successfulTests := []struct {
		source gaussdbtype.Int8
		result string
	}{
		{source: gaussdbtype.Int8{Int64: 0}, result: "null"},
		{source: gaussdbtype.Int8{Int64: 1, Valid: true}, result: "1"},
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

func TestInt8UnmarshalJSON(t *testing.T) {
	successfulTests := []struct {
		source string
		result gaussdbtype.Int8
	}{
		{source: "null", result: gaussdbtype.Int8{Int64: 0}},
		{source: "1", result: gaussdbtype.Int8{Int64: 1, Valid: true}},
	}
	for i, tt := range successfulTests {
		var r gaussdbtype.Int8
		err := r.UnmarshalJSON([]byte(tt.source))
		if err != nil {
			t.Errorf("%d: %v", i, err)
		}

		if r != tt.result {
			t.Errorf("%d: expected %v to convert to %v, but it was %v", i, tt.source, tt.result, r)
		}
	}
}
