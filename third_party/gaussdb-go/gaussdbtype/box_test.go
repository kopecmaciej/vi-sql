package gaussdbtype_test

import (
	"context"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
)

func TestBoxCodec(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "box", []gaussdbxtest.ValueRoundTripTest{
		{
			gaussdbtype.Box{
				P:     [2]gaussdbtype.Vec2{{7.1, 5.2345678}, {3.14, 1.678}},
				Valid: true,
			},
			new(gaussdbtype.Box),
			isExpectedEq(gaussdbtype.Box{
				P:     [2]gaussdbtype.Vec2{{7.1, 5.2345678}, {3.14, 1.678}},
				Valid: true,
			}),
		},
		{
			gaussdbtype.Box{
				P:     [2]gaussdbtype.Vec2{{7.1, 5.2345678}, {-13.14, -5.234}},
				Valid: true,
			},
			new(gaussdbtype.Box),
			isExpectedEq(gaussdbtype.Box{
				P:     [2]gaussdbtype.Vec2{{7.1, 5.2345678}, {-13.14, -5.234}},
				Valid: true,
			}),
		},
		{gaussdbtype.Box{}, new(gaussdbtype.Box), isExpectedEq(gaussdbtype.Box{})},
		{nil, new(gaussdbtype.Box), isExpectedEq(gaussdbtype.Box{})},
	})
}
