//go:build wezterm

package harness

// Environment variable names used to configure the wezterm test harness.
const (
	EnvDSN         = "VI_SQL_TESTS_DSN"
	EnvBinary      = "VI_SQL_TESTS_BINARY"
	EnvConnection  = "VI_SQL_TESTS_CONNECTION"
	EnvJump        = "VI_SQL_TESTS_JUMP"
	EnvLogPath     = "VI_SQL_TESTS_LOG_PATH"
	EnvSlow        = "VI_SQL_TESTS_SLOW"
	EnvKeyDelayMS  = "VI_SQL_TESTS_KEY_DELAY_MS"
	EnvFilterTable = "VI_SQL_TESTS_FILTER_TABLE"
)
