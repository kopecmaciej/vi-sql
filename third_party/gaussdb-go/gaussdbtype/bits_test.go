package gaussdbtype_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
)

func isExpectedEqBits(a any) func(any) bool {
	return func(v any) bool {
		ab := a.(gaussdbtype.Bits)
		vb := v.(gaussdbtype.Bits)
		return bytes.Equal(ab.Bytes, vb.Bytes) && ab.Len == vb.Len && ab.Valid == vb.Valid
	}
}

func TestBitsCodecBit(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "bit(40)", []gaussdbxtest.ValueRoundTripTest{
		{
			gaussdbtype.Bits{Bytes: []byte{0, 0, 0, 0, 0}, Len: 40, Valid: true},
			new(gaussdbtype.Bits),
			isExpectedEqBits(gaussdbtype.Bits{Bytes: []byte{0, 0, 0, 0, 0}, Len: 40, Valid: true}),
		},
		{
			gaussdbtype.Bits{Bytes: []byte{0, 1, 128, 254, 255}, Len: 40, Valid: true},
			new(gaussdbtype.Bits),
			isExpectedEqBits(gaussdbtype.Bits{Bytes: []byte{0, 1, 128, 254, 255}, Len: 40, Valid: true}),
		},
		{gaussdbtype.Bits{}, new(gaussdbtype.Bits), isExpectedEqBits(gaussdbtype.Bits{})},
		{nil, new(gaussdbtype.Bits), isExpectedEqBits(gaussdbtype.Bits{})},
	})
}

func TestBitsCodecVarbit(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "varbit", []gaussdbxtest.ValueRoundTripTest{
		{
			gaussdbtype.Bits{Bytes: []byte{}, Len: 0, Valid: true},
			new(gaussdbtype.Bits),
			isExpectedEqBits(gaussdbtype.Bits{Bytes: []byte{}, Len: 0, Valid: true}),
		},
		{
			gaussdbtype.Bits{Bytes: []byte{0, 1, 128, 254, 255}, Len: 40, Valid: true},
			new(gaussdbtype.Bits),
			isExpectedEqBits(gaussdbtype.Bits{Bytes: []byte{0, 1, 128, 254, 255}, Len: 40, Valid: true}),
		},
		{
			gaussdbtype.Bits{Bytes: []byte{0, 1, 128, 254, 128}, Len: 33, Valid: true},
			new(gaussdbtype.Bits),
			isExpectedEqBits(gaussdbtype.Bits{Bytes: []byte{0, 1, 128, 254, 128}, Len: 33, Valid: true}),
		},
		{gaussdbtype.Bits{}, new(gaussdbtype.Bits), isExpectedEqBits(gaussdbtype.Bits{})},
		{nil, new(gaussdbtype.Bits), isExpectedEqBits(gaussdbtype.Bits{})},
	})
}
