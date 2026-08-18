package gaussdbtype_test

import (
	"context"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
)

func isExpectedEqPolygon(a any) func(any) bool {
	return func(v any) bool {
		ap := a.(gaussdbtype.Polygon)
		vp := v.(gaussdbtype.Polygon)

		if !(ap.Valid == vp.Valid && len(ap.P) == len(vp.P)) {
			return false
		}

		for i := range ap.P {
			if ap.P[i] != vp.P[i] {
				return false
			}
		}

		return true
	}
}

func TestPolygonTranscode(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "polygon", []gaussdbxtest.ValueRoundTripTest{
		{
			gaussdbtype.Polygon{
				P:     []gaussdbtype.Vec2{{3.14, 1.678901234}, {7.1, 5.234}, {5.0, 3.234}},
				Valid: true,
			},
			new(gaussdbtype.Polygon),
			isExpectedEqPolygon(gaussdbtype.Polygon{
				P:     []gaussdbtype.Vec2{{3.14, 1.678901234}, {7.1, 5.234}, {5.0, 3.234}},
				Valid: true,
			}),
		},
		{
			gaussdbtype.Polygon{
				P:     []gaussdbtype.Vec2{{3.14, -1.678}, {7.1, -5.234}, {23.1, 9.34}},
				Valid: true,
			},
			new(gaussdbtype.Polygon),
			isExpectedEqPolygon(gaussdbtype.Polygon{
				P:     []gaussdbtype.Vec2{{3.14, -1.678}, {7.1, -5.234}, {23.1, 9.34}},
				Valid: true,
			}),
		},
		{gaussdbtype.Polygon{}, new(gaussdbtype.Polygon), isExpectedEqPolygon(gaussdbtype.Polygon{})},
		{nil, new(gaussdbtype.Polygon), isExpectedEqPolygon(gaussdbtype.Polygon{})},
	})
}
