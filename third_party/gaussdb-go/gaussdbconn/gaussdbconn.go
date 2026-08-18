package gaussdbconn

import (
	"container/list"
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbconn/ctxwatch"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbconn/internal/bgreader"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbproto"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/internal/gaussdbio"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/internal/iobufpool"
)

const (
	connStatusUninitialized = iota
	connStatusConnecting
	connStatusClosed
	connStatusIdle
	connStatusBusy
)

// Notice represents a notice response message reported by the GaussDB server. Be aware that this is distinct from
// LISTEN/NOTIFY notification.
type Notice GaussdbError

// Notification is a message received from the GaussDB LISTEN/NOTIFY system
type Notification struct {
	PID     uint32 // backend pid that sent the notification
	Channel string // channel from which notification was received
	Payload string
}

// DialFunc is a function that can be used to connect to a GaussDB server.
type DialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// LookupFunc is a function that can be used to lookup IPs addrs from host. Optionally an ip:port combination can be
// returned in order to override the connection string's port.
type LookupFunc func(ctx context.Context, host string) (addrs []string, err error)

// BuildFrontendFunc is a function that can be used to create Frontend implementation for connection.
type BuildFrontendFunc func(r io.Reader, w io.Writer) *gaussdbproto.Frontend

// GaussdbErrorHandler is a function that handles errors returned from Gaussdb. This function must return true to keep
// the connection open. Returning false will cause the connection to be closed immediately. You should return
// false on any FATAL-severity errors. This will not receive network errors. The *GaussdbConn is provided so the handler is
// aware of the origin of the error, but it must not invoke any query method.
type GaussdbErrorHandler func(*GaussdbConn, *GaussdbError) bool

// NoticeHandler is a function that can handle notices received from the GaussDB server. Notices can be received at
// any time, usually during handling of a query response. The *GaussdbConn is provided so the handler is aware of the origin
// of the notice, but it must not invoke any query method. Be aware that this is distinct from LISTEN/NOTIFY
// notification.
type NoticeHandler func(*GaussdbConn, *Notice)

// NotificationHandler is a function that can handle notifications received from the GaussDB server. Notifications
// can be received at any time, usually during handling of a query response. The *GaussdbConn is provided so the handler is
// aware of the origin of the notice, but it must not invoke any query method. Be aware that this is distinct from a
// notice event.
type NotificationHandler func(*GaussdbConn, *Notification)

// GaussdbConn is a low-level GaussDB connection handle. It is not safe for concurrent usage.
type GaussdbConn struct {
	scratch []byte

	conn              net.Conn
	pid               uint32            // backend pid
	secretKey         uint32            // key to use to send a cancel query message to the server
	parameterStatuses map[string]string // parameters that have been reported by the server
	txStatus          byte
	frontend          *gaussdbproto.Frontend
	bgReader          *bgreader.BGReader
	slowWriteTimer    *time.Timer
	bgReaderStarted   chan struct{}

	customData map[string]any

	config *Config

	status byte // One of connStatus* constants

	bufferingReceive    bool
	bufferingReceiveMux sync.Mutex
	bufferingReceiveMsg gaussdbproto.BackendMessage
	bufferingReceiveErr error

	peekedMsg gaussdbproto.BackendMessage

	// Reusable / preallocated resources
	resultReader      ResultReader
	multiResultReader MultiResultReader
	pipeline          Pipeline
	contextWatcher    *ctxwatch.ContextWatcher
	fieldDescriptions [16]FieldDescription

	cleanupDone chan struct{}
}

// Connect establishes a connection to a GaussDB server using the environment and connString (in URL or keyword/value
// format) to provide configuration. See documentation for [ParseConfig] for details. ctx can be used to cancel a
// connect attempt.
func Connect(ctx context.Context, connString string) (*GaussdbConn, error) {
	config, err := ParseConfig(connString)
	if err != nil {
		return nil, err
	}

	return ConnectConfig(ctx, config)
}

// Connect establishes a connection to a GaussDB server using the environment and connString (in URL or keyword/value
// format) and ParseConfigOptions to provide additional configuration. See documentation for [ParseConfig] for details.
// ctx can be used to cancel a connect attempt.
func ConnectWithOptions(ctx context.Context, connString string, parseConfigOptions ParseConfigOptions) (*GaussdbConn, error) {
	config, err := ParseConfigWithOptions(connString, parseConfigOptions)
	if err != nil {
		return nil, err
	}

	return ConnectConfig(ctx, config)
}

// ConnectConfig establishes a connection to a GaussDB server using config. config must have been constructed with
// [ParseConfig]. ctx can be used to cancel a connect attempt.
//
// If config.Fallbacks are present they will sequentially be tried in case of error establishing network connection. An
// authentication error will terminate the chain of attempts and be returned as the error.
func ConnectConfig(ctx context.Context, config *Config) (*GaussdbConn, error) {
	// Default values are set in ParseConfig. Enforce initial creation by ParseConfig rather than setting defaults from
	// zero values.
	if !config.createdByParseConfig {
		panic("config must be created by ParseConfig")
	}

	var allErrors []error

	connectConfigs, errs := buildConnectOneConfigs(ctx, config)
	if len(errs) > 0 {
		allErrors = append(allErrors, errs...)
	}

	if len(connectConfigs) == 0 {
		return nil, &ConnectError{Config: config, err: fmt.Errorf("hostname resolving error: %w", errors.Join(allErrors...))}
	}

	gaussdbConn, errs := connectPreferred(ctx, config, connectConfigs)
	if len(errs) > 0 {
		allErrors = append(allErrors, errs...)
		return nil, &ConnectError{Config: config, err: errors.Join(allErrors...)}
	}

	if config.AfterConnect != nil {
		err := config.AfterConnect(ctx, gaussdbConn)
		if err != nil {
			gaussdbConn.conn.Close()
			return nil, &ConnectError{Config: config, err: fmt.Errorf("AfterConnect error: %w", err)}
		}
	}

	return gaussdbConn, nil
}

// buildConnectOneConfigs resolves hostnames and builds a list of connectOneConfigs to try connecting to. It returns a
// slice of successfully resolved connectOneConfigs and a slice of errors. It is possible for both slices to contain
// values if some hosts were successfully resolved and others were not.
func buildConnectOneConfigs(ctx context.Context, config *Config) ([]*connectOneConfig, []error) {
	// Simplify usage by treating primary config and fallbacks the same.
	fallbackConfigs := []*FallbackConfig{
		{
			Host:      config.Host,
			Port:      config.Port,
			TLSConfig: config.TLSConfig,
		},
	}
	fallbackConfigs = append(fallbackConfigs, config.Fallbacks...)

	var configs []*connectOneConfig

	var allErrors []error

	for _, fb := range fallbackConfigs {
		// skip resolve for unix sockets
		if isAbsolutePath(fb.Host) {
			network, address := NetworkAddress(fb.Host, fb.Port)
			configs = append(configs, &connectOneConfig{
				network:          network,
				address:          address,
				originalHostname: fb.Host,
				tlsConfig:        fb.TLSConfig,
			})

			continue
		}

		ips, err := config.LookupFunc(ctx, fb.Host)
		if err != nil {
			allErrors = append(allErrors, err)
			continue
		}

		for _, ip := range ips {
			splitIP, splitPort, err := net.SplitHostPort(ip)
			if err == nil {
				port, err := strconv.ParseUint(splitPort, 10, 16)
				if err != nil {
					return nil, []error{fmt.Errorf("error parsing port (%s) from lookup: %w", splitPort, err)}
				}
				network, address := NetworkAddress(splitIP, uint16(port))
				configs = append(configs, &connectOneConfig{
					network:          network,
					address:          address,
					originalHostname: fb.Host,
					tlsConfig:        fb.TLSConfig,
				})
			} else {
				network, address := NetworkAddress(ip, fb.Port)
				configs = append(configs, &connectOneConfig{
					network:          network,
					address:          address,
					originalHostname: fb.Host,
					tlsConfig:        fb.TLSConfig,
				})
			}
		}
	}

	return configs, allErrors
}

// connectPreferred attempts to connect to the preferred host from connectOneConfigs. The connections are attempted in
// order. If a connection is successful it is returned. If no connection is successful then all errors are returned. If
// a connection attempt returns a [NotPreferredError], then that host will be used if no other hosts are successful.
func connectPreferred(ctx context.Context, config *Config, connectOneConfigs []*connectOneConfig) (*GaussdbConn, []error) {
	octx := ctx
	var allErrors []error

	var fallbackConnectOneConfig *connectOneConfig
	for i, c := range connectOneConfigs {
		// ConnectTimeout restricts the whole connection process.
		if config.ConnectTimeout != 0 {
			// create new context first time or when previous host was different
			if i == 0 || (connectOneConfigs[i].address != connectOneConfigs[i-1].address) {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(octx, config.ConnectTimeout)
				defer cancel()
			}
		} else {
			ctx = octx
		}

		gaussdbConn, err := connectOne(ctx, config, c, false)
		if gaussdbConn != nil {
			return gaussdbConn, nil
		}

		allErrors = append(allErrors, err)

		var gaussdbError *GaussdbError
		if errors.As(err, &gaussdbError) {
			const ERRCODE_INVALID_PASSWORD = "28P01"                    // wrong password
			const ERRCODE_INVALID_AUTHORIZATION_SPECIFICATION = "28000" // wrong password or bad pg_hba.conf settings
			const ERRCODE_INVALID_CATALOG_NAME = "3D000"                // db does not exist
			const ERRCODE_INSUFFICIENT_PRIVILEGE = "42501"              // missing connect privilege
			if gaussdbError.Code == ERRCODE_INVALID_PASSWORD ||
				gaussdbError.Code == ERRCODE_INVALID_AUTHORIZATION_SPECIFICATION && c.tlsConfig != nil ||
				gaussdbError.Code == ERRCODE_INVALID_CATALOG_NAME ||
				gaussdbError.Code == ERRCODE_INSUFFICIENT_PRIVILEGE {
				return nil, allErrors
			}
		}

		var npErr *NotPreferredError
		if errors.As(err, &npErr) {
			fallbackConnectOneConfig = c
		}
	}

	if fallbackConnectOneConfig != nil {
		gaussdbConn, err := connectOne(ctx, config, fallbackConnectOneConfig, true)
		if err == nil {
			return gaussdbConn, nil
		}
		allErrors = append(allErrors, err)
	}

	return nil, allErrors
}

// connectOne makes one connection attempt to a single host.
func connectOne(ctx context.Context, config *Config, connectConfig *connectOneConfig,
	ignoreNotPreferredErr bool,
) (*GaussdbConn, error) {
	gaussdbConn := new(GaussdbConn)
	gaussdbConn.config = config
	gaussdbConn.cleanupDone = make(chan struct{})
	gaussdbConn.customData = make(map[string]any)

	var err error

	newPerDialConnectError := func(msg string, err error) *perDialConnectError {
		err = normalizeTimeoutError(ctx, err)
		e := &perDialConnectError{address: connectConfig.address, originalHostname: connectConfig.originalHostname, err: fmt.Errorf("%s: %w", msg, err)}
		return e
	}

	gaussdbConn.conn, err = config.DialFunc(ctx, connectConfig.network, connectConfig.address)
	if err != nil {
		return nil, newPerDialConnectError("dial error", err)
	}

	if config.minReadBufferSize == 0 {
		config.minReadBufferSize = 8192
	}
	gaussdbConn.scratch = make([]byte, config.minReadBufferSize)

	if connectConfig.tlsConfig != nil {
		gaussdbConn.contextWatcher = ctxwatch.NewContextWatcher(&DeadlineContextWatcherHandler{Conn: gaussdbConn.conn})
		gaussdbConn.contextWatcher.Watch(ctx)
		tlsConn, err := startTLS(gaussdbConn.conn, connectConfig.tlsConfig)
		gaussdbConn.contextWatcher.Unwatch() // Always unwatch `netConn` after TLS.
		if err != nil {
			gaussdbConn.conn.Close()
			return nil, newPerDialConnectError("tls error", err)
		}

		gaussdbConn.conn = tlsConn
	}

	gaussdbConn.contextWatcher = ctxwatch.NewContextWatcher(config.BuildContextWatcherHandler(gaussdbConn))
	gaussdbConn.contextWatcher.Watch(ctx)
	defer gaussdbConn.contextWatcher.Unwatch()

	gaussdbConn.parameterStatuses = make(map[string]string)
	gaussdbConn.status = connStatusConnecting
	gaussdbConn.bgReader = bgreader.New(gaussdbConn.conn)
	gaussdbConn.slowWriteTimer = time.AfterFunc(time.Duration(math.MaxInt64),
		func() {
			gaussdbConn.bgReader.Start()
			gaussdbConn.bgReaderStarted <- struct{}{}
		},
	)
	gaussdbConn.slowWriteTimer.Stop()
	gaussdbConn.bgReaderStarted = make(chan struct{})
	gaussdbConn.frontend = config.BuildFrontend(gaussdbConn.bgReader, gaussdbConn.conn)

	startupMsg := gaussdbproto.StartupMessage{
		ProtocolVersion: gaussdbproto.ProtocolVersionNumber,
		Parameters:      make(map[string]string),
	}

	// Copy default run-time params
	for k, v := range config.RuntimeParams {
		startupMsg.Parameters[k] = v
	}

	startupMsg.Parameters["user"] = config.User
	if config.Database != "" {
		startupMsg.Parameters["database"] = config.Database
	}

	gaussdbConn.frontend.Send(&startupMsg)
	if err := gaussdbConn.flushWithPotentialWriteReadDeadlock(); err != nil {
		gaussdbConn.conn.Close()
		return nil, newPerDialConnectError("failed to write startup message", err)
	}

	for {
		msg, err := gaussdbConn.receiveMessage()
		if err != nil {
			gaussdbConn.conn.Close()
			if err, ok := err.(*GaussdbError); ok {
				return nil, newPerDialConnectError("server error", err)
			}
			return nil, newPerDialConnectError("failed to receive message", err)
		}

		switch msg := msg.(type) {
		case *gaussdbproto.BackendKeyData:
			gaussdbConn.pid = msg.ProcessID
			gaussdbConn.secretKey = msg.SecretKey

		case *gaussdbproto.AuthenticationOk:
		case *gaussdbproto.AuthenticationCleartextPassword:
			err = gaussdbConn.txPasswordMessage(gaussdbConn.config.Password)
			if err != nil {
				gaussdbConn.conn.Close()
				return nil, newPerDialConnectError("failed to write password message", err)
			}
		case *gaussdbproto.AuthenticationMD5Password:
			digestedPassword := "md5" + hexMD5(hexMD5(gaussdbConn.config.Password+gaussdbConn.config.User)+string(msg.Salt[:]))
			err = gaussdbConn.txPasswordMessage(digestedPassword)
			if err != nil {
				gaussdbConn.conn.Close()
				return nil, newPerDialConnectError("failed to write password message", err)
			}
		case *gaussdbproto.AuthenticationSASL:
			err = gaussdbConn.scramAuth(msg.AuthMechanisms)
			if err != nil {
				gaussdbConn.conn.Close()
				return nil, newPerDialConnectError("failed SASL auth", err)
			}
		case *gaussdbproto.AuthenticationSHA256:
			err = gaussdbConn.scramSha256Auth(msg.GetR())
			if err != nil {
				gaussdbConn.conn.Close()
				return nil, newPerDialConnectError("failed SASL SHA256 auth", err)
			}
		case *gaussdbproto.AuthenticationGSS:
			err = gaussdbConn.gssAuth()
			if err != nil {
				gaussdbConn.conn.Close()
				return nil, newPerDialConnectError("failed GSS auth", err)
			}
		case *gaussdbproto.ReadyForQuery:
			gaussdbConn.status = connStatusIdle
			if config.ValidateConnect != nil {
				// ValidateConnect may execute commands that cause the context to be watched again. Unwatch first to avoid
				// the watch already in progress panic. This is that last thing done by this method so there is no need to
				// restart the watch after ValidateConnect returns.
				gaussdbConn.contextWatcher.Unwatch()

				err := config.ValidateConnect(ctx, gaussdbConn)
				if err != nil {
					if _, ok := err.(*NotPreferredError); ignoreNotPreferredErr && ok {
						return gaussdbConn, nil
					}
					gaussdbConn.conn.Close()
					return nil, newPerDialConnectError("ValidateConnect failed", err)
				}
			}
			return gaussdbConn, nil
		case *gaussdbproto.ParameterStatus, *gaussdbproto.NoticeResponse:
			// handled by ReceiveMessage
		case *gaussdbproto.ErrorResponse:
			gaussdbConn.conn.Close()
			return nil, newPerDialConnectError("server error", ErrorResponseToGuassdbError(msg))
		default:
			gaussdbConn.conn.Close()
			return nil, newPerDialConnectError("received unexpected message", err)
		}
	}
}

func startTLS(conn net.Conn, tlsConfig *tls.Config) (net.Conn, error) {
	err := binary.Write(conn, binary.BigEndian, []int32{8, 80877103})
	if err != nil {
		return nil, err
	}

	response := make([]byte, 1)
	if _, err = io.ReadFull(conn, response); err != nil {
		return nil, err
	}

	if response[0] != 'S' {
		return nil, errors.New("server refused TLS connection")
	}

	return tls.Client(conn, tlsConfig), nil
}

func (gaussdbConn *GaussdbConn) txPasswordMessage(password string) (err error) {
	gaussdbConn.frontend.Send(&gaussdbproto.PasswordMessage{Password: password})
	return gaussdbConn.flushWithPotentialWriteReadDeadlock()
}

func hexMD5(s string) string {
	hash := md5.New()
	io.WriteString(hash, s)
	return hex.EncodeToString(hash.Sum(nil))
}

func (gaussdbConn *GaussdbConn) signalMessage() chan struct{} {
	if gaussdbConn.bufferingReceive {
		panic("BUG: signalMessage when already in progress")
	}

	gaussdbConn.bufferingReceive = true
	gaussdbConn.bufferingReceiveMux.Lock()

	ch := make(chan struct{})
	go func() {
		gaussdbConn.bufferingReceiveMsg, gaussdbConn.bufferingReceiveErr = gaussdbConn.frontend.Receive()
		gaussdbConn.bufferingReceiveMux.Unlock()
		close(ch)
	}()

	return ch
}

// ReceiveMessage receives one wire protocol message from the GaussDB server. It must only be used when the
// connection is not busy. e.g. It is an error to call ReceiveMessage while reading the result of a query. The messages
// are still handled by the core gaussdbconn message handling system so receiving a NotificationResponse will still trigger
// the OnNotification callback.
func (gaussdbConn *GaussdbConn) ReceiveMessage(ctx context.Context) (gaussdbproto.BackendMessage, error) {
	if err := gaussdbConn.lock(); err != nil {
		return nil, err
	}
	defer gaussdbConn.unlock()

	if ctx != context.Background() {
		select {
		case <-ctx.Done():
			return nil, newContextAlreadyDoneError(ctx)
		default:
		}
		gaussdbConn.contextWatcher.Watch(ctx)
		defer gaussdbConn.contextWatcher.Unwatch()
	}

	msg, err := gaussdbConn.receiveMessage()
	if err != nil {
		err = &gaussdbConnError{
			msg:         "receive message failed",
			err:         normalizeTimeoutError(ctx, err),
			safeToRetry: true,
		}
	}
	return msg, err
}

// peekMessage peeks at the next message without setting up context cancellation.
func (gaussdbConn *GaussdbConn) peekMessage() (gaussdbproto.BackendMessage, error) {
	if gaussdbConn.peekedMsg != nil {
		return gaussdbConn.peekedMsg, nil
	}

	var msg gaussdbproto.BackendMessage
	var err error
	if gaussdbConn.bufferingReceive {
		gaussdbConn.bufferingReceiveMux.Lock()
		msg = gaussdbConn.bufferingReceiveMsg
		err = gaussdbConn.bufferingReceiveErr
		gaussdbConn.bufferingReceiveMux.Unlock()
		gaussdbConn.bufferingReceive = false

		// If a timeout error happened in the background try the read again.
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			msg, err = gaussdbConn.frontend.Receive()
		}
	} else {
		msg, err = gaussdbConn.frontend.Receive()
	}

	if err != nil {
		// Close on anything other than timeout error - everything else is fatal
		var netErr net.Error
		isNetErr := errors.As(err, &netErr)
		if !(isNetErr && netErr.Timeout()) {
			gaussdbConn.asyncClose()
		}

		return nil, err
	}

	gaussdbConn.peekedMsg = msg
	return msg, nil
}

// receiveMessage receives a message without setting up context cancellation
func (gaussdbConn *GaussdbConn) receiveMessage() (gaussdbproto.BackendMessage, error) {
	msg, err := gaussdbConn.peekMessage()
	if err != nil {
		return nil, err
	}
	gaussdbConn.peekedMsg = nil

	switch msg := msg.(type) {
	case *gaussdbproto.ReadyForQuery:
		gaussdbConn.txStatus = msg.TxStatus
	case *gaussdbproto.ParameterStatus:
		gaussdbConn.parameterStatuses[msg.Name] = msg.Value
	case *gaussdbproto.ErrorResponse:
		err := ErrorResponseToGuassdbError(msg)
		if gaussdbConn.config.OnGaussdbError != nil && !gaussdbConn.config.OnGaussdbError(gaussdbConn, err) {
			gaussdbConn.status = connStatusClosed
			gaussdbConn.conn.Close() // Ignore error as the connection is already broken and there is already an error to return.
			close(gaussdbConn.cleanupDone)
			return nil, err
		}
	case *gaussdbproto.NoticeResponse:
		if gaussdbConn.config.OnNotice != nil {
			gaussdbConn.config.OnNotice(gaussdbConn, noticeResponseToNotice(msg))
		}
	case *gaussdbproto.NotificationResponse:
		if gaussdbConn.config.OnNotification != nil {
			gaussdbConn.config.OnNotification(gaussdbConn, &Notification{PID: msg.PID, Channel: msg.Channel, Payload: msg.Payload})
		}
	}

	return msg, nil
}

// Conn returns the underlying net.Conn. This rarely necessary. If the connection will be directly used for reading or
// writing then SyncConn should usually be called before Conn.
func (gaussdbConn *GaussdbConn) Conn() net.Conn {
	return gaussdbConn.conn
}

// PID returns the backend PID.
func (gaussdbConn *GaussdbConn) PID() uint32 {
	return gaussdbConn.pid
}

// TxStatus returns the current TxStatus as reported by the server in the ReadyForQuery message.
//
// Possible return values:
//
//	'I' - idle / not in transaction
//	'T' - in a transaction
//	'E' - in a failed transaction
func (gaussdbConn *GaussdbConn) TxStatus() byte {
	return gaussdbConn.txStatus
}

// SecretKey returns the backend secret key used to send a cancel query message to the server.
func (gaussdbConn *GaussdbConn) SecretKey() uint32 {
	return gaussdbConn.secretKey
}

// Frontend returns the underlying *gaussdbproto.Frontend. This rarely necessary.
func (gaussdbConn *GaussdbConn) Frontend() *gaussdbproto.Frontend {
	return gaussdbConn.frontend
}

// Close closes a connection. It is safe to call Close on an already closed connection. Close attempts a clean close by
// sending the exit message to GaussDB. However, this could block so ctx is available to limit the time to wait. The
// underlying net.Conn.Close() will always be called regardless of any other errors.
func (gaussdbConn *GaussdbConn) Close(ctx context.Context) error {
	if gaussdbConn.status == connStatusClosed {
		return nil
	}
	gaussdbConn.status = connStatusClosed

	defer close(gaussdbConn.cleanupDone)
	defer gaussdbConn.conn.Close()

	if ctx != context.Background() {
		// Close may be called while a cancellable query is in progress. This will most often be triggered by panic when
		// a defer closes the connection (possibly indirectly via a transaction or a connection pool). Unwatch to end any
		// previous watch. It is safe to Unwatch regardless of whether a watch is already is progress.
		gaussdbConn.contextWatcher.Unwatch()

		gaussdbConn.contextWatcher.Watch(ctx)
		defer gaussdbConn.contextWatcher.Unwatch()
	}

	// Ignore any errors sending Terminate message and waiting for server to close connection.
	// This mimics the behavior of libpq PQfinish. It calls closePGconn which calls sendTerminateConn which purposefully
	// ignores errors.
	gaussdbConn.frontend.Send(&gaussdbproto.Terminate{})
	gaussdbConn.flushWithPotentialWriteReadDeadlock()

	return gaussdbConn.conn.Close()
}

// asyncClose marks the connection as closed and asynchronously sends a cancel query message and closes the underlying
// connection.
func (gaussdbConn *GaussdbConn) asyncClose() {
	if gaussdbConn.status == connStatusClosed {
		return
	}
	gaussdbConn.status = connStatusClosed

	go func() {
		defer close(gaussdbConn.cleanupDone)
		defer gaussdbConn.conn.Close()

		deadline := time.Now().Add(time.Second * 15)

		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()

		gaussdbConn.CancelRequest(ctx)

		gaussdbConn.conn.SetDeadline(deadline)

		gaussdbConn.frontend.Send(&gaussdbproto.Terminate{})
		gaussdbConn.flushWithPotentialWriteReadDeadlock()
	}()
}

// CleanupDone returns a channel that will be closed after all underlying resources have been cleaned up. A closed
// connection is no longer usable, but underlying resources, in particular the net.Conn, may not have finished closing
// yet. This is because certain errors such as a context cancellation require that the interrupted function call return
// immediately, but the error may also cause the connection to be closed. In these cases the underlying resources are
// closed asynchronously.
//
// This is only likely to be useful to connection pools. It gives them a way avoid establishing a new connection while
// an old connection is still being cleaned up and thereby exceeding the maximum pool size.
func (gaussdbConn *GaussdbConn) CleanupDone() chan (struct{}) {
	return gaussdbConn.cleanupDone
}

// IsClosed reports if the connection has been closed.
//
// CleanupDone() can be used to determine if all cleanup has been completed.
func (gaussdbConn *GaussdbConn) IsClosed() bool {
	return gaussdbConn.status < connStatusIdle
}

// IsBusy reports if the connection is busy.
func (gaussdbConn *GaussdbConn) IsBusy() bool {
	return gaussdbConn.status == connStatusBusy
}

// lock locks the connection.
func (gaussdbConn *GaussdbConn) lock() error {
	switch gaussdbConn.status {
	case connStatusBusy:
		return &connLockError{status: "conn busy"} // This only should be possible in case of an application bug.
	case connStatusClosed:
		return &connLockError{status: "conn closed"}
	case connStatusUninitialized:
		return &connLockError{status: "conn uninitialized"}
	}
	gaussdbConn.status = connStatusBusy
	return nil
}

func (gaussdbConn *GaussdbConn) unlock() {
	switch gaussdbConn.status {
	case connStatusBusy:
		gaussdbConn.status = connStatusIdle
	case connStatusClosed:
	default:
		panic("BUG: cannot unlock unlocked connection") // This should only be possible if there is a bug in this package.
	}
}

// ParameterStatus returns the value of a parameter reported by the server (e.g.
// server_version). Returns an empty string for unknown parameters.
func (gaussdbConn *GaussdbConn) ParameterStatus(key string) string {
	return gaussdbConn.parameterStatuses[key]
}

// CommandTag is the status text returned by GaussDB for a query.
type CommandTag struct {
	s string
}

// NewCommandTag makes a CommandTag from s.
func NewCommandTag(s string) CommandTag {
	return CommandTag{s: s}
}

// RowsAffected returns the number of rows affected. If the CommandTag was not
// for a row affecting command (e.g. "CREATE TABLE") then it returns 0.
func (ct CommandTag) RowsAffected() int64 {
	// Find last non-digit
	idx := -1
	for i := len(ct.s) - 1; i >= 0; i-- {
		if ct.s[i] >= '0' && ct.s[i] <= '9' {
			idx = i
		} else {
			break
		}
	}

	if idx == -1 {
		return 0
	}

	var n int64
	for _, b := range ct.s[idx:] {
		n = n*10 + int64(b-'0')
	}

	return n
}

func (ct CommandTag) String() string {
	return ct.s
}

// Insert is true if the command tag starts with "INSERT".
func (ct CommandTag) Insert() bool {
	return strings.HasPrefix(ct.s, "INSERT")
}

// Update is true if the command tag starts with "UPDATE".
func (ct CommandTag) Update() bool {
	return strings.HasPrefix(ct.s, "UPDATE")
}

// Delete is true if the command tag starts with "DELETE".
func (ct CommandTag) Delete() bool {
	return strings.HasPrefix(ct.s, "DELETE")
}

// Select is true if the command tag starts with "SELECT".
func (ct CommandTag) Select() bool {
	return strings.HasPrefix(ct.s, "SELECT")
}

type FieldDescription struct {
	Name                 string
	TableOID             uint32
	TableAttributeNumber uint16
	DataTypeOID          uint32
	DataTypeSize         int16
	TypeModifier         int32
	Format               int16
}

func (gaussdbConn *GaussdbConn) convertRowDescription(dst []FieldDescription, rd *gaussdbproto.RowDescription) []FieldDescription {
	if cap(dst) >= len(rd.Fields) {
		dst = dst[:len(rd.Fields):len(rd.Fields)]
	} else {
		dst = make([]FieldDescription, len(rd.Fields))
	}

	for i := range rd.Fields {
		dst[i].Name = string(rd.Fields[i].Name)
		dst[i].TableOID = rd.Fields[i].TableOID
		dst[i].TableAttributeNumber = rd.Fields[i].TableAttributeNumber
		dst[i].DataTypeOID = rd.Fields[i].DataTypeOID
		dst[i].DataTypeSize = rd.Fields[i].DataTypeSize
		dst[i].TypeModifier = rd.Fields[i].TypeModifier
		dst[i].Format = rd.Fields[i].Format
	}

	return dst
}

type StatementDescription struct {
	Name      string
	SQL       string
	ParamOIDs []uint32
	Fields    []FieldDescription
}

// Prepare creates a prepared statement. If the name is empty, the anonymous prepared statement will be used. This
// allows Prepare to also to describe statements without creating a server-side prepared statement.
//
// Prepare does not send a PREPARE statement to the server. It uses the GaussDB Parse and Describe protocol messages
// directly.
func (gaussdbConn *GaussdbConn) Prepare(ctx context.Context, name, sql string, paramOIDs []uint32) (*StatementDescription, error) {
	if err := gaussdbConn.lock(); err != nil {
		return nil, err
	}
	defer gaussdbConn.unlock()

	if ctx != context.Background() {
		select {
		case <-ctx.Done():
			return nil, newContextAlreadyDoneError(ctx)
		default:
		}
		gaussdbConn.contextWatcher.Watch(ctx)
		defer gaussdbConn.contextWatcher.Unwatch()
	}

	gaussdbConn.frontend.SendParse(&gaussdbproto.Parse{Name: name, Query: sql, ParameterOIDs: paramOIDs})
	gaussdbConn.frontend.SendDescribe(&gaussdbproto.Describe{ObjectType: 'S', Name: name})
	gaussdbConn.frontend.SendSync(&gaussdbproto.Sync{})
	err := gaussdbConn.flushWithPotentialWriteReadDeadlock()
	if err != nil {
		gaussdbConn.asyncClose()
		return nil, err
	}

	psd := &StatementDescription{Name: name, SQL: sql}

	var parseErr error

readloop:
	for {
		msg, err := gaussdbConn.receiveMessage()
		if err != nil {
			gaussdbConn.asyncClose()
			return nil, normalizeTimeoutError(ctx, err)
		}

		switch msg := msg.(type) {
		case *gaussdbproto.ParameterDescription:
			psd.ParamOIDs = make([]uint32, len(msg.ParameterOIDs))
			copy(psd.ParamOIDs, msg.ParameterOIDs)
		case *gaussdbproto.RowDescription:
			psd.Fields = gaussdbConn.convertRowDescription(nil, msg)
		case *gaussdbproto.ErrorResponse:
			parseErr = ErrorResponseToGuassdbError(msg)
		case *gaussdbproto.ReadyForQuery:
			break readloop
		}
	}

	if parseErr != nil {
		return nil, parseErr
	}
	return psd, nil
}

// Deallocate deallocates a prepared statement.
//
// Deallocate does not send a DEALLOCATE statement to the server. It uses the GaussDB Close protocol message
// directly. This has slightly different behavior than executing DEALLOCATE statement.
//   - Deallocate can succeed in an aborted transaction.
//   - Deallocating a non-existent prepared statement is not an error.
func (gaussdbConn *GaussdbConn) Deallocate(ctx context.Context, name string) error {
	if err := gaussdbConn.lock(); err != nil {
		return err
	}
	defer gaussdbConn.unlock()

	if ctx != context.Background() {
		select {
		case <-ctx.Done():
			return newContextAlreadyDoneError(ctx)
		default:
		}
		gaussdbConn.contextWatcher.Watch(ctx)
		defer gaussdbConn.contextWatcher.Unwatch()
	}

	gaussdbConn.frontend.SendClose(&gaussdbproto.Close{ObjectType: 'S', Name: name})
	gaussdbConn.frontend.SendSync(&gaussdbproto.Sync{})
	err := gaussdbConn.flushWithPotentialWriteReadDeadlock()
	if err != nil {
		gaussdbConn.asyncClose()
		return err
	}

	for {
		msg, err := gaussdbConn.receiveMessage()
		if err != nil {
			gaussdbConn.asyncClose()
			return normalizeTimeoutError(ctx, err)
		}

		switch msg := msg.(type) {
		case *gaussdbproto.ErrorResponse:
			return ErrorResponseToGuassdbError(msg)
		case *gaussdbproto.ReadyForQuery:
			return nil
		}
	}
}

// ErrorResponseToGuassdbError converts a wire protocol error message to a *GaussdbError.
func ErrorResponseToGuassdbError(msg *gaussdbproto.ErrorResponse) *GaussdbError {
	return &GaussdbError{
		Severity:            msg.Severity,
		SeverityUnlocalized: msg.SeverityUnlocalized,
		Code:                string(msg.Code),
		Message:             string(msg.Message),
		Detail:              string(msg.Detail),
		Hint:                msg.Hint,
		Position:            msg.Position,
		InternalPosition:    msg.InternalPosition,
		InternalQuery:       string(msg.InternalQuery),
		Where:               string(msg.Where),
		SchemaName:          string(msg.SchemaName),
		TableName:           string(msg.TableName),
		ColumnName:          string(msg.ColumnName),
		DataTypeName:        string(msg.DataTypeName),
		ConstraintName:      msg.ConstraintName,
		File:                string(msg.File),
		Line:                msg.Line,
		Routine:             string(msg.Routine),
	}
}

func noticeResponseToNotice(msg *gaussdbproto.NoticeResponse) *Notice {
	gaussdbError := ErrorResponseToGuassdbError((*gaussdbproto.ErrorResponse)(msg))
	return (*Notice)(gaussdbError)
}

// CancelRequest sends a cancel request to the GaussDB server. It returns an error if unable to deliver the cancel
// request, but lack of an error does not ensure that the query was canceled. As specified in the documentation, there
// is no way to be sure a query was canceled.
func (gaussdbConn *GaussdbConn) CancelRequest(ctx context.Context) error {
	// Open a cancellation request to the same server. The address is taken from the net.Conn directly instead of reusing
	// the connection config. This is important in high availability configurations where fallback connections may be
	// specified or DNS may be used to load balance.
	serverAddr := gaussdbConn.conn.RemoteAddr()
	var serverNetwork string
	var serverAddress string
	if serverAddr.Network() == "unix" {
		// for unix sockets, RemoteAddr() calls getpeername() which returns the name the
		// server passed to bind(). For Gaussdb, this is always a relative path "./.s.PGSQL.5432"
		// so connecting to it will fail. Fall back to the config's value
		serverNetwork, serverAddress = NetworkAddress(gaussdbConn.config.Host, gaussdbConn.config.Port)
	} else {
		serverNetwork, serverAddress = serverAddr.Network(), serverAddr.String()
	}
	cancelConn, err := gaussdbConn.config.DialFunc(ctx, serverNetwork, serverAddress)
	if err != nil {
		// In case of unix sockets, RemoteAddr() returns only the file part of the path. If the
		// first connect failed, try the config.
		if serverAddr.Network() != "unix" {
			return err
		}
		serverNetwork, serverAddr := NetworkAddress(gaussdbConn.config.Host, gaussdbConn.config.Port)
		cancelConn, err = gaussdbConn.config.DialFunc(ctx, serverNetwork, serverAddr)
		if err != nil {
			return err
		}
	}
	defer cancelConn.Close()

	if ctx != context.Background() {
		contextWatcher := ctxwatch.NewContextWatcher(&DeadlineContextWatcherHandler{Conn: cancelConn})
		contextWatcher.Watch(ctx)
		defer contextWatcher.Unwatch()
	}

	buf := make([]byte, 16)
	binary.BigEndian.PutUint32(buf[0:4], 16)
	binary.BigEndian.PutUint32(buf[4:8], 80877102)
	binary.BigEndian.PutUint32(buf[8:12], gaussdbConn.pid)
	binary.BigEndian.PutUint32(buf[12:16], gaussdbConn.secretKey)

	if _, err := cancelConn.Write(buf); err != nil {
		return fmt.Errorf("write to connection for cancellation: %w", err)
	}

	// Wait for the cancel request to be acknowledged by the server.
	_, _ = cancelConn.Read(buf)

	return nil
}

// WaitForNotification waits for a LISTEN/NOTIFY message to be received. It returns an error if a notification was not
// received.
func (gaussdbConn *GaussdbConn) WaitForNotification(ctx context.Context) error {
	if err := gaussdbConn.lock(); err != nil {
		return err
	}
	defer gaussdbConn.unlock()

	if ctx != context.Background() {
		select {
		case <-ctx.Done():
			return newContextAlreadyDoneError(ctx)
		default:
		}

		gaussdbConn.contextWatcher.Watch(ctx)
		defer gaussdbConn.contextWatcher.Unwatch()
	}

	for {
		msg, err := gaussdbConn.receiveMessage()
		if err != nil {
			return normalizeTimeoutError(ctx, err)
		}

		switch msg.(type) {
		case *gaussdbproto.NotificationResponse:
			return nil
		}
	}
}

// Exec executes SQL via the GaussDB simple query protocol. SQL may contain multiple queries. Execution is
// implicitly wrapped in a transaction unless a transaction is already in progress or SQL contains transaction control
// statements.
//
// Prefer ExecParams unless executing arbitrary SQL that may contain multiple queries.
func (gaussdbConn *GaussdbConn) Exec(ctx context.Context, sql string) *MultiResultReader {
	if err := gaussdbConn.lock(); err != nil {
		return &MultiResultReader{
			closed: true,
			err:    err,
		}
	}

	gaussdbConn.multiResultReader = MultiResultReader{
		gaussdbConn: gaussdbConn,
		ctx:         ctx,
	}
	multiResult := &gaussdbConn.multiResultReader
	if ctx != context.Background() {
		select {
		case <-ctx.Done():
			multiResult.closed = true
			multiResult.err = newContextAlreadyDoneError(ctx)
			gaussdbConn.unlock()
			return multiResult
		default:
		}
		gaussdbConn.contextWatcher.Watch(ctx)
	}

	gaussdbConn.frontend.SendQuery(&gaussdbproto.Query{String: sql})
	err := gaussdbConn.flushWithPotentialWriteReadDeadlock()
	if err != nil {
		gaussdbConn.asyncClose()
		gaussdbConn.contextWatcher.Unwatch()
		multiResult.closed = true
		multiResult.err = err
		gaussdbConn.unlock()
		return multiResult
	}

	return multiResult
}

// ExecParams executes a command via the GaussDB extended query protocol.
//
// sql is a SQL command string. It may only contain one query. Parameter substitution is positional using $1, $2, $3,
// etc.
//
// paramValues are the parameter values. It must be encoded in the format given by paramFormats.
//
// paramOIDs is a slice of data type OIDs for paramValues. If paramOIDs is nil, the server will infer the data type for
// all parameters. Any paramOID element that is 0 that will cause the server to infer the data type for that parameter.
// ExecParams will panic if len(paramOIDs) is not 0, 1, or len(paramValues).
//
// paramFormats is a slice of format codes determining for each paramValue column whether it is encoded in text or
// binary format. If paramFormats is nil all params are text format. ExecParams will panic if
// len(paramFormats) is not 0, 1, or len(paramValues).
//
// resultFormats is a slice of format codes determining for each result column whether it is encoded in text or
// binary format. If resultFormats is nil all results will be in text format.
//
// ResultReader must be closed before GaussdbConn can be used again.
func (gaussdbConn *GaussdbConn) ExecParams(ctx context.Context, sql string, paramValues [][]byte, paramOIDs []uint32, paramFormats []int16, resultFormats []int16) *ResultReader {
	result := gaussdbConn.execExtendedPrefix(ctx, paramValues)
	if result.closed {
		return result
	}

	gaussdbConn.frontend.SendParse(&gaussdbproto.Parse{Query: sql, ParameterOIDs: paramOIDs})
	gaussdbConn.frontend.SendBind(&gaussdbproto.Bind{ParameterFormatCodes: paramFormats, Parameters: paramValues, ResultFormatCodes: resultFormats})

	gaussdbConn.execExtendedSuffix(result)

	return result
}

// ExecPrepared enqueues the execution of a prepared statement via the GaussDB extended query protocol.
//
// paramValues are the parameter values. It must be encoded in the format given by paramFormats.
//
// paramFormats is a slice of format codes determining for each paramValue column whether it is encoded in text or
// binary format. If paramFormats is nil all params are text format. ExecPrepared will panic if
// len(paramFormats) is not 0, 1, or len(paramValues).
//
// resultFormats is a slice of format codes determining for each result column whether it is encoded in text or
// binary format. If resultFormats is nil all results will be in text format.
//
// ResultReader must be closed before GaussdbConn can be used again.
func (gaussdbConn *GaussdbConn) ExecPrepared(ctx context.Context, stmtName string, paramValues [][]byte, paramFormats []int16, resultFormats []int16) *ResultReader {
	result := gaussdbConn.execExtendedPrefix(ctx, paramValues)
	if result.closed {
		return result
	}

	gaussdbConn.frontend.SendBind(&gaussdbproto.Bind{PreparedStatement: stmtName, ParameterFormatCodes: paramFormats, Parameters: paramValues, ResultFormatCodes: resultFormats})

	gaussdbConn.execExtendedSuffix(result)

	return result
}

func (gaussdbConn *GaussdbConn) execExtendedPrefix(ctx context.Context, paramValues [][]byte) *ResultReader {
	gaussdbConn.resultReader = ResultReader{
		gaussdbConn: gaussdbConn,
		ctx:         ctx,
	}
	result := &gaussdbConn.resultReader

	if err := gaussdbConn.lock(); err != nil {
		result.concludeCommand(CommandTag{}, err)
		result.closed = true
		return result
	}

	if len(paramValues) > math.MaxUint16 {
		result.concludeCommand(CommandTag{}, fmt.Errorf("extended protocol limited to %v parameters", math.MaxUint16))
		result.closed = true
		gaussdbConn.unlock()
		return result
	}

	if ctx != context.Background() {
		select {
		case <-ctx.Done():
			result.concludeCommand(CommandTag{}, newContextAlreadyDoneError(ctx))
			result.closed = true
			gaussdbConn.unlock()
			return result
		default:
		}
		gaussdbConn.contextWatcher.Watch(ctx)
	}

	return result
}

func (gaussdbConn *GaussdbConn) execExtendedSuffix(result *ResultReader) {
	gaussdbConn.frontend.SendDescribe(&gaussdbproto.Describe{ObjectType: 'P'})
	gaussdbConn.frontend.SendExecute(&gaussdbproto.Execute{})
	gaussdbConn.frontend.SendSync(&gaussdbproto.Sync{})

	err := gaussdbConn.flushWithPotentialWriteReadDeadlock()
	if err != nil {
		gaussdbConn.asyncClose()
		result.concludeCommand(CommandTag{}, err)
		gaussdbConn.contextWatcher.Unwatch()
		result.closed = true
		gaussdbConn.unlock()
		return
	}

	result.readUntilRowDescription()
}

// CopyTo executes the copy command sql and copies the results to w.
func (gaussdbConn *GaussdbConn) CopyTo(ctx context.Context, w io.Writer, sql string) (CommandTag, error) {
	if err := gaussdbConn.lock(); err != nil {
		return CommandTag{}, err
	}

	if ctx != context.Background() {
		select {
		case <-ctx.Done():
			gaussdbConn.unlock()
			return CommandTag{}, newContextAlreadyDoneError(ctx)
		default:
		}
		gaussdbConn.contextWatcher.Watch(ctx)
		defer gaussdbConn.contextWatcher.Unwatch()
	}

	// Send copy to command
	gaussdbConn.frontend.SendQuery(&gaussdbproto.Query{String: sql})

	err := gaussdbConn.flushWithPotentialWriteReadDeadlock()
	if err != nil {
		gaussdbConn.asyncClose()
		gaussdbConn.unlock()
		return CommandTag{}, err
	}

	// Read results
	var commandTag CommandTag
	var gaussdbErr error
	for {
		msg, err := gaussdbConn.receiveMessage()
		if err != nil {
			gaussdbConn.asyncClose()
			return CommandTag{}, normalizeTimeoutError(ctx, err)
		}

		switch msg := msg.(type) {
		case *gaussdbproto.CopyDone:
		case *gaussdbproto.CopyData:
			_, err := w.Write(msg.Data)
			if err != nil {
				gaussdbConn.asyncClose()
				return CommandTag{}, err
			}
		case *gaussdbproto.ReadyForQuery:
			gaussdbConn.unlock()
			return commandTag, gaussdbErr
		case *gaussdbproto.CommandComplete:
			commandTag = gaussdbConn.makeCommandTag(msg.CommandTag)
		case *gaussdbproto.ErrorResponse:
			gaussdbErr = ErrorResponseToGuassdbError(msg)
		}
	}
}

// CopyFrom executes the copy command sql and copies all of r to the GaussDB server.
//
// Note: context cancellation will only interrupt operations on the underlying GaussDB network connection. Reads on r
// could still block.
func (gaussdbConn *GaussdbConn) CopyFrom(ctx context.Context, r io.Reader, sql string) (CommandTag, error) {
	if err := gaussdbConn.lock(); err != nil {
		return CommandTag{}, err
	}
	defer gaussdbConn.unlock()

	if ctx != context.Background() {
		select {
		case <-ctx.Done():
			return CommandTag{}, newContextAlreadyDoneError(ctx)
		default:
		}
		gaussdbConn.contextWatcher.Watch(ctx)
		defer gaussdbConn.contextWatcher.Unwatch()
	}

	// Send copy from query
	gaussdbConn.frontend.SendQuery(&gaussdbproto.Query{String: sql})
	err := gaussdbConn.flushWithPotentialWriteReadDeadlock()
	if err != nil {
		gaussdbConn.asyncClose()
		return CommandTag{}, err
	}

	// Send copy data
	abortCopyChan := make(chan struct{})
	copyErrChan := make(chan error, 1)
	signalMessageChan := gaussdbConn.signalMessage()
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		buf := iobufpool.Get(65536)
		defer iobufpool.Put(buf)
		(*buf)[0] = 'd'

		for {
			n, readErr := r.Read((*buf)[5:cap(*buf)])
			if n > 0 {
				*buf = (*buf)[0 : n+5]
				gaussdbio.SetInt32((*buf)[1:], int32(n+4))

				writeErr := gaussdbConn.frontend.SendUnbufferedEncodedCopyData(*buf)
				if writeErr != nil {
					// Write errors are always fatal, but we can't use asyncClose because we are in a different goroutine. Not
					// setting gaussdbConn.status or closing gaussdbConn.cleanupDone for the same reason.
					gaussdbConn.conn.Close()

					copyErrChan <- writeErr
					return
				}
			}
			if readErr != nil {
				copyErrChan <- readErr
				return
			}

			select {
			case <-abortCopyChan:
				return
			default:
			}
		}
	}()

	var gaussdbErr error
	var copyErr error
	for copyErr == nil && gaussdbErr == nil {
		select {
		case copyErr = <-copyErrChan:
		case <-signalMessageChan:
			// If gaussdbConn.receiveMessage encounters an error it will call gaussdbConn.asyncClose. But that is a race condition with
			// the goroutine. So instead check gaussdbConn.bufferingReceiveErr which will have been set by the signalMessage. If an
			// error is found then forcibly close the connection without sending the Terminate message.
			if err := gaussdbConn.bufferingReceiveErr; err != nil {
				gaussdbConn.status = connStatusClosed
				gaussdbConn.conn.Close()
				close(gaussdbConn.cleanupDone)
				return CommandTag{}, normalizeTimeoutError(ctx, err)
			}
			msg, _ := gaussdbConn.receiveMessage()

			switch msg := msg.(type) {
			case *gaussdbproto.ErrorResponse:
				gaussdbErr = ErrorResponseToGuassdbError(msg)
			default:
				signalMessageChan = gaussdbConn.signalMessage()
			}
		}
	}
	close(abortCopyChan)
	// Make sure io goroutine finishes before writing.
	wg.Wait()

	if copyErr == io.EOF || gaussdbErr != nil {
		gaussdbConn.frontend.Send(&gaussdbproto.CopyDone{})
	} else {
		gaussdbConn.frontend.Send(&gaussdbproto.CopyFail{Message: copyErr.Error()})
	}
	err = gaussdbConn.flushWithPotentialWriteReadDeadlock()
	if err != nil {
		gaussdbConn.asyncClose()
		return CommandTag{}, err
	}

	// Read results
	var commandTag CommandTag
	for {
		msg, err := gaussdbConn.receiveMessage()
		if err != nil {
			gaussdbConn.asyncClose()
			return CommandTag{}, normalizeTimeoutError(ctx, err)
		}

		switch msg := msg.(type) {
		case *gaussdbproto.ReadyForQuery:
			return commandTag, gaussdbErr
		case *gaussdbproto.CommandComplete:
			commandTag = gaussdbConn.makeCommandTag(msg.CommandTag)
		case *gaussdbproto.ErrorResponse:
			gaussdbErr = ErrorResponseToGuassdbError(msg)
		}
	}
}

// MultiResultReader is a reader for a command that could return multiple results such as Exec or ExecBatch.
type MultiResultReader struct {
	gaussdbConn *GaussdbConn
	ctx         context.Context

	rr *ResultReader

	closed bool
	err    error
}

// ReadAll reads all available results. Calling ReadAll is mutually exclusive with all other MultiResultReader methods.
func (mrr *MultiResultReader) ReadAll() ([]*Result, error) {
	var results []*Result

	for mrr.NextResult() {
		results = append(results, mrr.ResultReader().Read())
	}
	err := mrr.Close()

	return results, err
}

func (mrr *MultiResultReader) receiveMessage() (gaussdbproto.BackendMessage, error) {
	msg, err := mrr.gaussdbConn.receiveMessage()
	if err != nil {
		mrr.gaussdbConn.contextWatcher.Unwatch()
		mrr.err = normalizeTimeoutError(mrr.ctx, err)
		mrr.closed = true
		mrr.gaussdbConn.asyncClose()
		return nil, mrr.err
	}

	switch msg := msg.(type) {
	case *gaussdbproto.ReadyForQuery:
		mrr.closed = true
		mrr.gaussdbConn.contextWatcher.Unwatch()
		mrr.gaussdbConn.unlock()
	case *gaussdbproto.ErrorResponse:
		mrr.err = ErrorResponseToGuassdbError(msg)
	}

	return msg, nil
}

// NextResult returns advances the MultiResultReader to the next result and returns true if a result is available.
func (mrr *MultiResultReader) NextResult() bool {
	for !mrr.closed && mrr.err == nil {
		msg, err := mrr.receiveMessage()
		if err != nil {
			return false
		}

		switch msg := msg.(type) {
		case *gaussdbproto.RowDescription:
			mrr.gaussdbConn.resultReader = ResultReader{
				gaussdbConn:       mrr.gaussdbConn,
				multiResultReader: mrr,
				ctx:               mrr.ctx,
				fieldDescriptions: mrr.gaussdbConn.convertRowDescription(mrr.gaussdbConn.fieldDescriptions[:], msg),
			}

			mrr.rr = &mrr.gaussdbConn.resultReader
			return true
		case *gaussdbproto.CommandComplete:
			mrr.gaussdbConn.resultReader = ResultReader{
				commandTag:       mrr.gaussdbConn.makeCommandTag(msg.CommandTag),
				commandConcluded: true,
				closed:           true,
			}
			mrr.rr = &mrr.gaussdbConn.resultReader
			return true
		case *gaussdbproto.EmptyQueryResponse:
			return false
		}
	}

	return false
}

// ResultReader returns the current ResultReader.
func (mrr *MultiResultReader) ResultReader() *ResultReader {
	return mrr.rr
}

// Close closes the MultiResultReader and returns the first error that occurred during the MultiResultReader's use.
func (mrr *MultiResultReader) Close() error {
	for !mrr.closed {
		_, err := mrr.receiveMessage()
		if err != nil {
			return mrr.err
		}
	}

	return mrr.err
}

// ResultReader is a reader for the result of a single query.
type ResultReader struct {
	gaussdbConn       *GaussdbConn
	multiResultReader *MultiResultReader
	pipeline          *Pipeline
	ctx               context.Context

	fieldDescriptions []FieldDescription
	rowValues         [][]byte
	commandTag        CommandTag
	commandConcluded  bool
	closed            bool
	err               error
}

// Result is the saved query response that is returned by calling Read on a ResultReader.
type Result struct {
	FieldDescriptions []FieldDescription
	Rows              [][][]byte
	CommandTag        CommandTag
	Err               error
}

// Read saves the query response to a Result.
func (rr *ResultReader) Read() *Result {
	br := &Result{}

	for rr.NextRow() {
		if br.FieldDescriptions == nil {
			br.FieldDescriptions = make([]FieldDescription, len(rr.FieldDescriptions()))
			copy(br.FieldDescriptions, rr.FieldDescriptions())
		}

		values := rr.Values()
		row := make([][]byte, len(values))
		for i := range row {
			if values[i] != nil {
				row[i] = make([]byte, len(values[i]))
				copy(row[i], values[i])
			}
		}
		br.Rows = append(br.Rows, row)
	}

	br.CommandTag, br.Err = rr.Close()

	return br
}

// NextRow advances the ResultReader to the next row and returns true if a row is available.
func (rr *ResultReader) NextRow() bool {
	for !rr.commandConcluded {
		msg, err := rr.receiveMessage()
		if err != nil {
			return false
		}

		switch msg := msg.(type) {
		case *gaussdbproto.DataRow:
			rr.rowValues = msg.Values
			return true
		}
	}

	return false
}

// FieldDescriptions returns the field descriptions for the current result set. The returned slice is only valid until
// the ResultReader is closed. It may return nil (for example, if the query did not return a result set or an error was
// encountered.)
func (rr *ResultReader) FieldDescriptions() []FieldDescription {
	return rr.fieldDescriptions
}

// Values returns the current row data. NextRow must have been previously been called. The returned [][]byte is only
// valid until the next NextRow call or the ResultReader is closed.
func (rr *ResultReader) Values() [][]byte {
	return rr.rowValues
}

// Close consumes any remaining result data and returns the command tag or
// error.
func (rr *ResultReader) Close() (CommandTag, error) {
	if rr.closed {
		return rr.commandTag, rr.err
	}
	rr.closed = true

	for !rr.commandConcluded {
		_, err := rr.receiveMessage()
		if err != nil {
			return CommandTag{}, rr.err
		}
	}

	if rr.multiResultReader == nil && rr.pipeline == nil {
		for {
			msg, err := rr.receiveMessage()
			if err != nil {
				return CommandTag{}, rr.err
			}

			switch msg := msg.(type) {
			// Detect a deferred constraint violation where the ErrorResponse is sent after CommandComplete.
			case *gaussdbproto.ErrorResponse:
				rr.err = ErrorResponseToGuassdbError(msg)
			case *gaussdbproto.ReadyForQuery:
				rr.gaussdbConn.contextWatcher.Unwatch()
				rr.gaussdbConn.unlock()
				return rr.commandTag, rr.err
			}
		}
	}

	return rr.commandTag, rr.err
}

// readUntilRowDescription ensures the ResultReader's fieldDescriptions are loaded. It does not return an error as any
// error will be stored in the ResultReader.
func (rr *ResultReader) readUntilRowDescription() {
	for !rr.commandConcluded {
		// Peek before receive to avoid consuming a DataRow if the result set does not include a RowDescription method.
		// This should never happen under normal gaussdbconn usage, but it is possible if SendBytes and ReceiveResults are
		// manually used to construct a query that does not issue a describe statement.
		msg, _ := rr.gaussdbConn.peekMessage()
		if _, ok := msg.(*gaussdbproto.DataRow); ok {
			return
		}

		// Consume the message
		msg, _ = rr.receiveMessage()
		if _, ok := msg.(*gaussdbproto.RowDescription); ok {
			return
		}
	}
}

func (rr *ResultReader) receiveMessage() (msg gaussdbproto.BackendMessage, err error) {
	if rr.multiResultReader == nil {
		msg, err = rr.gaussdbConn.receiveMessage()
	} else {
		msg, err = rr.multiResultReader.receiveMessage()
	}

	if err != nil {
		err = normalizeTimeoutError(rr.ctx, err)
		rr.concludeCommand(CommandTag{}, err)
		rr.gaussdbConn.contextWatcher.Unwatch()
		rr.closed = true
		if rr.multiResultReader == nil {
			rr.gaussdbConn.asyncClose()
		}

		return nil, rr.err
	}

	switch msg := msg.(type) {
	case *gaussdbproto.RowDescription:
		rr.fieldDescriptions = rr.gaussdbConn.convertRowDescription(rr.gaussdbConn.fieldDescriptions[:], msg)
	case *gaussdbproto.CommandComplete:
		rr.concludeCommand(rr.gaussdbConn.makeCommandTag(msg.CommandTag), nil)
	case *gaussdbproto.EmptyQueryResponse:
		rr.concludeCommand(CommandTag{}, nil)
	case *gaussdbproto.ErrorResponse:
		gaussdbError := ErrorResponseToGuassdbError(msg)
		if rr.pipeline != nil {
			rr.pipeline.state.HandleError(gaussdbError)
		}
		rr.concludeCommand(CommandTag{}, gaussdbError)
	}

	return msg, nil
}

func (rr *ResultReader) concludeCommand(commandTag CommandTag, err error) {
	// Keep the first error that is recorded. Store the error before checking if the command is already concluded to
	// allow for receiving an error after CommandComplete but before ReadyForQuery.
	if err != nil && rr.err == nil {
		rr.err = err
	}

	if rr.commandConcluded {
		return
	}

	rr.commandTag = commandTag
	rr.rowValues = nil
	rr.commandConcluded = true
}

// Batch is a collection of queries that can be sent to the GaussDB server in a single round-trip.
type Batch struct {
	buf []byte
	err error
}

// ExecParams appends an ExecParams command to the batch. See GaussdbConn.ExecParams for parameter descriptions.
func (batch *Batch) ExecParams(sql string, paramValues [][]byte, paramOIDs []uint32, paramFormats []int16, resultFormats []int16) {
	if batch.err != nil {
		return
	}

	batch.buf, batch.err = (&gaussdbproto.Parse{Query: sql, ParameterOIDs: paramOIDs}).Encode(batch.buf)
	if batch.err != nil {
		return
	}
	batch.ExecPrepared("", paramValues, paramFormats, resultFormats)
}

// ExecPrepared appends an ExecPrepared e command to the batch. See GaussdbConn.ExecPrepared for parameter descriptions.
func (batch *Batch) ExecPrepared(stmtName string, paramValues [][]byte, paramFormats []int16, resultFormats []int16) {
	if batch.err != nil {
		return
	}

	batch.buf, batch.err = (&gaussdbproto.Bind{PreparedStatement: stmtName, ParameterFormatCodes: paramFormats, Parameters: paramValues, ResultFormatCodes: resultFormats}).Encode(batch.buf)
	if batch.err != nil {
		return
	}

	batch.buf, batch.err = (&gaussdbproto.Describe{ObjectType: 'P'}).Encode(batch.buf)
	if batch.err != nil {
		return
	}

	batch.buf, batch.err = (&gaussdbproto.Execute{}).Encode(batch.buf)
	if batch.err != nil {
		return
	}
}

// ExecBatch executes all the queries in batch in a single round-trip. Execution is implicitly transactional unless a
// transaction is already in progress or SQL contains transaction control statements. This is a simpler way of executing
// multiple queries in a single round trip than using pipeline mode.
func (gaussdbConn *GaussdbConn) ExecBatch(ctx context.Context, batch *Batch) *MultiResultReader {
	if batch.err != nil {
		return &MultiResultReader{
			closed: true,
			err:    batch.err,
		}
	}

	if err := gaussdbConn.lock(); err != nil {
		return &MultiResultReader{
			closed: true,
			err:    err,
		}
	}

	gaussdbConn.multiResultReader = MultiResultReader{
		gaussdbConn: gaussdbConn,
		ctx:         ctx,
	}
	multiResult := &gaussdbConn.multiResultReader

	if ctx != context.Background() {
		select {
		case <-ctx.Done():
			multiResult.closed = true
			multiResult.err = newContextAlreadyDoneError(ctx)
			gaussdbConn.unlock()
			return multiResult
		default:
		}
		gaussdbConn.contextWatcher.Watch(ctx)
	}

	batch.buf, batch.err = (&gaussdbproto.Sync{}).Encode(batch.buf)
	if batch.err != nil {
		gaussdbConn.contextWatcher.Unwatch()
		multiResult.err = normalizeTimeoutError(multiResult.ctx, batch.err)
		multiResult.closed = true
		gaussdbConn.asyncClose()
		return multiResult
	}

	gaussdbConn.enterPotentialWriteReadDeadlock()
	defer gaussdbConn.exitPotentialWriteReadDeadlock()
	_, err := gaussdbConn.conn.Write(batch.buf)
	if err != nil {
		gaussdbConn.contextWatcher.Unwatch()
		multiResult.err = normalizeTimeoutError(multiResult.ctx, err)
		multiResult.closed = true
		gaussdbConn.asyncClose()
		return multiResult
	}

	return multiResult
}

// EscapeString escapes a string such that it can safely be interpolated into a SQL command string. It does not include
// the surrounding single quotes.
//
// The current implementation requires that standard_conforming_strings=on and client_encoding="UTF8". If these
// conditions are not met an error will be returned. It is possible these restrictions will be lifted in the future.
func (gaussdbConn *GaussdbConn) EscapeString(s string) (string, error) {
	if gaussdbConn.ParameterStatus("standard_conforming_strings") != "on" {
		return "", errors.New("EscapeString must be run with standard_conforming_strings=on")
	}

	if gaussdbConn.ParameterStatus("client_encoding") != "UTF8" {
		return "", errors.New("EscapeString must be run with client_encoding=UTF8")
	}

	return strings.Replace(s, "'", "''", -1), nil
}

// CheckConn checks the underlying connection without writing any bytes. This is currently implemented by doing a read
// with a very short deadline. This can be useful because a TCP connection can be broken such that a write will appear
// to succeed even though it will never actually reach the server. Reading immediately before a write will detect this
// condition. If this is done immediately before sending a query it reduces the chances a query will be sent that fails
// without the client knowing whether the server received it or not.
//
// Deprecated: CheckConn is deprecated in favor of Ping. CheckConn cannot detect all types of broken connections where
// the write would still appear to succeed. Prefer Ping unless on a high latency connection.
func (gaussdbConn *GaussdbConn) CheckConn() error {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	_, err := gaussdbConn.ReceiveMessage(ctx)
	if err != nil {
		if !Timeout(err) {
			return err
		}
	}

	return nil
}

// Ping pings the server. This can be useful because a TCP connection can be broken such that a write will appear to
// succeed even though it will never actually reach the server. Pinging immediately before sending a query reduces the
// chances a query will be sent that fails without the client knowing whether the server received it or not.
func (gaussdbConn *GaussdbConn) Ping(ctx context.Context) error {
	return gaussdbConn.Exec(ctx, "-- ping").Close()
}

// makeCommandTag makes a CommandTag. It does not retain a reference to buf or buf's underlying memory.
func (gaussdbConn *GaussdbConn) makeCommandTag(buf []byte) CommandTag {
	return CommandTag{s: string(buf)}
}

// enterPotentialWriteReadDeadlock must be called before a write that could deadlock if the server is simultaneously
// blocked writing to us.
func (gaussdbConn *GaussdbConn) enterPotentialWriteReadDeadlock() {
	// The time to wait is somewhat arbitrary. A Write should only take as long as the syscall and memcpy to the OS
	// outbound network buffer unless the buffer is full (which potentially is a block). It needs to be long enough for
	// the normal case, but short enough not to kill performance if a block occurs.
	//
	// In addition, on Windows the default timer resolution is 15.6ms. So setting the timer to less than that is
	// ineffective.
	if gaussdbConn.slowWriteTimer.Reset(15 * time.Millisecond) {
		panic("BUG: slow write timer already active")
	}
}

// exitPotentialWriteReadDeadlock must be called after a call to enterPotentialWriteReadDeadlock.
func (gaussdbConn *GaussdbConn) exitPotentialWriteReadDeadlock() {
	if !gaussdbConn.slowWriteTimer.Stop() {
		// The timer starts its function in a separate goroutine. It is necessary to ensure the background reader has
		// started before calling Stop. Otherwise, the background reader may not be stopped. That on its own is not a
		// serious problem. But what is a serious problem is that the background reader may start at an inopportune time in
		// a subsequent query. For example, if a subsequent query was canceled then a deadline may be set on the net.Conn to
		// interrupt an in-progress read. After the read is interrupted, but before the deadline is cleared, the background
		// reader could start and read a deadline error. Then the next query would receive the an unexpected deadline error.
		<-gaussdbConn.bgReaderStarted
		gaussdbConn.bgReader.Stop()
	}
}

func (gaussdbConn *GaussdbConn) flushWithPotentialWriteReadDeadlock() error {
	gaussdbConn.enterPotentialWriteReadDeadlock()
	defer gaussdbConn.exitPotentialWriteReadDeadlock()
	err := gaussdbConn.frontend.Flush()
	return err
}

// SyncConn prepares the underlying net.Conn for direct use. GaussdbConn may internally buffer reads or use goroutines for
// background IO. This means that any direct use of the underlying net.Conn may be corrupted if a read is already
// buffered or a read is in progress. SyncConn drains read buffers and stops background IO. In some cases this may
// require sending a ping to the server. ctx can be used to cancel this operation. This should be called before any
// operation that will use the underlying net.Conn directly. e.g. Before Conn() or Hijack().
//
// This should not be confused with the GaussDB protocol Sync message.
func (gaussdbConn *GaussdbConn) SyncConn(ctx context.Context) error {
	for i := 0; i < 10; i++ {
		if gaussdbConn.bgReader.Status() == bgreader.StatusStopped && gaussdbConn.frontend.ReadBufferLen() == 0 {
			return nil
		}

		err := gaussdbConn.Ping(ctx)
		if err != nil {
			return fmt.Errorf("SyncConn: Ping failed while syncing conn: %w", err)
		}
	}

	// This should never happen. Only way I can imagine this occurring is if the server is constantly sending data such as
	// LISTEN/NOTIFY or log notifications such that we never can get an empty buffer.
	return errors.New("SyncConn: conn never synchronized")
}

// CustomData returns a map that can be used to associate custom data with the connection.
func (gaussdbConn *GaussdbConn) CustomData() map[string]any {
	return gaussdbConn.customData
}

// HijackedConn is the result of hijacking a connection.
//
// Due to the necessary exposure of internal implementation details, it is not covered by the semantic versioning
// compatibility.
type HijackedConn struct {
	Conn              net.Conn
	PID               uint32            // backend pid
	SecretKey         uint32            // key to use to send a cancel query message to the server
	ParameterStatuses map[string]string // parameters that have been reported by the server
	TxStatus          byte
	Frontend          *gaussdbproto.Frontend
	Config            *Config
	CustomData        map[string]any
}

// Hijack extracts the internal connection data. gaussdbConn must be in an idle state. SyncConn should be called immediately
// before Hijack. gaussdbConn is unusable after hijacking. Hijacking is typically only useful when using gaussdbconn to establish
// a connection, but taking complete control of the raw connection after that (e.g. a load balancer or proxy).
//
// Due to the necessary exposure of internal implementation details, it is not covered by the semantic versioning
// compatibility.
func (gaussdbConn *GaussdbConn) Hijack() (*HijackedConn, error) {
	if err := gaussdbConn.lock(); err != nil {
		return nil, err
	}
	gaussdbConn.status = connStatusClosed

	return &HijackedConn{
		Conn:              gaussdbConn.conn,
		PID:               gaussdbConn.pid,
		SecretKey:         gaussdbConn.secretKey,
		ParameterStatuses: gaussdbConn.parameterStatuses,
		TxStatus:          gaussdbConn.txStatus,
		Frontend:          gaussdbConn.frontend,
		Config:            gaussdbConn.config,
		CustomData:        gaussdbConn.customData,
	}, nil
}

// Construct created a GaussdbConn from an already established connection to a GaussDB server. This is the inverse of
// GaussdbConn.Hijack. The connection must be in an idle state.
//
// hc.Frontend is replaced by a new gaussdbproto.Frontend built by hc.Config.BuildFrontend.
//
// Due to the necessary exposure of internal implementation details, it is not covered by the semantic versioning
// compatibility.
func Construct(hc *HijackedConn) (*GaussdbConn, error) {
	gaussdbConn := &GaussdbConn{
		conn:              hc.Conn,
		pid:               hc.PID,
		secretKey:         hc.SecretKey,
		parameterStatuses: hc.ParameterStatuses,
		txStatus:          hc.TxStatus,
		frontend:          hc.Frontend,
		config:            hc.Config,
		customData:        hc.CustomData,

		status: connStatusIdle,

		cleanupDone: make(chan struct{}),
	}

	gaussdbConn.contextWatcher = ctxwatch.NewContextWatcher(hc.Config.BuildContextWatcherHandler(gaussdbConn))
	gaussdbConn.bgReader = bgreader.New(gaussdbConn.conn)
	gaussdbConn.slowWriteTimer = time.AfterFunc(time.Duration(math.MaxInt64),
		func() {
			gaussdbConn.bgReader.Start()
			gaussdbConn.bgReaderStarted <- struct{}{}
		},
	)
	gaussdbConn.slowWriteTimer.Stop()
	gaussdbConn.bgReaderStarted = make(chan struct{})
	gaussdbConn.frontend = hc.Config.BuildFrontend(gaussdbConn.bgReader, gaussdbConn.conn)

	return gaussdbConn, nil
}

// Pipeline represents a connection in pipeline mode.
//
// SendPrepare, SendQueryParams, and SendQueryPrepared queue requests to the server. These requests are not written until
// pipeline is flushed by Flush or Sync. Sync must be called after the last request is queued. Requests between
// synchronization points are implicitly transactional unless explicit transaction control statements have been issued.
//
// The context the pipeline was started with is in effect for the entire life of the Pipeline.
type Pipeline struct {
	conn *GaussdbConn
	ctx  context.Context

	state  pipelineState
	err    error
	closed bool
}

// PipelineSync is returned by GetResults when a ReadyForQuery message is received.
type PipelineSync struct{}

// CloseComplete is returned by GetResults when a CloseComplete message is received.
type CloseComplete struct{}

type pipelineRequestType int

const (
	pipelineNil pipelineRequestType = iota
	pipelinePrepare
	pipelineQueryParams
	pipelineQueryPrepared
	pipelineDeallocate
	pipelineSyncRequest
	pipelineFlushRequest
)

type pipelineRequestEvent struct {
	RequestType       pipelineRequestType
	WasSentToServer   bool
	BeforeFlushOrSync bool
}

type pipelineState struct {
	requestEventQueue          list.List
	lastRequestType            pipelineRequestType
	gaussdbErr                 *GaussdbError
	expectedReadyForQueryCount int
}

func (s *pipelineState) Init() {
	s.requestEventQueue.Init()
	s.lastRequestType = pipelineNil
}

func (s *pipelineState) RegisterSendingToServer() {
	for elem := s.requestEventQueue.Back(); elem != nil; elem = elem.Prev() {
		val := elem.Value.(pipelineRequestEvent)
		if val.WasSentToServer {
			return
		}
		val.WasSentToServer = true
		elem.Value = val
	}
}

func (s *pipelineState) registerFlushingBufferOnServer() {
	for elem := s.requestEventQueue.Back(); elem != nil; elem = elem.Prev() {
		val := elem.Value.(pipelineRequestEvent)
		if val.BeforeFlushOrSync {
			return
		}
		val.BeforeFlushOrSync = true
		elem.Value = val
	}
}

func (s *pipelineState) PushBackRequestType(req pipelineRequestType) {
	if req == pipelineNil {
		return
	}

	if req != pipelineFlushRequest {
		s.requestEventQueue.PushBack(pipelineRequestEvent{RequestType: req})
	}
	if req == pipelineFlushRequest || req == pipelineSyncRequest {
		s.registerFlushingBufferOnServer()
	}
	s.lastRequestType = req

	if req == pipelineSyncRequest {
		s.expectedReadyForQueryCount++
	}
}

func (s *pipelineState) ExtractFrontRequestType() pipelineRequestType {
	for {
		elem := s.requestEventQueue.Front()
		if elem == nil {
			return pipelineNil
		}
		val := elem.Value.(pipelineRequestEvent)
		if !(val.WasSentToServer && val.BeforeFlushOrSync) {
			return pipelineNil
		}

		s.requestEventQueue.Remove(elem)
		if val.RequestType == pipelineSyncRequest {
			s.gaussdbErr = nil
		}
		if s.gaussdbErr == nil {
			return val.RequestType
		}
	}
}

func (s *pipelineState) HandleError(err *GaussdbError) {
	s.gaussdbErr = err
}

func (s *pipelineState) HandleReadyForQuery() {
	s.expectedReadyForQueryCount--
}

func (s *pipelineState) PendingSync() bool {
	var notPendingSync bool

	if elem := s.requestEventQueue.Back(); elem != nil {
		val := elem.Value.(pipelineRequestEvent)
		notPendingSync = (val.RequestType == pipelineSyncRequest) && val.WasSentToServer
	} else {
		notPendingSync = (s.lastRequestType == pipelineSyncRequest) || (s.lastRequestType == pipelineNil)
	}

	return !notPendingSync
}

func (s *pipelineState) ExpectedReadyForQuery() int {
	return s.expectedReadyForQueryCount
}

// StartPipeline switches the connection to pipeline mode and returns a *Pipeline. In pipeline mode requests can be sent
// to the server without waiting for a response. Close must be called on the returned *Pipeline to return the connection
// to normal mode. While in pipeline mode, no methods that communicate with the server may be called except
// CancelRequest and Close. ctx is in effect for entire life of the *Pipeline.
//
// Prefer ExecBatch when only sending one group of queries at once.
func (gaussdbConn *GaussdbConn) StartPipeline(ctx context.Context) *Pipeline {
	if err := gaussdbConn.lock(); err != nil {
		pipeline := &Pipeline{
			closed: true,
			err:    err,
		}
		pipeline.state.Init()

		return pipeline
	}

	gaussdbConn.pipeline = Pipeline{
		conn: gaussdbConn,
		ctx:  ctx,
	}
	gaussdbConn.pipeline.state.Init()

	pipeline := &gaussdbConn.pipeline

	if ctx != context.Background() {
		select {
		case <-ctx.Done():
			pipeline.closed = true
			pipeline.err = newContextAlreadyDoneError(ctx)
			gaussdbConn.unlock()
			return pipeline
		default:
		}
		gaussdbConn.contextWatcher.Watch(ctx)
	}

	return pipeline
}

// SendPrepare is the pipeline version of *GaussdbConn.Prepare.
func (p *Pipeline) SendPrepare(name, sql string, paramOIDs []uint32) {
	if p.closed {
		return
	}

	p.conn.frontend.SendParse(&gaussdbproto.Parse{Name: name, Query: sql, ParameterOIDs: paramOIDs})
	p.conn.frontend.SendDescribe(&gaussdbproto.Describe{ObjectType: 'S', Name: name})
	p.state.PushBackRequestType(pipelinePrepare)
}

// SendDeallocate deallocates a prepared statement.
func (p *Pipeline) SendDeallocate(name string) {
	if p.closed {
		return
	}

	p.conn.frontend.SendClose(&gaussdbproto.Close{ObjectType: 'S', Name: name})
	p.state.PushBackRequestType(pipelineDeallocate)
}

// SendQueryParams is the pipeline version of *GaussdbConn.QueryParams.
func (p *Pipeline) SendQueryParams(sql string, paramValues [][]byte, paramOIDs []uint32, paramFormats []int16, resultFormats []int16) {
	if p.closed {
		return
	}

	p.conn.frontend.SendParse(&gaussdbproto.Parse{Query: sql, ParameterOIDs: paramOIDs})
	p.conn.frontend.SendBind(&gaussdbproto.Bind{ParameterFormatCodes: paramFormats, Parameters: paramValues, ResultFormatCodes: resultFormats})
	p.conn.frontend.SendDescribe(&gaussdbproto.Describe{ObjectType: 'P'})
	p.conn.frontend.SendExecute(&gaussdbproto.Execute{})
	p.state.PushBackRequestType(pipelineQueryParams)
}

// SendQueryPrepared is the pipeline version of *GaussdbConn.QueryPrepared.
func (p *Pipeline) SendQueryPrepared(stmtName string, paramValues [][]byte, paramFormats []int16, resultFormats []int16) {
	if p.closed {
		return
	}

	p.conn.frontend.SendBind(&gaussdbproto.Bind{PreparedStatement: stmtName, ParameterFormatCodes: paramFormats, Parameters: paramValues, ResultFormatCodes: resultFormats})
	p.conn.frontend.SendDescribe(&gaussdbproto.Describe{ObjectType: 'P'})
	p.conn.frontend.SendExecute(&gaussdbproto.Execute{})
	p.state.PushBackRequestType(pipelineQueryPrepared)
}

// SendFlushRequest sends a request for the server to flush its output buffer.
//
// The server flushes its output buffer automatically as a result of Sync being called,
// or on any request when not in pipeline mode; this function is useful to cause the server
// to flush its output buffer in pipeline mode without establishing a synchronization point.
// Note that the request is not itself flushed to the server automatically; use Flush if
// necessary. This copies the behavior of libpq PQsendFlushRequest.
func (p *Pipeline) SendFlushRequest() {
	if p.closed {
		return
	}

	p.conn.frontend.Send(&gaussdbproto.Flush{})
	p.state.PushBackRequestType(pipelineFlushRequest)
}

// SendPipelineSync marks a synchronization point in a pipeline by sending a sync message
// without flushing the send buffer. This serves as the delimiter of an implicit
// transaction and an error recovery point.
//
// Note that the request is not itself flushed to the server automatically; use Flush if
// necessary. This copies the behavior of libpq PQsendPipelineSync.
func (p *Pipeline) SendPipelineSync() {
	if p.closed {
		return
	}

	p.conn.frontend.SendSync(&gaussdbproto.Sync{})
	p.state.PushBackRequestType(pipelineSyncRequest)
}

// Flush flushes the queued requests without establishing a synchronization point.
func (p *Pipeline) Flush() error {
	if p.closed {
		if p.err != nil {
			return p.err
		}
		return errors.New("pipeline closed")
	}

	err := p.conn.flushWithPotentialWriteReadDeadlock()
	if err != nil {
		err = normalizeTimeoutError(p.ctx, err)

		p.conn.asyncClose()

		p.conn.contextWatcher.Unwatch()
		p.conn.unlock()
		p.closed = true
		p.err = err
		return err
	}

	p.state.RegisterSendingToServer()
	return nil
}

// Sync establishes a synchronization point and flushes the queued requests.
func (p *Pipeline) Sync() error {
	p.SendPipelineSync()
	return p.Flush()
}

// GetResults gets the next results. If results are present, results may be a *ResultReader, *StatementDescription, or
// *PipelineSync. If an ErrorResponse is received from the server, results will be nil and err will be a *GaussdbError. If no
// results are available, results and err will both be nil.
func (p *Pipeline) GetResults() (results any, err error) {
	if p.closed {
		if p.err != nil {
			return nil, p.err
		}
		return nil, errors.New("pipeline closed")
	}

	if p.state.ExtractFrontRequestType() == pipelineNil {
		return nil, nil
	}

	return p.getResults()
}

func (p *Pipeline) getResults() (results any, err error) {
	for {
		msg, err := p.conn.receiveMessage()
		if err != nil {
			p.closed = true
			p.err = err
			p.conn.asyncClose()
			return nil, normalizeTimeoutError(p.ctx, err)
		}

		switch msg := msg.(type) {
		case *gaussdbproto.RowDescription:
			p.conn.resultReader = ResultReader{
				gaussdbConn:       p.conn,
				pipeline:          p,
				ctx:               p.ctx,
				fieldDescriptions: p.conn.convertRowDescription(p.conn.fieldDescriptions[:], msg),
			}
			return &p.conn.resultReader, nil
		case *gaussdbproto.CommandComplete:
			p.conn.resultReader = ResultReader{
				commandTag:       p.conn.makeCommandTag(msg.CommandTag),
				commandConcluded: true,
				closed:           true,
			}
			return &p.conn.resultReader, nil
		case *gaussdbproto.ParseComplete:
			peekedMsg, err := p.conn.peekMessage()
			if err != nil {
				p.conn.asyncClose()
				return nil, normalizeTimeoutError(p.ctx, err)
			}
			if _, ok := peekedMsg.(*gaussdbproto.ParameterDescription); ok {
				return p.getResultsPrepare()
			}
		case *gaussdbproto.CloseComplete:
			return &CloseComplete{}, nil
		case *gaussdbproto.ReadyForQuery:
			p.state.HandleReadyForQuery()
			return &PipelineSync{}, nil
		case *gaussdbproto.ErrorResponse:
			gaussdbErr := ErrorResponseToGuassdbError(msg)
			p.state.HandleError(gaussdbErr)
			return nil, gaussdbErr
		}
	}
}

func (p *Pipeline) getResultsPrepare() (*StatementDescription, error) {
	psd := &StatementDescription{}

	for {
		msg, err := p.conn.receiveMessage()
		if err != nil {
			p.conn.asyncClose()
			return nil, normalizeTimeoutError(p.ctx, err)
		}

		switch msg := msg.(type) {
		case *gaussdbproto.ParameterDescription:
			psd.ParamOIDs = make([]uint32, len(msg.ParameterOIDs))
			copy(psd.ParamOIDs, msg.ParameterOIDs)
		case *gaussdbproto.RowDescription:
			psd.Fields = p.conn.convertRowDescription(nil, msg)
			return psd, nil

		// NoData is returned instead of RowDescription when there is no expected result. e.g. An INSERT without a RETURNING
		// clause.
		case *gaussdbproto.NoData:
			return psd, nil

		// These should never happen here. But don't take chances that could lead to a deadlock.
		case *gaussdbproto.ErrorResponse:
			gaussdbErr := ErrorResponseToGuassdbError(msg)
			p.state.HandleError(gaussdbErr)
			return nil, gaussdbErr
		case *gaussdbproto.CommandComplete:
			p.conn.asyncClose()
			return nil, errors.New("BUG: received CommandComplete while handling Describe")
		case *gaussdbproto.ReadyForQuery:
			p.conn.asyncClose()
			return nil, errors.New("BUG: received ReadyForQuery while handling Describe")
		}
	}
}

// Close closes the pipeline and returns the connection to normal mode.
func (p *Pipeline) Close() error {
	if p.closed {
		return p.err
	}

	p.closed = true

	if p.state.PendingSync() {
		p.conn.asyncClose()
		p.err = errors.New("pipeline has unsynced requests")
		p.conn.contextWatcher.Unwatch()
		p.conn.unlock()

		return p.err
	}

	for p.state.ExpectedReadyForQuery() > 0 {
		_, err := p.getResults()
		if err != nil {
			p.err = err
			var gaussdbError *GaussdbError
			if !errors.As(err, &gaussdbError) {
				p.conn.asyncClose()
				break
			}
		}
	}

	p.conn.contextWatcher.Unwatch()
	p.conn.unlock()

	return p.err
}

// DeadlineContextWatcherHandler handles canceled contexts by setting a deadline on a net.Conn.
type DeadlineContextWatcherHandler struct {
	Conn net.Conn

	// DeadlineDelay is the delay to set on the deadline set on net.Conn when the context is canceled.
	DeadlineDelay time.Duration
}

func (h *DeadlineContextWatcherHandler) HandleCancel(ctx context.Context) {
	h.Conn.SetDeadline(time.Now().Add(h.DeadlineDelay))
}

func (h *DeadlineContextWatcherHandler) HandleUnwatchAfterCancel() {
	h.Conn.SetDeadline(time.Time{})
}

// CancelRequestContextWatcherHandler handles canceled contexts by sending a cancel request to the server. It also sets
// a deadline on a net.Conn as a fallback.
type CancelRequestContextWatcherHandler struct {
	Conn *GaussdbConn

	// CancelRequestDelay is the delay before sending the cancel request to the server.
	CancelRequestDelay time.Duration

	// DeadlineDelay is the delay to set on the deadline set on net.Conn when the context is canceled.
	DeadlineDelay time.Duration

	cancelFinishedChan             chan struct{}
	handleUnwatchAfterCancelCalled func()
}

func (h *CancelRequestContextWatcherHandler) HandleCancel(context.Context) {
	h.cancelFinishedChan = make(chan struct{})
	var handleUnwatchedAfterCancelCalledCtx context.Context
	handleUnwatchedAfterCancelCalledCtx, h.handleUnwatchAfterCancelCalled = context.WithCancel(context.Background())

	deadline := time.Now().Add(h.DeadlineDelay)
	h.Conn.conn.SetDeadline(deadline)

	go func() {
		defer close(h.cancelFinishedChan)

		select {
		case <-handleUnwatchedAfterCancelCalledCtx.Done():
			return
		case <-time.After(h.CancelRequestDelay):
		}

		cancelRequestCtx, cancel := context.WithDeadline(handleUnwatchedAfterCancelCalledCtx, deadline)
		defer cancel()
		h.Conn.CancelRequest(cancelRequestCtx)

		// CancelRequest is inherently racy. Even though the cancel request has been received by the server at this point,
		// it hasn't necessarily been delivered to the other connection. If we immediately return and the connection is
		// immediately used then it is possible the CancelRequest will actually cancel our next query. The
		// TestCancelRequestContextWatcherHandler Stress test can produce this error without the sleep below. The sleep time
		// is arbitrary, but should be sufficient to prevent this error case.
		time.Sleep(100 * time.Millisecond)
	}()
}

func (h *CancelRequestContextWatcherHandler) HandleUnwatchAfterCancel() {
	h.handleUnwatchAfterCancelCalled()
	<-h.cancelFinishedChan

	h.Conn.conn.SetDeadline(time.Time{})
}

const (
	PlainPassword  = 0
	Md5Password    = 1
	Sha256Password = 2
)

func (gaussdbConn *GaussdbConn) writeBuf(b byte) *writeBuf {
	gaussdbConn.scratch[0] = b
	return &writeBuf{
		buf: gaussdbConn.scratch[:5],
		pos: 1,
	}
}

func md5s(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))
}
