package gaussdbtype_test

import (
	"context"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
)

func TestTIDCodec(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "tid", []gaussdbxtest.ValueRoundTripTest{
		{
			gaussdbtype.TID{BlockNumber: 42, OffsetNumber: 43, Valid: true},
			new(gaussdbtype.TID),
			isExpectedEq(gaussdbtype.TID{BlockNumber: 42, OffsetNumber: 43, Valid: true}),
		},
		{
			gaussdbtype.TID{BlockNumber: 4294967295, OffsetNumber: 65535, Valid: true},
			new(gaussdbtype.TID),
			isExpectedEq(gaussdbtype.TID{BlockNumber: 4294967295, OffsetNumber: 65535, Valid: true}),
		},
		{
			gaussdbtype.TID{BlockNumber: 42, OffsetNumber: 43, Valid: true},
			new(string),
			isExpectedEq("(42,43)"),
		},
		{
			gaussdbtype.TID{BlockNumber: 4294967295, OffsetNumber: 65535, Valid: true},
			new(string),
			isExpectedEq("(4294967295,65535)"),
		},
		{gaussdbtype.TID{}, new(gaussdbtype.TID), isExpectedEq(gaussdbtype.TID{})},
		{nil, new(gaussdbtype.TID), isExpectedEq(gaussdbtype.TID{})},
	})
}
