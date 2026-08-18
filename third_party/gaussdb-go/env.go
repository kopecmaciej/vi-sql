package gaussdbgo

import "os"

// this just defines some environment variables that are used in the tests
const (
	EnvGaussdbTestDatabase            = "GAUSSDB_TEST_DATABASE"
	EnvGaussdbBenchSelectRowsCounts   = "GAUSSDB_BENCH_SELECT_ROWS_COUNTS"
	EnvGaussdbSslPassword             = "GAUSSDB_SSL_PASSWORD"
	EnvGaussdbTestCratedbConnString   = "GAUSSDB_TEST_CRATEDB_CONN_STRING"
	EnvGaussdbTestPgbouncerConnString = "GAUSSDB_TEST_PGBOUNCER_CONN_STRING"
	EnvGaussdbTestStressFactor        = "GAUSSDB_TEST_STRESS_FACTOR"
	EnvGaussdbTestTcpConnString       = "GAUSSDB_TEST_TCP_CONN_STRING"
	EnvGaussdbTestTlsClientConnString = "GAUSSDB_TEST_TLS_CLIENT_CONN_STRING"
	EnvGaussdbTestTlsConnString       = "GAUSSDB_TEST_TLS_CONN_STRING"
	EnvIsOpengauss                    = "IS_OPENGAUSS"
)

func IsTestingWithOpengauss() bool {
	return os.Getenv(EnvIsOpengauss) == "true"
}
