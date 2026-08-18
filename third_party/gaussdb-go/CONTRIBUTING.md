# Contributing

## Discuss Significant Changes

Before you invest a significant amount of time on a change, please create a discussion or issue describing your
proposal. This will help to ensure your proposed change has a reasonable chance of being merged.

## Avoid Dependencies

Adding a dependency is a big deal. While on occasion a new dependency may be accepted, the default answer to any change
that adds a dependency is no.

## Development Environment Setup

gaussdb-go tests naturally require a GaussDB database. It will connect to the database specified in the `GAUSSDB_TEST_DATABASE`
environment variable. The `GAUSSDB_TEST_DATABASE` environment variable can either be a URL or key-value pairs. In addition,
the standard `GAUSSDB_*` environment variables will be respected. 

### Using an Existing GaussDB Cluster

If you already have a GaussDB development server this is the quickest way to start and run the majority of the gaussdb-go
test suite. Some tests will be skipped that require server configuration changes (e.g. those testing different
authentication methods).

Create and setup a test database:

```
gsql -c 'create database gaussdbgo_test DBCOMPATIBILITY 'PG'; create extension hstore;'
```

```
Ensure your `GAUSSDB_TEST_DATABASE` environment variable points to the database you just created and run the tests.

```
export GAUSSDB_TEST_DATABASE="host='your gaussdb host' database=gaussdbgo_test"
go test ./...
```

This will run the vast majority of the tests, but some tests will be skipped (e.g. those testing different connection methods).

### Creating a New GaussDB Cluster Exclusively for Testing

The following environment variables need to be set both for initial setup and whenever the tests are run. (direnv is
highly recommended). Depending on your platform, you may need to change the host for `GAUSSDB_TEST_UNIX_SOCKET_CONN_STRING`.

```
export GAUSSDB_PORT=5015
export GAUSSDB_USER=root
export GAUSSDB_DATABASE=gaussdbgo_test
export GAUSSDB_DATA_DIR=/var/lib/opengauss/data

export GAUSSDB_TEST_DATABASE="host=127.0.0.1 database=gaussdbgo_test user=gaussdbgo_md5 password=secret"
export GAUSSDB_TEST_UNIX_SOCKET_CONN_STRING="host=/private/tmp database=gaussdbgo_test"
export GAUSSDB_TEST_TCP_CONN_STRING="host=127.0.0.1 database=gaussdbgo_test user=gaussdbgo_md5 password=secret"
export GAUSSDB_TEST_SCRAM_PASSWORD_CONN_STRING="host=127.0.0.1 user=gaussdbgo_scram password=secret database=gaussdbgo_test"
export GAUSSDB_TEST_MD5_PASSWORD_CONN_STRING="host=127.0.0.1 database=gaussdbgo_test user=gaussdbgo_md5 password=secret"
export GAUSSDB_TEST_PLAIN_PASSWORD_CONN_STRING="host=127.0.0.1 user=gaussdbgo_pw password=secret"
export GAUSSDB_TEST_TLS_CONN_STRING="host=localhost user=gaussdbgo_ssl password=secret sslmode=verify-full sslrootcert=`pwd`/.testdb/ca.pem"
export GAUSSDB_SSL_PASSWORD=certpw
export GAUSSDB_TEST_TLS_CLIENT_CONN_STRING="host=localhost user=gaussdbgo_sslcert sslmode=verify-full sslrootcert=`pwd`/.testdb/ca.pem database=gaussdbgo_test sslcert=`pwd`/.testdb/gaussdbgo_sslcert.crt sslkey=`pwd`/.testdb/gaussdbgo_sslcert.key"
```

Create a new database cluster.

```
initdb --locale=en_US -E UTF-8 --username=postgres .testdb/$POSTGRESQL_DATA_DIR

echo "listen_addresses = '127.0.0.1'" >> .testdb/$POSTGRESQL_DATA_DIR/GaussDB.conf
echo "port = $GAUSSDB_PORT" >> .testdb/$POSTGRESQL_DATA_DIR/GaussDB.conf
cat testsetup/postgresql_ssl.conf >> .testdb/$POSTGRESQL_DATA_DIR/GaussDB.conf
cp testsetup/pg_hba.conf .testdb/$POSTGRESQL_DATA_DIR/pg_hba.conf

cd .testdb

# Generate CA, server, and encrypted client certificates.
go run ../testsetup/generate_certs.go

# Copy certificates to server directory and set permissions.
cp ca.pem $POSTGRESQL_DATA_DIR/root.crt
cp localhost.key $POSTGRESQL_DATA_DIR/server.key
chmod 600 $POSTGRESQL_DATA_DIR/server.key
cp localhost.crt $POSTGRESQL_DATA_DIR/server.crt

cd ..
```


Start the new cluster. This will be necessary whenever you are running gaussdb-go tests.

```
postgres -D .testdb/$POSTGRESQL_DATA_DIR
```

Setup the test database in the new cluster.

```
createdb
psql --no-psqlrc -f testsetup/postgresql_setup.sql
```

### Optional Tests

gaussdb-go supports multiple connection types and means of authentication. These tests are optional. They will only run 
if theappropriate environment variables are set. In addition, there may be tests specific to particular GaussDB versions,
non-GaussDB servers (e.g. CockroachDB), or connection poolers (e.g. PgBouncer). `go test ./... -v | grep SKIP` to see
if any tests are being skipped.
