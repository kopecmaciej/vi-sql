package gaussdbtype_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
	"github.com/stretchr/testify/require"
)

type renamedUUIDByteArray [16]byte

func TestUUIDCodec(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "uuid", []gaussdbxtest.ValueRoundTripTest{
		{
			gaussdbtype.UUID{Bytes: [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}, Valid: true},
			new(gaussdbtype.UUID),
			isExpectedEq(gaussdbtype.UUID{Bytes: [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}, Valid: true}),
		},
		{
			"00010203-0405-0607-0809-0a0b0c0d0e0f",
			new(gaussdbtype.UUID),
			isExpectedEq(gaussdbtype.UUID{Bytes: [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}, Valid: true}),
		},
		{
			"000102030405060708090a0b0c0d0e0f",
			new(gaussdbtype.UUID),
			isExpectedEq(gaussdbtype.UUID{Bytes: [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}, Valid: true}),
		},
		{
			gaussdbtype.UUID{Bytes: [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}, Valid: true},
			new(string),
			isExpectedEq("00010203-0405-0607-0809-0a0b0c0d0e0f"),
		},
		{gaussdbtype.UUID{}, new([]byte), isExpectedEqBytes([]byte(nil))},
		{gaussdbtype.UUID{}, new(gaussdbtype.UUID), isExpectedEq(gaussdbtype.UUID{})},
		{nil, new(gaussdbtype.UUID), isExpectedEq(gaussdbtype.UUID{})},
	})

	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, gaussdbxtest.KnownOIDQueryExecModes, "uuid", []gaussdbxtest.ValueRoundTripTest{
		{
			[16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			new(gaussdbtype.UUID),
			isExpectedEq(gaussdbtype.UUID{Bytes: [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}, Valid: true}),
		},
		{
			renamedUUIDByteArray{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			new(gaussdbtype.UUID),
			isExpectedEq(gaussdbtype.UUID{Bytes: [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}, Valid: true}),
		},
		{
			[]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			new(renamedUUIDByteArray),
			isExpectedEq(renamedUUIDByteArray{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}),
		},
		{
			[]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
			new(gaussdbtype.UUID),
			isExpectedEq(gaussdbtype.UUID{Bytes: [16]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}, Valid: true}),
		},
	})
}

func TestUUID_String(t *testing.T) {
	tests := []struct {
		name string
		src  gaussdbtype.UUID
		want string
	}{
		{
			name: "first",
			src: gaussdbtype.UUID{
				Bytes: [16]byte{29, 72, 90, 122, 109, 24, 69, 153, 140, 108, 52, 66, 86, 22, 136, 122},
				Valid: true,
			},
			want: "1d485a7a-6d18-4599-8c6c-34425616887a",
		},
		{
			name: "third",
			src: gaussdbtype.UUID{
				Bytes: [16]byte{},
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.src.String()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MarshalJSON() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUUID_MarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		src  gaussdbtype.UUID
		want []byte
	}{
		{
			name: "first",
			src: gaussdbtype.UUID{
				Bytes: [16]byte{29, 72, 90, 122, 109, 24, 69, 153, 140, 108, 52, 66, 86, 22, 136, 122},
				Valid: true,
			},
			want: []byte(`"1d485a7a-6d18-4599-8c6c-34425616887a"`),
		},
		{
			name: "third",
			src: gaussdbtype.UUID{
				Bytes: [16]byte{},
			},
			want: []byte("null"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.src.MarshalJSON()
			require.NoError(t, err)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MarshalJSON() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUUID_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		want    *gaussdbtype.UUID
		src     []byte
		wantErr bool
	}{
		{
			name: "first",
			want: &gaussdbtype.UUID{
				Bytes: [16]byte{29, 72, 90, 122, 109, 24, 69, 153, 140, 108, 52, 66, 86, 22, 136, 122},
				Valid: true,
			},
			src:     []byte(`"1d485a7a-6d18-4599-8c6c-34425616887a"`),
			wantErr: false,
		},
		{
			name: "second",
			want: &gaussdbtype.UUID{
				Bytes: [16]byte{},
			},
			src:     []byte("null"),
			wantErr: false,
		},
		{
			name: "third",
			want: &gaussdbtype.UUID{
				Bytes: [16]byte{},
				Valid: false,
			},
			src:     []byte("1d485a7a-6d18-4599-8c6c-34425616887a"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &gaussdbtype.UUID{}
			if err := got.UnmarshalJSON(tt.src); (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnmarshalJSON() got = %v, want %v", got, tt.want)
			}
		})
	}
}
