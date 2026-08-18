package gaussdbtype_test

// todo: ERROR: type "xid8" does not exist (SQLSTATE 42704)
//func TestUint64Codec(t *testing.T) {
//
//	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, gaussdbxtest.KnownOIDQueryExecModes, "xid8", []gaussdbxtest.ValueRoundTripTest{
//		{
//			gaussdbtype.Uint64{Uint64: 1 << 36, Valid: true},
//			new(gaussdbtype.Uint64),
//			isExpectedEq(gaussdbtype.Uint64{Uint64: 1 << 36, Valid: true}),
//		},
//		{gaussdbtype.Uint64{}, new(gaussdbtype.Uint64), isExpectedEq(gaussdbtype.Uint64{})},
//		{nil, new(gaussdbtype.Uint64), isExpectedEq(gaussdbtype.Uint64{})},
//		{
//			uint64(1 << 36),
//			new(uint64),
//			isExpectedEq(uint64(1 << 36)),
//		},
//		{"1147", new(string), isExpectedEq("1147")},
//	})
//}
