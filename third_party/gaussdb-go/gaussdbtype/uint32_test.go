package gaussdbtype_test

import (
	"context"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
)

func TestUint32Codec(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, gaussdbxtest.KnownOIDQueryExecModes, "oid", []gaussdbxtest.ValueRoundTripTest{
		{
			gaussdbtype.Uint32{Uint32: gaussdbtype.TextOID, Valid: true},
			new(gaussdbtype.Uint32),
			isExpectedEq(gaussdbtype.Uint32{Uint32: gaussdbtype.TextOID, Valid: true}),
		},
		{gaussdbtype.Uint32{}, new(gaussdbtype.Uint32), isExpectedEq(gaussdbtype.Uint32{})},
		{nil, new(gaussdbtype.Uint32), isExpectedEq(gaussdbtype.Uint32{})},
		{"1147", new(string), isExpectedEq("1147")},
	})
}
