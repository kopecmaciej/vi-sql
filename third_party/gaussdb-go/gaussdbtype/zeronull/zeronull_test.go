package zeronull_test

import (
	"context"
	"os"
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype/zeronull"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbxtest"
	"github.com/stretchr/testify/require"
)

var defaultConnTestRunner gaussdbxtest.ConnTestRunner

func init() {
	defaultConnTestRunner = gaussdbxtest.DefaultConnTestRunner()
	defaultConnTestRunner.CreateConfig = func(ctx context.Context, t testing.TB) *gaussdbgo.ConnConfig {
		config, err := gaussdbgo.ParseConfig(os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
		require.NoError(t, err)
		return config
	}
	defaultConnTestRunner.AfterConnect = func(ctx context.Context, t testing.TB, conn *gaussdbgo.Conn) {
		zeronull.Register(conn.TypeMap())
	}
}
