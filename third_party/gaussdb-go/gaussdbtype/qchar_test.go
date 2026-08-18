package gaussdbtype_test

import (
	"context"
	"math"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
)

func TestQcharTranscode(t *testing.T) {
	var tests []gaussdbxtest.ValueRoundTripTest
	for i := 0; i <= math.MaxUint8; i++ {
		tests = append(tests, gaussdbxtest.ValueRoundTripTest{rune(i), new(rune), isExpectedEq(rune(i))})
		tests = append(tests, gaussdbxtest.ValueRoundTripTest{byte(i), new(byte), isExpectedEq(byte(i))})
	}
	tests = append(tests, gaussdbxtest.ValueRoundTripTest{nil, new(*rune), isExpectedEq((*rune)(nil))})
	tests = append(tests, gaussdbxtest.ValueRoundTripTest{nil, new(*byte), isExpectedEq((*byte)(nil))})

	// Can only test with known OIDs as rune and byte would be considered numbers.
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, gaussdbxtest.KnownOIDQueryExecModes, `"char"`, tests)
}
