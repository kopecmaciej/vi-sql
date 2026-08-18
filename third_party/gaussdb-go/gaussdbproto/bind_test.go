package gaussdbproto_test

import (
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbproto"
	"github.com/stretchr/testify/require"
)

func TestBindBiggerThanMaxMessageBodyLen(t *testing.T) {
	t.Parallel()

	// Maximum allowed size.
	_, err := (&gaussdbproto.Bind{Parameters: [][]byte{make([]byte, gaussdbproto.MaxMessageBodyLen-16)}}).Encode(nil)
	require.NoError(t, err)

	// 1 byte too big
	_, err = (&gaussdbproto.Bind{Parameters: [][]byte{make([]byte, gaussdbproto.MaxMessageBodyLen-15)}}).Encode(nil)
	require.Error(t, err)
}
