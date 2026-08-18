package gaussdbtype_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
	"github.com/stretchr/testify/require"
)

func TestPointCodec(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "point", []gaussdbxtest.ValueRoundTripTest{
		{
			gaussdbtype.Point{P: gaussdbtype.Vec2{1.234, 5.6789012345}, Valid: true},
			new(gaussdbtype.Point),
			isExpectedEq(gaussdbtype.Point{P: gaussdbtype.Vec2{1.234, 5.6789012345}, Valid: true}),
		},
		{
			gaussdbtype.Point{P: gaussdbtype.Vec2{-1.234, -5.6789}, Valid: true},
			new(gaussdbtype.Point),
			isExpectedEq(gaussdbtype.Point{P: gaussdbtype.Vec2{-1.234, -5.6789}, Valid: true}),
		},
		{gaussdbtype.Point{}, new(gaussdbtype.Point), isExpectedEq(gaussdbtype.Point{})},
		{nil, new(gaussdbtype.Point), isExpectedEq(gaussdbtype.Point{})},
	})
}

func TestPoint_MarshalJSON(t *testing.T) {
	tests := []struct {
		name  string
		point gaussdbtype.Point
		want  []byte
	}{
		{
			name: "second",
			point: gaussdbtype.Point{
				P:     gaussdbtype.Vec2{X: 12.245, Y: 432.12},
				Valid: true,
			},
			want: []byte(`"(12.245,432.12)"`),
		},
		{
			name: "third",
			point: gaussdbtype.Point{
				P: gaussdbtype.Vec2{},
			},
			want: []byte("null"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.point.MarshalJSON()
			require.NoError(t, err)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MarshalJSON() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPoint_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		valid   bool
		arg     []byte
		wantErr bool
	}{
		{
			name:    "first",
			valid:   true,
			arg:     []byte(`"(123.123,54.12)"`),
			wantErr: false,
		},
		{
			name:    "second",
			valid:   false,
			arg:     []byte(`"(123.123,54.1sad2)"`),
			wantErr: true,
		},
		{
			name:    "third",
			valid:   false,
			arg:     []byte("null"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst := &gaussdbtype.Point{}
			if err := dst.UnmarshalJSON(tt.arg); (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if dst.Valid != tt.valid {
				t.Errorf("Valid mismatch: %v != %v", dst.Valid, tt.valid)
			}
		})
	}
}
