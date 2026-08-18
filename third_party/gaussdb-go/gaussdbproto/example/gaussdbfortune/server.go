package main

import (
	"fmt"
	"net"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbproto"
)

type GaussdbFortuneBackend struct {
	backend   *gaussdbproto.Backend
	conn      net.Conn
	responder func() ([]byte, error)
}

func NewGaussdbFortuneBackend(conn net.Conn, responder func() ([]byte, error)) *GaussdbFortuneBackend {
	backend := gaussdbproto.NewBackend(conn, conn)

	connHandler := &GaussdbFortuneBackend{
		backend:   backend,
		conn:      conn,
		responder: responder,
	}

	return connHandler
}

func (p *GaussdbFortuneBackend) Run() error {
	defer p.Close()

	err := p.handleStartup()
	if err != nil {
		return err
	}

	for {
		msg, err := p.backend.Receive()
		if err != nil {
			return fmt.Errorf("error receiving message: %w", err)
		}

		switch msg.(type) {
		case *gaussdbproto.Query:
			response, err := p.responder()
			if err != nil {
				return fmt.Errorf("error generating query response: %w", err)
			}

			buf := mustEncode((&gaussdbproto.RowDescription{Fields: []gaussdbproto.FieldDescription{
				{
					Name:                 []byte("fortune"),
					TableOID:             0,
					TableAttributeNumber: 0,
					DataTypeOID:          25,
					DataTypeSize:         -1,
					TypeModifier:         -1,
					Format:               0,
				},
			}}).Encode(nil))
			buf = mustEncode((&gaussdbproto.DataRow{Values: [][]byte{response}}).Encode(buf))
			buf = mustEncode((&gaussdbproto.CommandComplete{CommandTag: []byte("SELECT 1")}).Encode(buf))
			buf = mustEncode((&gaussdbproto.ReadyForQuery{TxStatus: 'I'}).Encode(buf))
			_, err = p.conn.Write(buf)
			if err != nil {
				return fmt.Errorf("error writing query response: %w", err)
			}
		case *gaussdbproto.Terminate:
			return nil
		default:
			return fmt.Errorf("received message other than Query from client: %#v", msg)
		}
	}
}

func (p *GaussdbFortuneBackend) handleStartup() error {
	startupMessage, err := p.backend.ReceiveStartupMessage()
	if err != nil {
		return fmt.Errorf("error receiving startup message: %w", err)
	}

	switch startupMessage.(type) {
	case *gaussdbproto.StartupMessage:
		buf := mustEncode((&gaussdbproto.AuthenticationOk{}).Encode(nil))
		buf = mustEncode((&gaussdbproto.ReadyForQuery{TxStatus: 'I'}).Encode(buf))
		_, err = p.conn.Write(buf)
		if err != nil {
			return fmt.Errorf("error sending ready for query: %w", err)
		}
	case *gaussdbproto.SSLRequest:
		_, err = p.conn.Write([]byte("N"))
		if err != nil {
			return fmt.Errorf("error sending deny SSL request: %w", err)
		}
		return p.handleStartup()
	default:
		return fmt.Errorf("unknown startup message: %#v", startupMessage)
	}

	return nil
}

func (p *GaussdbFortuneBackend) Close() error {
	return p.conn.Close()
}

func mustEncode(buf []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return buf
}
