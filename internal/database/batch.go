package database

import (
	"context"
	"database/sql"
	"fmt"
)

// RowUpdate captures a row's identity and values before and after an edit.
type RowUpdate struct {
	PrimaryKey PrimaryKey
	Original   Row
	Updated    Row
}

type SQLExec func(context.Context, string, ...any) (sql.Result, error)

// UpdateRowsTx applies a batch on one connection and commits only if all rows succeed.
func UpdateRowsTx(ctx context.Context, db *sql.DB, updates []RowUpdate, apply func(SQLExec, RowUpdate) error) error {
	if len(updates) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, update := range updates {
		if len(update.PrimaryKey.Columns) == 0 {
			return fmt.Errorf("row %d has no primary key", i+1)
		}
		if err := apply(tx.ExecContext, update); err != nil {
			return fmt.Errorf("row %d: %w", i+1, err)
		}
	}
	return tx.Commit()
}
