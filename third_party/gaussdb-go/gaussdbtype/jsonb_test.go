package gaussdbtype_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	gaussdbx "github.com/HuaweiCloudDeveloper/gaussdb-go"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
	"github.com/stretchr/testify/require"
)

func TestJSONBTranscode(t *testing.T) {
	type jsonStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "jsonb", []gaussdbxtest.ValueRoundTripTest{
		{nil, new(*jsonStruct), isExpectedEq((*jsonStruct)(nil))},
		{map[string]any(nil), new(*string), isExpectedEq((*string)(nil))},
		{map[string]any(nil), new([]byte), isExpectedEqBytes([]byte(nil))},
		{[]byte(nil), new([]byte), isExpectedEqBytes([]byte(nil))},
		{nil, new([]byte), isExpectedEqBytes([]byte(nil))},
	})

	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, gaussdbxtest.KnownOIDQueryExecModes, "jsonb", []gaussdbxtest.ValueRoundTripTest{
		{[]byte("{}"), new([]byte), isExpectedEqBytes([]byte("{}"))},
		{[]byte("null"), new([]byte), isExpectedEqBytes([]byte("null"))},
		{[]byte("42"), new([]byte), isExpectedEqBytes([]byte("42"))},
		{[]byte(`"hello"`), new([]byte), isExpectedEqBytes([]byte(`"hello"`))},
		{[]byte(`"hello"`), new(string), isExpectedEq(`"hello"`)},
		{map[string]any{"foo": "bar"}, new(map[string]any), isExpectedEqMap(map[string]any{"foo": "bar"})},
		{jsonStruct{Name: "Adam", Age: 10}, new(jsonStruct), isExpectedEq(jsonStruct{Name: "Adam", Age: 10})},
	})
}

func TestJSONBCodecUnmarshalSQLNull(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *gaussdbx.Conn) {
		// Slices are nilified
		slice := []string{"foo", "bar", "baz"}
		err := conn.QueryRow(ctx, "select null::jsonb").Scan(&slice)
		require.NoError(t, err)
		require.Nil(t, slice)

		// Maps are nilified
		m := map[string]any{"foo": "bar"}
		err = conn.QueryRow(ctx, "select null::jsonb").Scan(&m)
		require.NoError(t, err)
		require.Nil(t, m)

		m = map[string]interface{}{"foo": "bar"}
		err = conn.QueryRow(ctx, "select null::jsonb").Scan(&m)
		require.NoError(t, err)
		require.Nil(t, m)

		// Pointer to pointer are nilified
		n := 42
		p := &n
		err = conn.QueryRow(ctx, "select null::jsonb").Scan(&p)
		require.NoError(t, err)
		require.Nil(t, p)

		// A string cannot scan a NULL.
		str := "foobar"
		err = conn.QueryRow(ctx, "select null::jsonb").Scan(&str)
		require.EqualError(t, err, "can't scan into dest[0] (col: jsonb): cannot scan NULL into *string")

		// A non-string cannot scan a NULL.
		err = conn.QueryRow(ctx, "select null::jsonb").Scan(&n)
		require.EqualError(t, err, "can't scan into dest[0] (col: jsonb): cannot scan NULL into *int")
	})
}

func TestJSONBCodecEncodeJSONMarshalerThatCanBeWrapped(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *gaussdbx.Conn) {
		var jsonStr string
		err := conn.QueryRow(context.Background(), "select $1::jsonb", &ParentIssue1681{}).Scan(&jsonStr)
		require.NoError(t, err)
		require.Equal(t, `{"custom": "thing"}`, jsonStr) // Note that unlike json, jsonb reformats the JSON string.
	})
}

func TestJSONBCodecCustomMarshal(t *testing.T) {
	connTestRunner := defaultConnTestRunner
	connTestRunner.AfterConnect = func(ctx context.Context, t testing.TB, conn *gaussdbx.Conn) {
		conn.TypeMap().RegisterType(&gaussdbtype.Type{
			Name: "jsonb", OID: gaussdbtype.JSONBOID, Codec: &gaussdbtype.JSONBCodec{
				Marshal: func(v any) ([]byte, error) {
					return []byte(`{"custom":"value"}`), nil
				},
				Unmarshal: func(data []byte, v any) error {
					return json.Unmarshal([]byte(`{"custom":"value"}`), v)
				},
			}})
	}

	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, connTestRunner, gaussdbxtest.KnownOIDQueryExecModes, "jsonb", []gaussdbxtest.ValueRoundTripTest{
		// There is space between "custom" and "value" in jsonb type.
		{map[string]any{"something": "else"}, new(string), isExpectedEq(`{"custom": "value"}`)},
		{[]byte(`{"something":"else"}`), new(map[string]any), func(v any) bool {
			return reflect.DeepEqual(v, map[string]any{"custom": "value"})
		}},
	})
}
