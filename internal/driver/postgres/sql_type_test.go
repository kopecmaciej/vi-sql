package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToSQLStandardTypeName(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"bool", "boolean"},
		{"boolean", "boolean"},
		{"int4", "int4"},
		{"varchar", "varchar"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, toSQLStandardTypeName(tt.in))
		})
	}
}
