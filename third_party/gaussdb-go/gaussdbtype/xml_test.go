package gaussdbtype_test

import (
	"context"
	"encoding/xml"
	"testing"

	gaussdbx "github.com/HuaweiCloudDeveloper/gaussdb-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type xmlStruct struct {
	XMLName xml.Name `xml:"person"`
	Name    string   `xml:"name"`
	Age     int      `xml:"age,attr"`
}

// todo: ERROR: unsupported XML feature (SQLSTATE 0A000)
//func TestXMLCodec(t *testing.T) {
//	skipCockroachDB(t, "CockroachDB does not support XML.")
//	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "xml", []gaussdbxtest.ValueRoundTripTest{
//		{nil, new(*xmlStruct), isExpectedEq((*xmlStruct)(nil))},
//		{map[string]any(nil), new(*string), isExpectedEq((*string)(nil))},
//		{map[string]any(nil), new([]byte), isExpectedEqBytes([]byte(nil))},
//		{[]byte(nil), new([]byte), isExpectedEqBytes([]byte(nil))},
//		{nil, new([]byte), isExpectedEqBytes([]byte(nil))},
//
//		// Test sql.Scanner.
//		{"", new(sql.NullString), isExpectedEq(sql.NullString{String: "", Valid: true})},
//
//		// Test driver.Valuer.
//		{sql.NullString{String: "", Valid: true}, new(sql.NullString), isExpectedEq(sql.NullString{String: "", Valid: true})},
//	})
//
//	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, gaussdbxtest.KnownOIDQueryExecModes, "xml", []gaussdbxtest.ValueRoundTripTest{
//		{[]byte(`<?xml version="1.0"?><Root></Root>`), new([]byte), isExpectedEqBytes([]byte(`<Root></Root>`))},
//		{[]byte(`<?xml version="1.0"?>`), new([]byte), isExpectedEqBytes([]byte(``))},
//		{[]byte(`<?xml version="1.0"?>`), new(string), isExpectedEq(``)},
//		{[]byte(`<Root></Root>`), new([]byte), isExpectedEqBytes([]byte(`<Root></Root>`))},
//		{[]byte(`<Root></Root>`), new(string), isExpectedEq(`<Root></Root>`)},
//		{[]byte(""), new([]byte), isExpectedEqBytes([]byte(""))},
//		{xmlStruct{Name: "Adam", Age: 10}, new(xmlStruct), isExpectedEq(xmlStruct{XMLName: xml.Name{Local: "person"}, Name: "Adam", Age: 10})},
//		{xmlStruct{XMLName: xml.Name{Local: "person"}, Name: "Adam", Age: 10}, new(xmlStruct), isExpectedEq(xmlStruct{XMLName: xml.Name{Local: "person"}, Name: "Adam", Age: 10})},
//		{[]byte(`<person age="10"><name>Adam</name></person>`), new(xmlStruct), isExpectedEq(xmlStruct{XMLName: xml.Name{Local: "person"}, Name: "Adam", Age: 10})},
//	})
//}

func TestXMLCodecUnmarshalSQLNull(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *gaussdbx.Conn) {
		// Byte arrays are nilified
		slice := []byte{10, 4}
		err := conn.QueryRow(ctx, "select null::xml").Scan(&slice)
		assert.NoError(t, err)
		assert.Nil(t, slice)

		// Non-pointer structs are zeroed
		m := xmlStruct{Name: "Adam"}
		err = conn.QueryRow(ctx, "select null::xml").Scan(&m)
		assert.NoError(t, err)
		assert.Empty(t, m)

		// Pointers to structs are nilified
		pm := &xmlStruct{Name: "Adam"}
		err = conn.QueryRow(ctx, "select null::xml").Scan(&pm)
		assert.NoError(t, err)
		assert.Nil(t, pm)

		// Pointer to pointer are nilified
		n := ""
		p := &n
		err = conn.QueryRow(ctx, "select null::xml").Scan(&p)
		assert.NoError(t, err)
		assert.Nil(t, p)

		// A string cannot scan a NULL.
		str := "foobar"
		err = conn.QueryRow(ctx, "select null::xml").Scan(&str)
		assert.EqualError(t, err, "can't scan into dest[0] (col: xml): cannot scan NULL into *string")
	})
}

func TestXMLCodecPointerToPointerToString(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *gaussdbx.Conn) {
		var s *string
		// todo: gaussdb not support "select ''::xml", complaints: invalid XML content (SQLSTATE 2200N)
		//err := conn.QueryRow(ctx, "select ''::xml").Scan(&s)
		//require.NoError(t, err)
		//require.Nil(t, s)
		//require.NotNil(t, s)
		//require.Equal(t, "", *s)

		err := conn.QueryRow(ctx, "select null::xml").Scan(&s)
		require.NoError(t, err)
		require.Nil(t, s)
	})
}

func TestXMLCodecDecodeValue(t *testing.T) {
	if gaussdbx.IsTestingWithOpengauss() {
		t.Skip("skip opengauss,unsupported XML feature (SQLSTATE 0A000)")
	}
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, _ testing.TB, conn *gaussdbx.Conn) {
		for _, tt := range []struct {
			sql      string
			expected any
		}{
			{
				sql:      `select '<foo>bar</foo>'::xml`,
				expected: []byte("<foo>bar</foo>"),
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
}
