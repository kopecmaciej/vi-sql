package sqlite

import (
	"context"
	"database/sql"
	"testing"

	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateRowsTransaction(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{true: "rollback", false: "commit"}[fail], func(t *testing.T) {
			db, err := sql.Open("sqlite", ":memory:")
			require.NoError(t, err)
			defer db.Close()
			db.SetMaxOpenConns(1)
			_, err = db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, value TEXT CHECK(value != 'invalid')); INSERT INTO items VALUES (1, 'first'), (2, 'second'), (3, 'untouched')`)
			require.NoError(t, err)
			dao := NewDao(&Client{DB: db})
			second := "changed"
			if fail {
				second = "invalid"
			}
			updates := []database.RowUpdate{
				{PrimaryKey: database.PrimaryKey{Columns: map[string]any{"id": 1}}, Original: database.Row{"id": 1, "value": "first"}, Updated: database.Row{"id": 1, "value": "quote ' ; DROP TABLE items; --"}},
				{PrimaryKey: database.PrimaryKey{Columns: map[string]any{"id": 2}}, Original: database.Row{"id": 2, "value": "second"}, Updated: database.Row{"id": 2, "value": second}},
			}
			err = dao.UpdateRows(context.Background(), "main", "items", updates)
			if fail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			var firstValue, secondValue, thirdValue string
			require.NoError(t, db.QueryRow("SELECT value FROM items WHERE id=1").Scan(&firstValue))
			require.NoError(t, db.QueryRow("SELECT value FROM items WHERE id=2").Scan(&secondValue))
			require.NoError(t, db.QueryRow("SELECT value FROM items WHERE id=3").Scan(&thirdValue))
			if fail {
				assert.Equal(t, "first", firstValue)
				assert.Equal(t, "second", secondValue)
			} else {
				assert.Equal(t, updates[0].Updated["value"], firstValue)
				assert.Equal(t, "changed", secondValue)
			}
			assert.Equal(t, "untouched", thirdValue)
		})
	}
}
