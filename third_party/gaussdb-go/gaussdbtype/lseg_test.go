package gaussdbtype_test

import (
	"context"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
)

func TestLsegTranscode(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "lseg", []gaussdbxtest.ValueRoundTripTest{
		{
			gaussdbtype.Lseg{
				P:     [2]gaussdbtype.Vec2{{3.14, 1.678}, {7.1, 5.2345678901}},
				Valid: true,
			},
			new(gaussdbtype.Lseg),
			isExpectedEq(gaussdbtype.Lseg{
				P:     [2]gaussdbtype.Vec2{{3.14, 1.678}, {7.1, 5.2345678901}},
				Valid: true,
			}),
		},
		{
			gaussdbtype.Lseg{
				P:     [2]gaussdbtype.Vec2{{7.1, 1.678}, {-13.14, -5.234}},
				Valid: true,
			},
			new(gaussdbtype.Lseg),
			isExpectedEq(gaussdbtype.Lseg{
				P:     [2]gaussdbtype.Vec2{{7.1, 1.678}, {-13.14, -5.234}},
				Valid: true,
			}),
		},
		{gaussdbtype.Lseg{}, new(gaussdbtype.Lseg), isExpectedEq(gaussdbtype.Lseg{})},
		{nil, new(gaussdbtype.Lseg), isExpectedEq(gaussdbtype.Lseg{})},
	})
}
