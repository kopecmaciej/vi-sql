package zeronull_test

import (
	"context"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype/zeronull"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
)

func TestUUIDTranscode(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "uuid", []gaussdbxtest.ValueRoundTripTest{
		{
			(zeronull.UUID)([16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}),
			new(zeronull.UUID),
			isExpectedEq((zeronull.UUID)([16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})),
		},
		{
			nil,
			new(zeronull.UUID),
			isExpectedEq((zeronull.UUID)([16]byte{})),
		},
		{
			(zeronull.UUID)([16]byte{}),
			new(any),
			isExpectedEq(nil),
		},
	})
}
