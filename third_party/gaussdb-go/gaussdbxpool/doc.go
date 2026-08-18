// Package gaussdbxpool is a concurrency-safe connection pool for gaussdbgo.
/*
gaussdbxpool implements a nearly identical interface to gaussdbgo connections.

Creating a Pool

The primary way of creating a pool is with [gaussdbxpool.New]:

    pool, err := gaussdbxpool.New(context.Background(), os.Getenv("DATABASE_URL"))

The database connection string can be in URL or keyword/value format. GaussDB settings, gaussdbgo settings, and pool settings can be
specified here. In addition, a config struct can be created by [ParseConfig].

    config, err := gaussdbxpool.ParseConfig(os.Getenv("DATABASE_URL"))
    if err != nil {
        // ...
    }
    config.AfterConnect = func(ctx context.Context, conn *gaussdbgo.Conn) error {
        // do something with every new connection
    }

    pool, err := gaussdbxpool.NewWithConfig(context.Background(), config)

A pool returns without waiting for any connections to be established. Acquire a connection immediately after creating
the pool to check if a connection can successfully be established.
*/
package gaussdbxpool
