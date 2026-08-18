// Package gaussdbgo is a GaussDB database driver.
/*
gaussdbgo provides a native GaussDB driver and can act as a database/sql driver. The native GaussDB interface is similar
to the database/sql interface while providing better speed and access to GaussDB specific features. Use
github.com/HuaweiCloudDeveloper/gaussdb-go/stdlib to use gaussdbgo as a database/sql compatible driver.
See that package's documentation for details.

Establishing a Connection

The primary way of establishing a connection is with [gaussdbgo.Connect]:

    conn, err := gaussdbgo.Connect(context.Background(), os.Getenv("DATABASE_URL"))

The database connection string can be in URL or key/value format. Both GaussDB settings and gaussdbgo settings can be
specified here. In addition, a config struct can be created by [ParseConfig] and modified before establishing the
connection with [ConnectConfig] to configure settings such as tracing that cannot be configured with a connection
string.

Connection Pool

[*gaussdbgo.Conn] represents a single connection to the database and is not concurrency safe. Use package
github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxpool for a concurrency safe connection pool.

Query Interface

gaussdbgo implements Query in the familiar database/sql style. However, gaussdbgo provides generic functions such as
CollectRows and ForEachRow that are a simpler and safer way of processing rows than manually calling defer rows.Close(),
rows.Next(), rows.Scan, and rows.Err().

CollectRows can be used collect all returned rows into a slice.

    rows, _ := conn.Query(context.Background(), "select generate_series(1,$1)", 5)
    numbers, err := gaussdbgo.CollectRows(rows, gaussdbgo.RowTo[int32])
    if err != nil {
      return err
    }
    // numbers => [1 2 3 4 5]

ForEachRow can be used to execute a callback function for every row. This is often easier than iterating over rows
directly.

    var sum, n int32
    rows, _ := conn.Query(context.Background(), "select generate_series(1,$1)", 10)
    _, err := gaussdbgo.ForEachRow(rows, []any{&n}, func() error {
      sum += n
      return nil
    })
    if err != nil {
      return err
    }

gaussdbgo also implements QueryRow in the same style as database/sql.

    var name string
    var weight int64
    err := conn.QueryRow(context.Background(), "select name, weight from widgets where id=$1", 42).Scan(&name, &weight)
    if err != nil {
        return err
    }

Use Exec to execute a query that does not return a result set.

    commandTag, err := conn.Exec(context.Background(), "delete from widgets where id=$1", 42)
    if err != nil {
        return err
    }
    if commandTag.RowsAffected() != 1 {
        return errors.New("No row found to delete")
    }

GaussDB Data Types

gaussdbgo uses the gaussdbtype package to converting Go values to and from GaussDB values. It supports many GaussDB types
directly and is customizable and extendable. User defined data types such as enums, domains,  and composite types may
require type registration. See that package's documentation for details.

Transactions

Transactions are started by calling Begin.

    tx, err := conn.Begin(context.Background())
    if err != nil {
        return err
    }
    // Rollback is safe to call even if the tx is already closed, so if
    // the tx commits successfully, this is a no-op
    defer tx.Rollback(context.Background())

    _, err = tx.Exec(context.Background(), "insert into foo(id) values (1)")
    if err != nil {
        return err
    }

    err = tx.Commit(context.Background())
    if err != nil {
        return err
    }

The Tx returned from Begin also implements the Begin method. This can be used to implement pseudo nested transactions.
These are internally implemented with savepoints.

Use BeginTx to control the transaction mode. BeginTx also can be used to ensure a new transaction is created instead of
a pseudo nested transaction.

BeginFunc and BeginTxFunc are functions that begin a transaction, execute a function, and commit or rollback the
transaction depending on the return value of the function. These can be simpler and less error prone to use.

    err = gaussdbgo.BeginFunc(context.Background(), conn, func(tx gaussdbgo.Tx) error {
        _, err := tx.Exec(context.Background(), "insert into foo(id) values (1)")
        return err
    })
    if err != nil {
        return err
    }

Prepared Statements

Prepared statements can be manually created with the Prepare method. However, this is rarely necessary because gaussdbgo
includes an automatic statement cache by default. Queries run through the normal Query, QueryRow, and Exec functions are
automatically prepared on first execution and the prepared statement is reused on subsequent executions. See ParseConfig
for information on how to customize or disable the statement cache.

Copy Protocol

Use CopyFrom to efficiently insert multiple rows at a time using the GaussDB copy protocol. CopyFrom accepts a
CopyFromSource interface. If the data is already in a [][]any use CopyFromRows to wrap it in a CopyFromSource interface.
Or implement CopyFromSource to avoid buffering the entire data set in memory.

    rows := [][]any{
        {"John", "Smith", int32(36)},
        {"Jane", "Doe", int32(29)},
    }

    copyCount, err := conn.CopyFrom(
        context.Background(),
        gaussdbgo.Identifier{"people"},
        []string{"first_name", "last_name", "age"},
        gaussdbgo.CopyFromRows(rows),
    )

When you already have a typed array using CopyFromSlice can be more convenient.

    rows := []User{
        {"John", "Smith", 36},
        {"Jane", "Doe", 29},
    }

    copyCount, err := conn.CopyFrom(
        context.Background(),
        gaussdbgo.Identifier{"people"},
        []string{"first_name", "last_name", "age"},
        gaussdbgo.CopyFromSlice(len(rows), func(i int) ([]any, error) {
            return []any{rows[i].FirstName, rows[i].LastName, rows[i].Age}, nil
        }),
    )

CopyFrom can be faster than an insert with as few as 5 rows.

Listen and Notify

gaussdbgo can listen to the GaussDB notification system with the `Conn.WaitForNotification` method. It blocks until a
notification is received or the context is canceled.

    _, err := conn.Exec(context.Background(), "listen channelname")
    if err != nil {
        return err
    }

    notification, err := conn.WaitForNotification(context.Background())
    if err != nil {
        return err
    }
    // do something with notification


Tracing and Logging

gaussdbgo supports tracing by setting ConnConfig.Tracer. To combine several tracers you can use the multitracer.Tracer.

In addition, the tracelog package provides the TraceLog type which lets a traditional logger act as a Tracer.

For debug tracing of the actual GaussDB wire protocol messages see github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbproto.

Lower Level GaussDB Functionality

github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbconn contains a lower level GaussDB driver .gaussdbgo.Conn in
implemented on top of gaussdbconn. The Conn.GaussdbConn() method can be used to access this lower layer.
*/
package gaussdbgo
