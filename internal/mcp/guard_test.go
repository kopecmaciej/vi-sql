package mcp

import "testing"

func TestAssertReadOnly(t *testing.T) {
	allowed := []string{
		"SELECT * FROM users",
		"select id from auth.users where status = 'active'",
		"WITH cte AS (SELECT id FROM orders) SELECT * FROM cte",
		"EXPLAIN SELECT * FROM products",
		"EXPLAIN ANALYZE SELECT * FROM products",
		"TABLE catalog.products",
		"SELECT 'INSERT INTO foo' AS hint FROM dual",
		"SELECT id -- DELETE me later\nFROM users",
		"SELECT id /* UPDATE: remove this */ FROM users",
	}

	blocked := []string{
		"INSERT INTO users (email) VALUES ('x@x.com')",
		"UPDATE users SET status = 'deleted' WHERE id = '1'",
		"DELETE FROM users WHERE id = '1'",
		"DROP TABLE users",
		"ALTER TABLE users ADD COLUMN foo text",
		"CREATE TABLE foo (id int)",
		"TRUNCATE TABLE users",
		"WITH del AS (DELETE FROM users WHERE id = '1' RETURNING id) SELECT * FROM del",
		"SELECT * FROM (DELETE FROM users RETURNING id) AS d",
	}

	for _, q := range allowed {
		if err := assertReadOnly(q); err != nil {
			t.Errorf("expected allowed, got error for %q: %v", q, err)
		}
	}

	for _, q := range blocked {
		if err := assertReadOnly(q); err == nil {
			t.Errorf("expected blocked, but was allowed: %q", q)
		}
	}
}
