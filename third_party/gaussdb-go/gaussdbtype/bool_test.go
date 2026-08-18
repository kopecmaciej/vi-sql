package gaussdbtype_test

import (
	"context"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
)

func TestBoolCodec(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "bool", []gaussdbxtest.ValueRoundTripTest{
		{true, new(bool), isExpectedEq(true)},
		{false, new(bool), isExpectedEq(false)},
		{true, new(gaussdbtype.Bool), isExpectedEq(gaussdbtype.Bool{Bool: true, Valid: true})},
		{gaussdbtype.Bool{}, new(gaussdbtype.Bool), isExpectedEq(gaussdbtype.Bool{})},
		{nil, new(*bool), isExpectedEq((*bool)(nil))},
	})
}

func TestBoolMarshalJSON(t *testing.T) {
	successfulTests := []struct {
		source gaussdbtype.Bool
		result string
	}{
		{source: gaussdbtype.Bool{}, result: "null"},
		{source: gaussdbtype.Bool{Bool: true, Valid: true}, result: "true"},
		{source: gaussdbtype.Bool{Bool: false, Valid: true}, result: "false"},
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

func TestBoolUnmarshalJSON(t *testing.T) {
	successfulTests := []struct {
		source string
		result gaussdbtype.Bool
	}{
		{source: "null", result: gaussdbtype.Bool{}},
		{source: "true", result: gaussdbtype.Bool{Bool: true, Valid: true}},
		{source: "false", result: gaussdbtype.Bool{Bool: false, Valid: true}},
	}
	for i, tt := range successfulTests {
		var r gaussdbtype.Bool
		err := r.UnmarshalJSON([]byte(tt.source))
		if err != nil {
			t.Errorf("%d: %v", i, err)
		}

		if r != tt.result {
			t.Errorf("%d: expected %v to convert to %v, but it was %v", i, tt.source, tt.result, r)
		}
	}
}
