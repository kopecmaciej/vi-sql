package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseJumpTarget(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantSchema string
		wantTable  string
		wantErr    bool
	}{
		{
			name:       "valid schema/table",
			input:      "public/users",
			wantSchema: "public",
			wantTable:  "users",
		},
		{
			name:       "whitespace trimmed",
			input:      " public / users ",
			wantSchema: "public",
			wantTable:  "users",
		},
		{
			name:       "table name with slash kept as-is",
			input:      "public/schema/table",
			wantSchema: "public",
			wantTable:  "schema/table",
		},
		{
			name:    "no slash — error, not panic",
			input:   "public",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "empty schema",
			input:   "/users",
			wantErr: true,
		},
		{
			name:    "empty table",
			input:   "public/",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, table, err := parseJumpTarget(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantSchema, schema)
			assert.Equal(t, tt.wantTable, table)
		})
	}
}
