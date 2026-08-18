// Package gaussdbmock provides the ability to mock a GaussDB server.
package gaussdbmock

import (
	"fmt"
	"io"
	"reflect"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbproto"
)

type Step interface {
	Step(*gaussdbproto.Backend) error
}

type Script struct {
	Steps []Step
}

func (s *Script) Run(backend *gaussdbproto.Backend) error {
	for _, step := range s.Steps {
		err := step.Step(backend)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Script) Step(backend *gaussdbproto.Backend) error {
	return s.Run(backend)
}

type expectMessageStep struct {
	want gaussdbproto.FrontendMessage
	any  bool
}

func (e *expectMessageStep) Step(backend *gaussdbproto.Backend) error {
	msg, err := backend.Receive()
	if err != nil {
		return err
	}

	if e.any && reflect.TypeOf(msg) == reflect.TypeOf(e.want) {
		return nil
	}

	if !reflect.DeepEqual(msg, e.want) {
		return fmt.Errorf("msg => %#v, e.want => %#v", msg, e.want)
	}

	return nil
}

type expectStartupMessageStep struct {
	want *gaussdbproto.StartupMessage
	any  bool
}

func (e *expectStartupMessageStep) Step(backend *gaussdbproto.Backend) error {
	msg, err := backend.ReceiveStartupMessage()
	if err != nil {
		return err
	}

	if e.any {
		return nil
	}

	if !reflect.DeepEqual(msg, e.want) {
		return fmt.Errorf("msg => %#v, e.want => %#v", msg, e.want)
	}

	return nil
}

func ExpectMessage(want gaussdbproto.FrontendMessage) Step {
	return expectMessage(want, false)
}

func ExpectAnyMessage(want gaussdbproto.FrontendMessage) Step {
	return expectMessage(want, true)
}

func expectMessage(want gaussdbproto.FrontendMessage, any bool) Step {
	if want, ok := want.(*gaussdbproto.StartupMessage); ok {
		return &expectStartupMessageStep{want: want, any: any}
	}

	return &expectMessageStep{want: want, any: any}
}

type sendMessageStep struct {
	msg gaussdbproto.BackendMessage
}

func (e *sendMessageStep) Step(backend *gaussdbproto.Backend) error {
	backend.Send(e.msg)
	return backend.Flush()
}

func SendMessage(msg gaussdbproto.BackendMessage) Step {
	return &sendMessageStep{msg: msg}
}

type waitForCloseMessageStep struct{}

func (e *waitForCloseMessageStep) Step(backend *gaussdbproto.Backend) error {
	for {
		msg, err := backend.Receive()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		if _, ok := msg.(*gaussdbproto.Terminate); ok {
			return nil
		}
	}
}

func WaitForClose() Step {
	return &waitForCloseMessageStep{}
}

func AcceptUnauthenticatedConnRequestSteps() []Step {
	return []Step{
		ExpectAnyMessage(&gaussdbproto.StartupMessage{ProtocolVersion: gaussdbproto.ProtocolVersionNumber, Parameters: map[string]string{}}),
		SendMessage(&gaussdbproto.AuthenticationOk{}),
		SendMessage(&gaussdbproto.BackendKeyData{ProcessID: 0, SecretKey: 0}),
		SendMessage(&gaussdbproto.ReadyForQuery{TxStatus: 'I'}),
	}
}
