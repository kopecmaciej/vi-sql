package gaussdbproto_test

import (
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbproto"
	"github.com/stretchr/testify/require"
)

func TestQueryBiggerThanMaxMessageBodyLen(t *testing.T) {
	t.Parallel()

	// Maximum allowed size. 4 bytes for size and 1 byte for 0 terminated string.
	_, err := (&gaussdbproto.Query{String: string(make([]byte, gaussdbproto.MaxMessageBodyLen-5))}).Encode(nil)
	require.NoError(t, err)

	// 1 byte too big
	_, err = (&gaussdbproto.Query{String: string(make([]byte, gaussdbproto.MaxMessageBodyLen-4))}).Encode(nil)
	require.Error(t, err)
}
