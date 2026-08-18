package gaussdbtype_test

// todo: ERROR: DOMAIN is not yet supported. (SQLSTATE 0A000)
//func TestDerivedTypes(t *testing.T) {
//	skipCockroachDB(t, "Server does not support composite types (see https://github.com/cockroachdb/cockroach/issues/27792)")
//
//	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *gaussdbx.Conn) {
//		_, err := conn.Exec(ctx, `
//drop type if exists dt_test;
//drop domain if exists dt_uint64;
//
//create domain dt_uint64 as numeric(20,0);
//create type dt_test as (
//	a text,
//    b dt_uint64,
//    c dt_uint64[]
//);`)
//		require.NoError(t, err)
//		defer conn.Exec(ctx, "drop domain dt_uint64")
//		defer conn.Exec(ctx, "drop type dt_test")
//
//		dtypes, err := conn.LoadTypes(ctx, []string{"dt_test"})
//		require.Len(t, dtypes, 6)
//		require.Equal(t, dtypes[0].Name, "public.dt_uint64")
//		require.Equal(t, dtypes[1].Name, "dt_uint64")
//		require.Equal(t, dtypes[2].Name, "public._dt_uint64")
//		require.Equal(t, dtypes[3].Name, "_dt_uint64")
//		require.Equal(t, dtypes[4].Name, "public.dt_test")
//		require.Equal(t, dtypes[5].Name, "dt_test")
//		require.NoError(t, err)
//		conn.TypeMap().RegisterTypes(dtypes)
//
//		formats := []struct {
//			name string
//			code int16
//		}{
//			{name: "TextFormat", code: gaussdbx.TextFormatCode},
//			{name: "BinaryFormat", code: gaussdbx.BinaryFormatCode},
//		}
//
//		for _, format := range formats {
//			var a string
//			var b uint64
//			var c *[]uint64
//
//			row := conn.QueryRow(ctx, "select $1::dt_test", gaussdbx.QueryResultFormats{format.code}, gaussdbtype.CompositeFields{"hi", uint64(42), []uint64{10, 20, 30}})
//			err := row.Scan(gaussdbtype.CompositeFields{&a, &b, &c})
//			require.NoError(t, err)
//			require.EqualValuesf(t, "hi", a, "%v", format.name)
//			require.EqualValuesf(t, 42, b, "%v", format.name)
//		}
//	})
//}
