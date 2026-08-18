package gaussdbtype_test

import (
	"context"
	"testing"

	gaussdbx "github.com/HuaweiCloudDeveloper/gaussdb-go"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
	"github.com/stretchr/testify/require"
)

type someFmtStringer struct{}

func (someFmtStringer) String() string {
	return "some fmt.Stringer"
}

func TestTextCodec(t *testing.T) {
	for _, gaussdbTypeName := range []string{"text", "varchar"} {
		gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, gaussdbTypeName, []gaussdbxtest.ValueRoundTripTest{
			{
				gaussdbtype.Text{String: "", Valid: true},
				new(gaussdbtype.Text),
				isExpectedEq(gaussdbtype.Text{String: "", Valid: true}),
			},
			{
				gaussdbtype.Text{String: "foo", Valid: true},
				new(gaussdbtype.Text),
				isExpectedEq(gaussdbtype.Text{String: "foo", Valid: true}),
			},
			{nil, new(gaussdbtype.Text), isExpectedEq(gaussdbtype.Text{})},
			{"foo", new(string), isExpectedEq("foo")},
			{someFmtStringer{}, new(string), isExpectedEq("some fmt.Stringer")},
		})
	}
}

// name is GaussDB's special 63-byte data type, used for identifiers like table names.  The pg_class.relname column
// is a good example of where the name data type is used.
//
// TextCodec does not do length checking. Inputting a longer name into GaussDB will result in silent truncation to
// 63 bytes.
//
// Length checking would be possible with a Codec specialized for "name" but it would be perfect because a
// custom-compiled GaussDB could have set NAMEDATALEN to a different value rather than the default 63.
//
// So this is simply a smoke test of the name type.
func TestTextCodecName(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "name", []gaussdbxtest.ValueRoundTripTest{
		{
			gaussdbtype.Text{String: "", Valid: true},
			new(gaussdbtype.Text),
			isExpectedEq(gaussdbtype.Text{String: "", Valid: true}),
		},
		{
			gaussdbtype.Text{String: "foo", Valid: true},
			new(gaussdbtype.Text),
			isExpectedEq(gaussdbtype.Text{String: "foo", Valid: true}),
		},
		{nil, new(gaussdbtype.Text), isExpectedEq(gaussdbtype.Text{})},
		{"foo", new(string), isExpectedEq("foo")},
	})
}

// Test fixed length char types like char(3)
func TestTextCodecBPChar(t *testing.T) {
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, defaultConnTestRunner, nil, "char(3)", []gaussdbxtest.ValueRoundTripTest{
		{
			gaussdbtype.Text{String: "a  ", Valid: true},
			new(gaussdbtype.Text),
			isExpectedEq(gaussdbtype.Text{String: "a  ", Valid: true}),
		},
		{nil, new(gaussdbtype.Text), isExpectedEq(gaussdbtype.Text{})},
		{"   ", new(string), isExpectedEq("   ")},
		{"", new(string), isExpectedEq("   ")},
		//{" 嗨 ", new(string), isExpectedEq(" 嗨 ")}, // todo The 3-byte limit of char (3) cannot store "hi" (5 bytes)
	})
}

// ACLItem is used for GaussDB's aclitem data type. A sample aclitem
// might look like this:
//
//	root=arwdDxt/root
//
// Note, however, that because the user/role name part of an aclitem is
// an identifier, it follows all the usual formatting rules for SQL
// identifiers: if it contains spaces and other special characters,
// it should appear in double-quotes:
//
//	root=arwdDxt/"role with spaces"
//
// It only supports the text format.
func TestTextCodecACLItem(t *testing.T) {
	ctr := defaultConnTestRunner
	ctr.AfterConnect = func(ctx context.Context, t testing.TB, conn *gaussdbx.Conn) {}
	str := "root=arwdDxt/root"
	if gaussdbx.IsTestingWithOpengauss() {
		str = "gaussdb=arwdDxt/gaussdb"
	}
	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, ctr, nil, "aclitem", []gaussdbxtest.ValueRoundTripTest{
		{
			gaussdbtype.Text{String: str, Valid: true},
			new(gaussdbtype.Text),
			isExpectedEq(gaussdbtype.Text{String: str, Valid: true}),
		},
		{gaussdbtype.Text{}, new(gaussdbtype.Text), isExpectedEq(gaussdbtype.Text{})},
		{nil, new(gaussdbtype.Text), isExpectedEq(gaussdbtype.Text{})},
	})
}

func TestTextCodecACLItemRoleWithSpecialCharacters(t *testing.T) {
	ctr := defaultConnTestRunner
	ctr.AfterConnect = func(ctx context.Context, t testing.TB, conn *gaussdbx.Conn) {

		// The tricky test user, below, has to actually exist so that it can be used in a test
		// of aclitem formatting. It turns out aclitems cannot contain non-existing users/roles.
		roleWithSpecialCharacters := ` tricky, ' } " \ test user `

		commandTag, err := conn.Exec(ctx, `select * from pg_roles where rolname = $1`, roleWithSpecialCharacters)
		require.NoError(t, err)

		if commandTag.RowsAffected() == 0 {
			t.Skipf("Role with special characters does not exist.")
		}
	}

	gaussdbxtest.RunValueRoundTripTests(context.Background(), t, ctr, nil, "aclitem", []gaussdbxtest.ValueRoundTripTest{
		{
			gaussdbtype.Text{String: `postgres=arwdDxt/" tricky, ' } "" \ test user "`, Valid: true},
			new(gaussdbtype.Text),
			isExpectedEq(gaussdbtype.Text{String: `postgres=arwdDxt/" tricky, ' } "" \ test user "`, Valid: true}),
		},
	})
}

func TestTextMarshalJSON(t *testing.T) {
	successfulTests := []struct {
		source gaussdbtype.Text
		result string
	}{
		{source: gaussdbtype.Text{String: ""}, result: "null"},
		{source: gaussdbtype.Text{String: "a", Valid: true}, result: "\"a\""},
	}
	for i, tt := range successfulTests {
		r, err := tt.source.MarshalJSON()
		if err != nil {
			t.Errorf("%d: %v", i, err)
		}

		if string(r) != tt.result {
			t.Errorf("%d: expected %v to convert to %v, but it was %v", i, tt.source, tt.result, string(r))
		}
	}
}

func TestTextUnmarshalJSON(t *testing.T) {
	successfulTests := []struct {
		source string
		result gaussdbtype.Text
	}{
		{source: "null", result: gaussdbtype.Text{String: ""}},
		{source: "\"a\"", result: gaussdbtype.Text{String: "a", Valid: true}},
	}
	for i, tt := range successfulTests {
		var r gaussdbtype.Text
		err := r.UnmarshalJSON([]byte(tt.source))
		if err != nil {
			t.Errorf("%d: %v", i, err)
		}

		if r != tt.result {
			t.Errorf("%d: expected %v to convert to %v, but it was %v", i, tt.source, tt.result, r)
		}
	}
}
