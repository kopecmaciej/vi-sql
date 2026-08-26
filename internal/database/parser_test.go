package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRebuildSelectSQL(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		where   string
		orderBy string
		want    string
	}{
		{
			name:  "inject WHERE into plain SELECT",
			sql:   "SELECT * FROM t",
			where: "id > 0",
			want:  "SELECT * FROM t WHERE id > 0",
		},
		{
			name:    "inject ORDER BY into plain SELECT",
			sql:     "SELECT * FROM t",
			orderBy: "name ASC",
			want:    "SELECT * FROM t ORDER BY name ASC",
		},
		{
			name:    "inject WHERE and ORDER BY",
			sql:     "SELECT * FROM t",
			where:   "id > 0",
			orderBy: "id ASC",
			want:    "SELECT * FROM t WHERE id > 0 ORDER BY id ASC",
		},
		{
			name:    "preserve LIMIT when adding ORDER BY",
			sql:     "SELECT * FROM t LIMIT 100",
			orderBy: "id ASC",
			want:    "SELECT * FROM t ORDER BY id ASC LIMIT 100",
		},
		{
			name:  "preserve LIMIT when adding WHERE",
			sql:   "SELECT * FROM t LIMIT 100",
			where: "id > 5",
			want:  "SELECT * FROM t WHERE id > 5 LIMIT 100",
		},
		{
			name:    "replace existing ORDER BY, preserve LIMIT",
			sql:     "SELECT * FROM t ORDER BY old ASC LIMIT 50",
			orderBy: "new DESC",
			want:    "SELECT * FROM t ORDER BY new DESC LIMIT 50",
		},
		{
			name:    "replace WHERE and ORDER BY, preserve LIMIT",
			sql:     "SELECT * FROM t WHERE a = 1 ORDER BY b LIMIT 20",
			where:   "c = 3",
			orderBy: "d ASC",
			want:    "SELECT * FROM t WHERE c = 3 ORDER BY d ASC LIMIT 20",
		},
		{
			name:  "clear WHERE by passing empty string",
			sql:   "SELECT * FROM t WHERE a = 1",
			where: "",
			want:  "SELECT * FROM t",
		},
		{
			name:    "clear ORDER BY by passing empty string",
			sql:     "SELECT * FROM t ORDER BY x",
			orderBy: "",
			want:    "SELECT * FROM t",
		},
		{
			name:    "LIMIT with OFFSET is fully preserved",
			sql:     "SELECT * FROM t ORDER BY id LIMIT 50 OFFSET 100",
			orderBy: "name ASC",
			want:    "SELECT * FROM t ORDER BY name ASC LIMIT 50 OFFSET 100",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RebuildSelectSQL(tc.sql, tc.where, tc.orderBy)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExtractLimitValue_MySQLCommaForm(t *testing.T) {
	limit, ok := ExtractLimitValue("SELECT * FROM t LIMIT 20, 10")
	assert.True(t, ok)
	assert.Equal(t, int64(10), limit, "second number is the count, not the offset")
}

func TestExtractLimitValue_SimpleForm(t *testing.T) {
	limit, ok := ExtractLimitValue("select * from t limit 5 offset 3")
	assert.True(t, ok)
	assert.Equal(t, int64(5), limit)
}

func TestExtractLimitValue_Absent(t *testing.T) {
	_, ok := ExtractLimitValue("SELECT * FROM t")
	assert.False(t, ok)
}

func TestRebuildSelectSQL_NormalizesMySQLLimitForm(t *testing.T) {
	got := RebuildSelectSQL("SELECT * FROM users LIMIT 20, 10", "id > 1", "")
	assert.Equal(t, "SELECT * FROM users WHERE id > 1 LIMIT 10 OFFSET 20", got)
}
