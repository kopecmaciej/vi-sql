package gaussdbtype_test

import (
	"context"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
)

func TestCircleTranscode(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "circle", []gaussdbxtest.ValueRoundTripTest{
		{
			gaussdbtype.Circle{P: gaussdbtype.Vec2{1.234, 5.67890123}, R: 3.5, Valid: true},
			new(gaussdbtype.Circle),
			isExpectedEq(gaussdbtype.Circle{P: gaussdbtype.Vec2{1.234, 5.67890123}, R: 3.5, Valid: true}),
		},
		{
			gaussdbtype.Circle{P: gaussdbtype.Vec2{1.234, 5.67890123}, R: 3.5, Valid: true},
			new(gaussdbtype.Circle),
			isExpectedEq(gaussdbtype.Circle{P: gaussdbtype.Vec2{1.234, 5.67890123}, R: 3.5, Valid: true}),
		},
		{gaussdbtype.Circle{}, new(gaussdbtype.Circle), isExpectedEq(gaussdbtype.Circle{})},
		{nil, new(gaussdbtype.Circle), isExpectedEq(gaussdbtype.Circle{})},
	})
}
