package gaussdbconn_test

import (
	"testing"

	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbconn"
	"github.com/stretchr/testify/assert"
)

func TestConfigError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		expectedMsg string
	}{
		{
			name:        "url with password",
			err:         gaussdbconn.NewParseConfigError("gaussdb://foo:password@host", "msg", nil),
			expectedMsg: "cannot parse `gaussdb://foo:xxxxx@host`: msg",
		},
		{
			name:        "keyword/value with password unquoted",
			err:         gaussdbconn.NewParseConfigError("host=host password=password user=user", "msg", nil),
			expectedMsg: "cannot parse `host=host password=xxxxx user=user`: msg",
		},
		{
			name:        "keyword/value with password quoted",
			err:         gaussdbconn.NewParseConfigError("host=host password='pass word' user=user", "msg", nil),
			expectedMsg: "cannot parse `host=host password=xxxxx user=user`: msg",
		},
		{
			name:        "weird url",
			err:         gaussdbconn.NewParseConfigError("gaussdb://foo::password@host:1:", "msg", nil),
			expectedMsg: "cannot parse `gaussdb://foo:xxxxx@host:1:`: msg",
		},
		{
			name:        "weird url with slash in password",
			err:         gaussdbconn.NewParseConfigError("gaussdb://user:pass/word@host:5432/db_name", "msg", nil),
			expectedMsg: "cannot parse `gaussdb://user:xxxxxx@host:5432/db_name`: msg",
		},
		{
			name:        "url without password",
			err:         gaussdbconn.NewParseConfigError("gaussdb://other@host/db", "msg", nil),
			expectedMsg: "cannot parse `gaussdb://other@host/db`: msg",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.EqualError(t, tt.err, tt.expectedMsg)
		})
	}
}
