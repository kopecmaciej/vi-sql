package gaussdbtype_test

import (
	"bytes"
	"context"
	"net"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
)

func isExpectedEqHardwareAddr(a any) func(any) bool {
	return func(v any) bool {
		aa := a.(net.HardwareAddr)
		vv := v.(net.HardwareAddr)

		if (aa == nil) != (vv == nil) {
			return false
		}

		if aa == nil {
			return true
		}

		return bytes.Equal(aa, vv)
	}
}

func TestMacaddrCodec(t *testing.T) {
	// Only testing known OID query exec modes as net.HardwareAddr could map to macaddr or macaddr8.
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, gaussdbxtest.KnownOIDQueryExecModes, "macaddr", []gaussdbxtest.ValueRoundTripTest{
		{
			mustParseMacaddr(t, "01:23:45:67:89:ab"),
			new(net.HardwareAddr),
			isExpectedEqHardwareAddr(mustParseMacaddr(t, "01:23:45:67:89:ab")),
		},
		{
			"01:23:45:67:89:ab",
			new(net.HardwareAddr),
			isExpectedEqHardwareAddr(mustParseMacaddr(t, "01:23:45:67:89:ab")),
		},
		{
			mustParseMacaddr(t, "01:23:45:67:89:ab"),
			new(string),
			isExpectedEq("01:23:45:67:89:ab"),
		},
		{nil, new(*net.HardwareAddr), isExpectedEq((*net.HardwareAddr)(nil))},
	})

	// todo: gaussdb not support macaddr8 type
	/*gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, gaussdbxtest.KnownOIDQueryExecModes, "macaddr8", []gaussdbxtest.ValueRoundTripTest{
		{
			mustParseMacaddr(t, "01:23:45:67:89:ab:01:08"),
			new(net.HardwareAddr),
			isExpectedEqHardwareAddr(mustParseMacaddr(t, "01:23:45:67:89:ab:01:08")),
		},
		{
			"01:23:45:67:89:ab:01:08",
			new(net.HardwareAddr),
			isExpectedEqHardwareAddr(mustParseMacaddr(t, "01:23:45:67:89:ab:01:08")),
		},
		{
			mustParseMacaddr(t, "01:23:45:67:89:ab:01:08"),
			new(string),
			isExpectedEq("01:23:45:67:89:ab:01:08"),
		},
		{nil, new(*net.HardwareAddr), isExpectedEq((*net.HardwareAddr)(nil))},
	})*/
}
