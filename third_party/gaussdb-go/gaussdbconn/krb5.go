package gaussdbconn

import (
	"errors"
	"fmt"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbproto"
)

// NewGSSFunc creates a GSS authentication provider, for use with
// RegisterGSSProvider.
type NewGSSFunc func() (GSS, error)

var newGSS NewGSSFunc

// RegisterGSSProvider registers a GSS authentication provider. For example, if
// you need to use Kerberos to authenticate with your server, add this to your
// main package:
//
//	import "github.com/otan/gopgkrb5"
//
//	func init() {
//		gaussdbconn.RegisterGSSProvider(func() (gaussdbconn.GSS, error) { return gopgkrb5.NewGSS() })
//	}
func RegisterGSSProvider(newGSSArg NewGSSFunc) {
	newGSS = newGSSArg
}

// GSS provides GSSAPI authentication (e.g., Kerberos).
type GSS interface {
	GetInitToken(host string, service string) ([]byte, error)
	GetInitTokenFromSPN(spn string) ([]byte, error)
	Continue(inToken []byte) (done bool, outToken []byte, err error)
}

func (g *GaussdbConn) gssAuth() error {
	if newGSS == nil {
		return errors.New("kerberos error: no GSSAPI provider registered, see https://github.com/otan/gopgkrb5")
	}
	cli, err := newGSS()
	if err != nil {
		return err
	}

	var nextData []byte
	if g.config.KerberosSpn != "" {
		// Use the supplied SPN if provided.
		nextData, err = cli.GetInitTokenFromSPN(g.config.KerberosSpn)
	} else {
		// Allow the kerberos service name to be overridden
		service := "postgres"
		if g.config.KerberosSrvName != "" {
			service = g.config.KerberosSrvName
		}
		nextData, err = cli.GetInitToken(g.config.Host, service)
	}
	if err != nil {
		return err
	}

	for {
		gssResponse := &gaussdbproto.GSSResponse{
			Data: nextData,
		}
		g.frontend.Send(gssResponse)
		err = g.flushWithPotentialWriteReadDeadlock()
		if err != nil {
			return err
		}
		resp, err := g.rxGSSContinue()
		if err != nil {
			return err
		}
		var done bool
		done, nextData, err = cli.Continue(resp.Data)
		if err != nil {
			return err
		}
		if done {
			break
		}
	}
	return nil
}

func (g *GaussdbConn) rxGSSContinue() (*gaussdbproto.AuthenticationGSSContinue, error) {
	msg, err := g.receiveMessage()
	if err != nil {
		return nil, err
	}

	switch m := msg.(type) {
	case *gaussdbproto.AuthenticationGSSContinue:
		return m, nil
	case *gaussdbproto.ErrorResponse:
		return nil, ErrorResponseToGuassdbError(m)
	}

	return nil, fmt.Errorf("expected AuthenticationGSSContinue message but received unexpected message %T", msg)
}
