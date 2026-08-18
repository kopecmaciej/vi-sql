package gaussdbproto_test

import (
	"bytes"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbproto"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/internal/gaussdbio"
	"github.com/stretchr/testify/require"
)

func FuzzFrontend(f *testing.F) {
	testcases := []struct {
		msgType byte
		msgLen  uint32
		msgBody []byte
	}{
		{
			msgType: 'Z',
			msgLen:  2,
			msgBody: []byte{'I'},
		},
		{
			msgType: 'Z',
			msgLen:  5,
			msgBody: []byte{'I'},
		},
	}
	for _, tc := range testcases {
		f.Add(tc.msgType, tc.msgLen, tc.msgBody)
	}
	f.Fuzz(func(t *testing.T, msgType byte, msgLen uint32, msgBody []byte) {
		// Prune any msgLen > len(msgBody) because they would hang the test waiting for more input.
		if int(msgLen) > len(msgBody)+4 {
			return
		}

		// Prune any messages that are too long.
		if msgLen > 128 || len(msgBody) > 128 {
			return
		}

		r := &bytes.Buffer{}
		w := &bytes.Buffer{}
		fe := gaussdbproto.NewFrontend(r, w)

		var encodedMsg []byte
		encodedMsg = append(encodedMsg, msgType)
		encodedMsg = gaussdbio.AppendUint32(encodedMsg, msgLen)
		encodedMsg = append(encodedMsg, msgBody...)
		_, err := r.Write(encodedMsg)
		require.NoError(t, err)

		// Not checking anything other than no panic.
		fe.Receive()
	})
}
