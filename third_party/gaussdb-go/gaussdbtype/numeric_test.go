package gaussdbtype_test

import (
	"context"
	"encoding/json"
	"math"
	"math/big"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"testing"

	gaussdbx "github.com/HuaweiCloudDeveloper/gaussdb-go"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustParseBigInt(t *testing.T, src string) *big.Int {
	i := &big.Int{}
	if _, ok := i.SetString(src, 10); !ok {
		t.Fatalf("could not parse big.Int: %s", src)
	}
	return i
}

func isExpectedEqNumeric(a any) func(any) bool {
	return func(v any) bool {
		aa := a.(gaussdbtype.Numeric)
		vv := v.(gaussdbtype.Numeric)

		if aa.Valid != vv.Valid {
			return false
		}

		// If NULL doesn't matter what the rest of the values are.
		if !aa.Valid {
			return true
		}

		if !(aa.NaN == vv.NaN && aa.InfinityModifier == vv.InfinityModifier) {
			return false
		}

		// If NaN or InfinityModifier are set then Int and Exp don't matter.
		if aa.NaN || aa.InfinityModifier != gaussdbtype.Finite {
			return true
		}

		aaInt := (&big.Int{}).Set(aa.Int)
		vvInt := (&big.Int{}).Set(vv.Int)

		if aa.Exp < vv.Exp {
			mul := (&big.Int{}).Exp(big.NewInt(10), big.NewInt(int64(vv.Exp-aa.Exp)), nil)
			vvInt.Mul(vvInt, mul)
		} else if aa.Exp > vv.Exp {
			mul := (&big.Int{}).Exp(big.NewInt(10), big.NewInt(int64(aa.Exp-vv.Exp)), nil)
			aaInt.Mul(aaInt, mul)
		}

		return aaInt.Cmp(vvInt) == 0
	}
}

func mustParseNumeric(t *testing.T, src string) gaussdbtype.Numeric {
	var n gaussdbtype.Numeric
	plan := gaussdbtype.NumericCodec{}.PlanScan(nil, gaussdbtype.NumericOID, gaussdbtype.TextFormatCode, &n)
	require.NotNil(t, plan)
	err := plan.Scan([]byte(src), &n)
	require.NoError(t, err)
	return n
}

// todo The precision of the NUMERIC type in GaussDB must be between 1 and 1000
func TestNumericCodec(t *testing.T) {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(147454), nil)
	max.Add(max, big.NewInt(1))
	//longestNumeric := gaussdbtype.Numeric{Int: max, Exp: -16383, Valid: true}

	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "numeric", []gaussdbxtest.ValueRoundTripTest{
		{mustParseNumeric(t, "1"), new(gaussdbtype.Numeric), isExpectedEqNumeric(mustParseNumeric(t, "1"))},
		{mustParseNumeric(t, "3.14159"), new(gaussdbtype.Numeric), isExpectedEqNumeric(mustParseNumeric(t, "3.14159"))},
		{mustParseNumeric(t, "100010001"), new(gaussdbtype.Numeric), isExpectedEqNumeric(mustParseNumeric(t, "100010001"))},
		{mustParseNumeric(t, "100010001.0001"), new(gaussdbtype.Numeric), isExpectedEqNumeric(mustParseNumeric(t, "100010001.0001"))},
		{mustParseNumeric(t, "4237234789234789289347892374324872138321894178943189043890124832108934.43219085471578891547854892438945012347981"), new(gaussdbtype.Numeric), isExpectedEqNumeric(mustParseNumeric(t, "4237234789234789289347892374324872138321894178943189043890124832108934.43219085471578891547854892438945012347981"))},
		{mustParseNumeric(t, "0.8925092023480223478923478978978937897879595901237890234789243679037419057877231734823098432903527585734549035904590854890345905434578345789347890402348952348905890489054234237489234987723894789234"), new(gaussdbtype.Numeric), isExpectedEqNumeric(mustParseNumeric(t, "0.8925092023480223478923478978978937897879595901237890234789243679037419057877231734823098432903527585734549035904590854890345905434578345789347890402348952348905890489054234237489234987723894789234"))},
		{mustParseNumeric(t, "0.000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000123"), new(gaussdbtype.Numeric), isExpectedEqNumeric(mustParseNumeric(t, "0.000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000123"))},
		{gaussdbtype.Numeric{Int: mustParseBigInt(t, "243723409723490243842378942378901237502734019231380123"), Exp: 23790, Valid: true}, new(gaussdbtype.Numeric), isExpectedEqNumeric(gaussdbtype.Numeric{Int: mustParseBigInt(t, "243723409723490243842378942378901237502734019231380123"), Exp: 23790, Valid: true})},
		{gaussdbtype.Numeric{Int: mustParseBigInt(t, "2437"), Exp: 23790, Valid: true}, new(gaussdbtype.Numeric), isExpectedEqNumeric(gaussdbtype.Numeric{Int: mustParseBigInt(t, "2437"), Exp: 23790, Valid: true})},
		{gaussdbtype.Numeric{Int: mustParseBigInt(t, "43723409723490243842378942378901237502734019231380123"), Exp: 80, Valid: true}, new(gaussdbtype.Numeric), isExpectedEqNumeric(gaussdbtype.Numeric{Int: mustParseBigInt(t, "43723409723490243842378942378901237502734019231380123"), Exp: 80, Valid: true})},
		{gaussdbtype.Numeric{Int: mustParseBigInt(t, "43723409723490243842378942378901237502734019231380123"), Exp: 81, Valid: true}, new(gaussdbtype.Numeric), isExpectedEqNumeric(gaussdbtype.Numeric{Int: mustParseBigInt(t, "43723409723490243842378942378901237502734019231380123"), Exp: 81, Valid: true})},
		{gaussdbtype.Numeric{Int: mustParseBigInt(t, "43723409723490243842378942378901237502734019231380123"), Exp: 82, Valid: true}, new(gaussdbtype.Numeric), isExpectedEqNumeric(gaussdbtype.Numeric{Int: mustParseBigInt(t, "43723409723490243842378942378901237502734019231380123"), Exp: 82, Valid: true})},
		{gaussdbtype.Numeric{Int: mustParseBigInt(t, "43723409723490243842378942378901237502734019231380123"), Exp: 83, Valid: true}, new(gaussdbtype.Numeric), isExpectedEqNumeric(gaussdbtype.Numeric{Int: mustParseBigInt(t, "43723409723490243842378942378901237502734019231380123"), Exp: 83, Valid: true})},
		{gaussdbtype.Numeric{Int: mustParseBigInt(t, "43723409723490243842378942378901237502734019231380123"), Exp: 84, Valid: true}, new(gaussdbtype.Numeric), isExpectedEqNumeric(gaussdbtype.Numeric{Int: mustParseBigInt(t, "43723409723490243842378942378901237502734019231380123"), Exp: 84, Valid: true})},
		//{gaussdbtype.Numeric{Int: mustParseBigInt(t, "913423409823409243892349028349023482934092340892390101"), Exp: -14021, Valid: true}, new(gaussdbtype.Numeric), isExpectedEqNumeric(gaussdbtype.Numeric{Int: mustParseBigInt(t, "913423409823409243892349028349023482934092340892390101"), Exp: -14021, Valid: true})},
		{gaussdbtype.Numeric{Int: mustParseBigInt(t, "13423409823409243892349028349023482934092340892390101"), Exp: -90, Valid: true}, new(gaussdbtype.Numeric), isExpectedEqNumeric(gaussdbtype.Numeric{Int: mustParseBigInt(t, "13423409823409243892349028349023482934092340892390101"), Exp: -90, Valid: true})},
		{gaussdbtype.Numeric{Int: mustParseBigInt(t, "13423409823409243892349028349023482934092340892390101"), Exp: -91, Valid: true}, new(gaussdbtype.Numeric), isExpectedEqNumeric(gaussdbtype.Numeric{Int: mustParseBigInt(t, "13423409823409243892349028349023482934092340892390101"), Exp: -91, Valid: true})},
		{gaussdbtype.Numeric{Int: mustParseBigInt(t, "13423409823409243892349028349023482934092340892390101"), Exp: -92, Valid: true}, new(gaussdbtype.Numeric), isExpectedEqNumeric(gaussdbtype.Numeric{Int: mustParseBigInt(t, "13423409823409243892349028349023482934092340892390101"), Exp: -92, Valid: true})},
		{gaussdbtype.Numeric{Int: mustParseBigInt(t, "13423409823409243892349028349023482934092340892390101"), Exp: -93, Valid: true}, new(gaussdbtype.Numeric), isExpectedEqNumeric(gaussdbtype.Numeric{Int: mustParseBigInt(t, "13423409823409243892349028349023482934092340892390101"), Exp: -93, Valid: true})},
		{gaussdbtype.Numeric{NaN: true, Valid: true}, new(gaussdbtype.Numeric), isExpectedEqNumeric(gaussdbtype.Numeric{NaN: true, Valid: true})},
		//{longestNumeric, new(gaussdbtype.Numeric), isExpectedEqNumeric(longestNumeric)},
		{mustParseNumeric(t, "1"), new(int64), isExpectedEq(int64(1))},
		{math.NaN(), new(float64), func(a any) bool { return math.IsNaN(a.(float64)) }},
		{float32(math.NaN()), new(float32), func(a any) bool { return math.IsNaN(float64(a.(float32))) }},
		{int64(-1), new(gaussdbtype.Numeric), isExpectedEqNumeric(mustParseNumeric(t, "-1"))},
		{int64(0), new(gaussdbtype.Numeric), isExpectedEqNumeric(mustParseNumeric(t, "0"))},
		{int64(1), new(gaussdbtype.Numeric), isExpectedEqNumeric(mustParseNumeric(t, "1"))},
		{int64(math.MinInt64), new(gaussdbtype.Numeric), isExpectedEqNumeric(mustParseNumeric(t, strconv.FormatInt(math.MinInt64, 10)))},
		{int64(math.MinInt64 + 1), new(gaussdbtype.Numeric), isExpectedEqNumeric(mustParseNumeric(t, strconv.FormatInt(math.MinInt64+1, 10)))},
		{int64(math.MaxInt64), new(gaussdbtype.Numeric), isExpectedEqNumeric(mustParseNumeric(t, strconv.FormatInt(math.MaxInt64, 10)))},
		{int64(math.MaxInt64 - 1), new(gaussdbtype.Numeric), isExpectedEqNumeric(mustParseNumeric(t, strconv.FormatInt(math.MaxInt64-1, 10)))},
		{uint64(100), new(uint64), isExpectedEq(uint64(100))},
		{uint64(math.MaxUint64), new(uint64), isExpectedEq(uint64(math.MaxUint64))},
		{uint(math.MaxUint), new(uint), isExpectedEq(uint(math.MaxUint))},
		{uint(100), new(uint), isExpectedEq(uint(100))},
		{"1.23", new(string), isExpectedEq("1.23")},
		{gaussdbtype.Numeric{}, new(gaussdbtype.Numeric), isExpectedEq(gaussdbtype.Numeric{})},
		{nil, new(gaussdbtype.Numeric), isExpectedEq(gaussdbtype.Numeric{})},
		{mustParseNumeric(t, "1"), new(string), isExpectedEq("1")},
		{gaussdbtype.Numeric{NaN: true, Valid: true}, new(string), isExpectedEq("NaN")},
	})

	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "int8", []gaussdbxtest.ValueRoundTripTest{
		{mustParseNumeric(t, "-1"), new(gaussdbtype.Numeric), isExpectedEqNumeric(mustParseNumeric(t, "-1"))},
		{mustParseNumeric(t, "0"), new(gaussdbtype.Numeric), isExpectedEqNumeric(mustParseNumeric(t, "0"))},
		{mustParseNumeric(t, "1"), new(gaussdbtype.Numeric), isExpectedEqNumeric(mustParseNumeric(t, "1"))},
	})
}

// todo The numeric type of GaussDB does not support input for Infinity and - Infinity
/*func TestNumericCodecInfinity(t *testing.T) {

	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "numeric", []gaussdbxtest.ValueRoundTripTest{
		{math.Inf(1), new(float64), isExpectedEq(math.Inf(1))},
		{float32(math.Inf(1)), new(float32), isExpectedEq(float32(math.Inf(1)))},
		{math.Inf(-1), new(float64), isExpectedEq(math.Inf(-1))},
		{float32(math.Inf(-1)), new(float32), isExpectedEq(float32(math.Inf(-1)))},
		{gaussdbtype.Numeric{InfinityModifier: gaussdbtype.Infinity, Valid: true}, new(gaussdbtype.Numeric), isExpectedEqNumeric(gaussdbtype.Numeric{InfinityModifier: gaussdbtype.Infinity, Valid: true})},
		{gaussdbtype.Numeric{InfinityModifier: gaussdbtype.NegativeInfinity, Valid: true}, new(gaussdbtype.Numeric), isExpectedEqNumeric(gaussdbtype.Numeric{InfinityModifier: gaussdbtype.NegativeInfinity, Valid: true})},
		{gaussdbtype.Numeric{InfinityModifier: gaussdbtype.Infinity, Valid: true}, new(string), isExpectedEq("Infinity")},
		{gaussdbtype.Numeric{InfinityModifier: gaussdbtype.NegativeInfinity, Valid: true}, new(string), isExpectedEq("-Infinity")},
	})
}*/

func TestNumericFloat64Valuer(t *testing.T) {
	for i, tt := range []struct {
		n gaussdbtype.Numeric
		f gaussdbtype.Float8
	}{
		{mustParseNumeric(t, "1"), gaussdbtype.Float8{Float64: 1, Valid: true}},
		{mustParseNumeric(t, "0.0000000000000000001"), gaussdbtype.Float8{Float64: 0.0000000000000000001, Valid: true}},
		{mustParseNumeric(t, "-99999999999"), gaussdbtype.Float8{Float64: -99999999999, Valid: true}},
		{gaussdbtype.Numeric{InfinityModifier: gaussdbtype.Infinity, Valid: true}, gaussdbtype.Float8{Float64: math.Inf(1), Valid: true}},
		{gaussdbtype.Numeric{InfinityModifier: gaussdbtype.NegativeInfinity, Valid: true}, gaussdbtype.Float8{Float64: math.Inf(-1), Valid: true}},
		{gaussdbtype.Numeric{Valid: true}, gaussdbtype.Float8{Valid: true}},
		{gaussdbtype.Numeric{}, gaussdbtype.Float8{}},
	} {
		f, err := tt.n.Float64Value()
		assert.NoErrorf(t, err, "%d", i)
		assert.Equalf(t, tt.f, f, "%d", i)
	}

	f, err := gaussdbtype.Numeric{NaN: true, Valid: true}.Float64Value()
	assert.NoError(t, err)
	assert.True(t, math.IsNaN(f.Float64))
	assert.True(t, f.Valid)
}

func TestNumericCodecFuzz(t *testing.T) {
	r := rand.New(rand.NewSource(0))
	max := &big.Int{}
	max.SetString("9999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999999", 10)

	tests := make([]gaussdbxtest.ValueRoundTripTest, 0, 2000)
	for i := 0; i < 10; i++ {
		for j := -50; j < 50; j++ {
			num := (&big.Int{}).Rand(r, max)

			n := gaussdbtype.Numeric{Int: num, Exp: int32(j), Valid: true}
			tests = append(tests, gaussdbxtest.ValueRoundTripTest{n, new(gaussdbtype.Numeric), isExpectedEqNumeric(n)})

			negNum := &big.Int{}
			negNum.Neg(num)
			n = gaussdbtype.Numeric{Int: negNum, Exp: int32(j), Valid: true}
			tests = append(tests, gaussdbxtest.ValueRoundTripTest{n, new(gaussdbtype.Numeric), isExpectedEqNumeric(n)})
		}
	}

	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "numeric", tests)
}

func TestNumericMarshalJSON(t *testing.T) {
	defaultConnTestRunner.RunTest(context.Background(), t, func(ctx context.Context, t testing.TB, conn *gaussdbx.Conn) {

		for i, tt := range []struct {
			decString    string
			skip         bool
			customFormat string
		}{
			/*{"NaN", false, ""},
			{"0", false, ""},
			{"1", false, ""},
			{"-1", false, ""},
			{"1000000000000000000", false, ""},
			{"1234.56789", false, ""},
			{"1.56789", false, ""},*/
			//{"0.00000000000056789", false, "FM0.99999999999999999"},
			{"0.00123000", false, "FM0.99999999999999999"},
			{"123e-3", false, "FM0.99999999999999999"},
			//{"243723409723490243842378942378901237502734019231380123e23790"},// todo Exceeding GaussDB numeric range
			//{"3409823409243892349028349023482934092340892390101e-14021"},// todo Exceeding GaussDB numeric range
			{"9.9999999999999999999999999999999999999", false, ""},
			{"-9.9999999999999999999999999999999999999", false, ""},
			{"-1.1", false, ""},
			{"-1.0231", false, ""},
			{"-10.0231", false, ""},
			{"-0.1", false, "FM0.99999999999999999"},   // failed with "invalid character '.' in numeric literal"
			{"-0.01", false, "FM0.99999999999999999"},  // failed with "invalid character '-' after decimal point in numeric literal"
			{"-0.001", false, "FM0.99999999999999999"}, // failed with "invalid character '-' after top-level value"
		} {
			if tt.skip {
				t.Skip("超出GaussDB numeric范围，跳过测试")
			}

			var num gaussdbtype.Numeric
			var gaussdbJSON string
			var err error

			if tt.customFormat != "" {
				err = conn.QueryRow(ctx, `select $1::numeric, to_json(to_char($1::numeric, $2))`, tt.decString, tt.customFormat).Scan(&num, &gaussdbJSON)
			} else {
				err = conn.QueryRow(ctx, `select $1::numeric, to_json($1::numeric)`, tt.decString).Scan(&num, &gaussdbJSON)
			}

			require.NoErrorf(t, err, "%d", i)

			goJSON, err := json.Marshal(num)
			require.NoErrorf(t, err, "%d", i)

			normalize := func(s string) string {
				s = strings.Trim(s, `"`)
				if strings.Contains(s, ".") {
					s = strings.TrimRight(s, "0")
					s = strings.TrimRight(s, ".")
				}
				return s
			}

			normalizeGaussdbJSON := normalize(gaussdbJSON)

			stringGoJSON := string(goJSON)
			normalizeStringGoJSON := normalize(stringGoJSON)

			require.Equal(t, normalizeGaussdbJSON, normalizeStringGoJSON)
		}
	})
}

func TestNumericUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		want    *gaussdbtype.Numeric
		src     []byte
		wantErr bool
	}{
		{
			name:    "null",
			want:    &gaussdbtype.Numeric{},
			src:     []byte(`null`),
			wantErr: false,
		},
		{
			name:    "NaN",
			want:    &gaussdbtype.Numeric{Valid: true, NaN: true},
			src:     []byte(`"NaN"`),
			wantErr: false,
		},
		{
			name:    "0",
			want:    &gaussdbtype.Numeric{Valid: true, Int: big.NewInt(0)},
			src:     []byte("0"),
			wantErr: false,
		},
		{
			name:    "1",
			want:    &gaussdbtype.Numeric{Valid: true, Int: big.NewInt(1)},
			src:     []byte("1"),
			wantErr: false,
		},
		{
			name:    "-1",
			want:    &gaussdbtype.Numeric{Valid: true, Int: big.NewInt(-1)},
			src:     []byte("-1"),
			wantErr: false,
		},
		{
			name:    "bigInt",
			want:    &gaussdbtype.Numeric{Valid: true, Int: big.NewInt(1), Exp: 30},
			src:     []byte("1000000000000000000000000000000"),
			wantErr: false,
		},
		{
			name:    "float: 1234.56789",
			want:    &gaussdbtype.Numeric{Valid: true, Int: big.NewInt(123456789), Exp: -5},
			src:     []byte("1234.56789"),
			wantErr: false,
		},
		{
			name:    "invalid value",
			want:    &gaussdbtype.Numeric{},
			src:     []byte("0xffff"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := &gaussdbtype.Numeric{}
			if err := got.UnmarshalJSON(tt.src); (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnmarshalJSON() got = %v, want %v", got, tt.want)
			}
		})
	}
}
