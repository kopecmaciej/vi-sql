package gaussdbtype_test

import (
	"context"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
)

func isExpectedEqPath(a any) func(any) bool {
	return func(v any) bool {
		ap := a.(gaussdbtype.Path)
		vp := v.(gaussdbtype.Path)

		if !(ap.Valid == vp.Valid && ap.Closed == vp.Closed && len(ap.P) == len(vp.P)) {
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

func TestPathTranscode(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "path", []gaussdbxtest.ValueRoundTripTest{
		{
			gaussdbtype.Path{
				P:      []gaussdbtype.Vec2{{3.14, 1.678901234}, {7.1, 5.234}},
				Closed: false,
				Valid:  true,
			},
			new(gaussdbtype.Path),
			isExpectedEqPath(gaussdbtype.Path{
				P:      []gaussdbtype.Vec2{{3.14, 1.678901234}, {7.1, 5.234}},
				Closed: false,
				Valid:  true,
			}),
		},
		{
			gaussdbtype.Path{
				P:      []gaussdbtype.Vec2{{3.14, 1.678}, {7.1, 5.234}, {23.1, 9.34}},
				Closed: true,
				Valid:  true,
			},
			new(gaussdbtype.Path),
			isExpectedEqPath(gaussdbtype.Path{
				P:      []gaussdbtype.Vec2{{3.14, 1.678}, {7.1, 5.234}, {23.1, 9.34}},
				Closed: true,
				Valid:  true,
			}),
		},
		{
			gaussdbtype.Path{
				P:      []gaussdbtype.Vec2{{7.1, 1.678}, {-13.14, -5.234}},
				Closed: true,
				Valid:  true,
			},
			new(gaussdbtype.Path),
			isExpectedEqPath(gaussdbtype.Path{
				P:      []gaussdbtype.Vec2{{7.1, 1.678}, {-13.14, -5.234}},
				Closed: true,
				Valid:  true,
			}),
		},
		{gaussdbtype.Path{}, new(gaussdbtype.Path), isExpectedEqPath(gaussdbtype.Path{})},
		{nil, new(gaussdbtype.Path), isExpectedEqPath(gaussdbtype.Path{})},
	})
}
