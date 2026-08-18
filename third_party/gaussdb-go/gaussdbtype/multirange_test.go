package gaussdbtype_test

// todo: gaussdb not support multirange type
/*func TestMultirangeCodecTranscode(t *testing.T) {

	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "int4multirange", []gaussdbxtest.ValueRoundTripTest{
		{
			gaussdbtype.Multirange[gaussdbtype.Range[gaussdbtype.Int4]](nil),
			new(gaussdbtype.Multirange[gaussdbtype.Range[gaussdbtype.Int4]]),
			func(a any) bool {
				return reflect.DeepEqual(gaussdbtype.Multirange[gaussdbtype.Range[gaussdbtype.Int4]](nil), a)
			},
		},
		{
			gaussdbtype.Multirange[gaussdbtype.Range[gaussdbtype.Int4]]{},
			new(gaussdbtype.Multirange[gaussdbtype.Range[gaussdbtype.Int4]]),
			func(a any) bool {
				return reflect.DeepEqual(gaussdbtype.Multirange[gaussdbtype.Range[gaussdbtype.Int4]]{}, a)
			},
		},
		{
			gaussdbtype.Multirange[gaussdbtype.Range[gaussdbtype.Int4]]{
				{
					Lower:     gaussdbtype.Int4{Int32: 1, Valid: true},
					Upper:     gaussdbtype.Int4{Int32: 5, Valid: true},
					LowerType: gaussdbtype.Inclusive,
					UpperType: gaussdbtype.Exclusive,
					Valid:     true,
				},
				{
					Lower:     gaussdbtype.Int4{Int32: 7, Valid: true},
					Upper:     gaussdbtype.Int4{Int32: 9, Valid: true},
					LowerType: gaussdbtype.Inclusive,
					UpperType: gaussdbtype.Exclusive,
					Valid:     true,
				},
			},
			new(gaussdbtype.Multirange[gaussdbtype.Range[gaussdbtype.Int4]]),
			func(a any) bool {
				return reflect.DeepEqual(gaussdbtype.Multirange[gaussdbtype.Range[gaussdbtype.Int4]]{
					{
						Lower:     gaussdbtype.Int4{Int32: 1, Valid: true},
						Upper:     gaussdbtype.Int4{Int32: 5, Valid: true},
						LowerType: gaussdbtype.Inclusive,
						UpperType: gaussdbtype.Exclusive,
						Valid:     true,
					},
					{
						Lower:     gaussdbtype.Int4{Int32: 7, Valid: true},
						Upper:     gaussdbtype.Int4{Int32: 9, Valid: true},
						LowerType: gaussdbtype.Inclusive,
						UpperType: gaussdbtype.Exclusive,
						Valid:     true,
					},
				}, a)
			},
		},
	})
}*/

// todo: gaussdb not support multirange type
/*func TestMultirangeCodecDecodeValue(t *testing.T) {

	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, _ testing.TB, conn *gaussdbx.Conn) {

		for _, tt := range []struct {
			sql      string
			expected any
		}{
			{
				sql: `select int4multirange(int4range(1, 5), int4range(7,9))`,
				expected: gaussdbtype.Multirange[gaussdbtype.Range[any]]{
					{
						Lower:     int32(1),
						Upper:     int32(5),
						LowerType: gaussdbtype.Inclusive,
						UpperType: gaussdbtype.Exclusive,
						Valid:     true,
					},
					{
						Lower:     int32(7),
						Upper:     int32(9),
						LowerType: gaussdbtype.Inclusive,
						UpperType: gaussdbtype.Exclusive,
						Valid:     true,
					},
				},
			},
		} {
			t.Run(tt.sql, func(t *testing.T) {
				rows, err := conn.Query(ctx, tt.sql)
				require.NoError(t, err)

				for rows.Next() {
					values, err := rows.Values()
					require.NoError(t, err)
					require.Len(t, values, 1)
					require.Equal(t, tt.expected, values[0])
				}

				require.NoError(t, rows.Err())
			})
		}
	})
}*/
