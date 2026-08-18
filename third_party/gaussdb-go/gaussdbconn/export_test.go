// File export_test exports some methods for better testing.

package gaussdbconn

func NewParseConfigError(conn, msg string, err error) error {
	return &ParseConfigError{
		ConnString: conn,
		msg:        msg,
		err:        err,
	}
}
