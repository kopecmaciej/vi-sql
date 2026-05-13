package sql

import (
	"testing"
)

func TestIsExplainQuery(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want bool
	}{
		{"EXPLAIN keyword", "EXPLAIN SELECT * FROM t", true},
		{"EXPLAIN ANALYZE", "EXPLAIN ANALYZE SELECT * FROM t", true},
		{"lowercase explain", "explain select * from t", true},
		{"leading whitespace", "  EXPLAIN SELECT 1", true},
		{"plain SELECT", "SELECT * FROM t", false},
		{"empty string", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsExplainQuery(tc.sql)
			if got != tc.want {
				t.Errorf("IsExplainQuery(%q) = %v, want %v", tc.sql, got, tc.want)
			}
		})
	}
}

func TestDestructiveStatement(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		wantNil   bool
		wantOp    string
		wantTable string
		wantWhere bool
	}{
		{"DELETE without WHERE", "DELETE FROM users", false, "DELETE", "users", false},
		{"DELETE with WHERE", "DELETE FROM orders WHERE id = 1", false, "DELETE", "orders", true},
		{"DELETE schema-qualified", "DELETE FROM public.logs WHERE id > 0", false, "DELETE", "public.logs", true},
		{"DELETE subquery has WHERE but top-level does not", "DELETE FROM t WHERE id IN (SELECT id FROM s WHERE active)", false, "DELETE", "t", true},
		{"UPDATE without WHERE", "UPDATE accounts SET balance = 0", false, "UPDATE", "accounts", false},
		{"UPDATE with WHERE", "UPDATE accounts SET balance = 0 WHERE id = 5", false, "UPDATE", "accounts", true},
		{"TRUNCATE", "TRUNCATE users", false, "TRUNCATE", "users", false},
		{"TRUNCATE TABLE", "TRUNCATE TABLE public.events", false, "TRUNCATE", "public.events", false},
		{"DROP TABLE", "DROP TABLE temp", false, "DROP", "temp", false},
		{"DROP TABLE IF EXISTS", "DROP TABLE IF EXISTS temp", false, "DROP", "temp", false},
		{"DROP INDEX", "DROP INDEX idx_name", false, "DROP", "idx_name", false},
		{"SELECT is not destructive", "SELECT * FROM t", true, "", "", false},
		{"INSERT is not destructive", "INSERT INTO t VALUES (1)", true, "", "", false},
		{"empty", "", true, "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := HasDestructiveStatement(tc.sql)
			if tc.wantNil {
				if got != nil {
					t.Errorf("HasDestructiveStatement(%q) = %+v, want nil", tc.sql, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("HasDestructiveStatement(%q) = nil, want non-nil", tc.sql)
			}
			if got.Operation != tc.wantOp {
				t.Errorf("Operation=%q, want %q", got.Operation, tc.wantOp)
			}
			if got.Table != tc.wantTable {
				t.Errorf("Table=%q, want %q", got.Table, tc.wantTable)
			}
			if got.HasWhere != tc.wantWhere {
				t.Errorf("HasWhere=%v, want %v", got.HasWhere, tc.wantWhere)
			}
		})
	}
}
