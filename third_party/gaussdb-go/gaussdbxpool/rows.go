package gaussdbxpool

import (
	"github.com/HuaweiCloudDeveloper/gaussdb-go"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbconn"
)

type errRows struct {
	err error
}

func (errRows) Close()                                            {}
func (e errRows) Err() error                                      { return e.err }
func (errRows) CommandTag() gaussdbconn.CommandTag                { return gaussdbconn.CommandTag{} }
func (errRows) FieldDescriptions() []gaussdbconn.FieldDescription { return nil }
func (errRows) Next() bool                                        { return false }
func (e errRows) Scan(dest ...any) error                          { return e.err }
func (e errRows) Values() ([]any, error)                          { return nil, e.err }
func (e errRows) RawValues() [][]byte                             { return nil }
func (e errRows) Conn() *gaussdbgo.Conn                           { return nil }

type errRow struct {
	err error
}

func (e errRow) Scan(dest ...any) error { return e.err }

type poolRows struct {
	r   gaussdbgo.Rows
	c   *Conn
	err error
}

func (rows *poolRows) Close() {
	rows.r.Close()
	if rows.c != nil {
		rows.c.Release()
		rows.c = nil
	}
}

func (rows *poolRows) Err() error {
	if rows.err != nil {
		return rows.err
	}
	return rows.r.Err()
}

func (rows *poolRows) CommandTag() gaussdbconn.CommandTag {
	return rows.r.CommandTag()
}

func (rows *poolRows) FieldDescriptions() []gaussdbconn.FieldDescription {
	return rows.r.FieldDescriptions()
}

func (rows *poolRows) Next() bool {
	if rows.err != nil {
		return false
	}

	n := rows.r.Next()
	if !n {
		rows.Close()
	}
	return n
}

func (rows *poolRows) Scan(dest ...any) error {
	err := rows.r.Scan(dest...)
	if err != nil {
		rows.Close()
	}
	return err
}

func (rows *poolRows) Values() ([]any, error) {
	values, err := rows.r.Values()
	if err != nil {
		rows.Close()
	}
	return values, err
}

func (rows *poolRows) RawValues() [][]byte {
	return rows.r.RawValues()
}

func (rows *poolRows) Conn() *gaussdbgo.Conn {
	return rows.r.Conn()
}

type poolRow struct {
	r   gaussdbgo.Row
	c   *Conn
	err error
}

func (row *poolRow) Scan(dest ...any) error {
	if row.err != nil {
		return row.err
	}

	panicked := true
	defer func() {
		if panicked && row.c != nil {
			row.c.Release()
		}
	}()
	err := row.r.Scan(dest...)
	panicked = false
	if row.c != nil {
		row.c.Release()
	}
	return err
}
