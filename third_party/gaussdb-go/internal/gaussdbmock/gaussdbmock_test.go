package gaussdbmock_test

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbconn"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbproto"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/internal/gaussdbmock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScript(t *testing.T) {
	script := &gaussdbmock.Script{
		Steps: gaussdbmock.AcceptUnauthenticatedConnRequestSteps(),
	}
	script.Steps = append(script.Steps, gaussdbmock.ExpectMessage(&gaussdbproto.Query{String: "select 42"}))
	script.Steps = append(script.Steps, gaussdbmock.SendMessage(&gaussdbproto.RowDescription{
		Fields: []gaussdbproto.FieldDescription{
			{
				Name:                 []byte("?column?"),
				TableOID:             0,
				TableAttributeNumber: 0,
				DataTypeOID:          23,
				DataTypeSize:         4,
				TypeModifier:         -1,
				Format:               0,
			},
		},
	}))
	script.Steps = append(script.Steps, gaussdbmock.SendMessage(&gaussdbproto.DataRow{
		Values: [][]byte{[]byte("42")},
	}))
	script.Steps = append(script.Steps, gaussdbmock.SendMessage(&gaussdbproto.CommandComplete{CommandTag: []byte("SELECT 1")}))
	script.Steps = append(script.Steps, gaussdbmock.SendMessage(&gaussdbproto.ReadyForQuery{TxStatus: 'I'}))
	script.Steps = append(script.Steps, gaussdbmock.ExpectMessage(&gaussdbproto.Terminate{}))

	ln, err := net.Listen("tcp", "127.0.0.1:")
	require.NoError(t, err)
	defer ln.Close()

	serverErrChan := make(chan error, 1)
	go func() {
		defer close(serverErrChan)

		conn, err := ln.Accept()
		if err != nil {
			serverErrChan <- err
			return
		}
		defer conn.Close()

		err = conn.SetDeadline(time.Now().Add(time.Second))
		if err != nil {
			serverErrChan <- err
			return
		}

		err = script.Run(gaussdbproto.NewBackend(conn, conn))
		if err != nil {
			serverErrChan <- err
			return
		}
	}()

	host, port, _ := strings.Cut(ln.Addr().String(), ":")
	connStr := fmt.Sprintf("sslmode=disable host=%s port=%s", host, port)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	gaussdbConn, err := gaussdbconn.Connect(ctx, connStr)
	require.NoError(t, err)
	results, err := gaussdbConn.Exec(ctx, "select 42").ReadAll()
	assert.NoError(t, err)

	assert.Len(t, results, 1)
	assert.Nil(t, results[0].Err)
	assert.Equal(t, "SELECT 1", results[0].CommandTag.String())
	assert.Len(t, results[0].Rows, 1)
	assert.Equal(t, "42", string(results[0].Rows[0][0]))

	gaussdbConn.Close(ctx)

	assert.NoError(t, <-serverErrChan)
}
