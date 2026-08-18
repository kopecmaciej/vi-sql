package zeronull_test

import (
	"context"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype/zeronull"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
)

func isExpectedEq(a any) func(any) bool {
	return func(v any) bool {
		return a == v
	}
}

func TestFloat8Transcode(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "float8", []gaussdbxtest.ValueRoundTripTest{
		{
			(zeronull.Float8)(1),
			new(zeronull.Float8),
			isExpectedEq((zeronull.Float8)(1)),
		},
		{
			nil,
			new(zeronull.Float8),
			isExpectedEq((zeronull.Float8)(0)),
		},
		{
			(zeronull.Float8)(0),
			new(any),
			isExpectedEq(nil),
		},
	})
}
