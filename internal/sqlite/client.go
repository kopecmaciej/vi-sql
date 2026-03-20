package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/kopecmaciej/vi-sql/internal/config"
	_ "modernc.org/sqlite"
)

// Client wraps a database/sql connection to a SQLite database.
type Client struct {
	DB     *sql.DB
	Config *config.SQLConfig
}

func NewClient(cfg *config.SQLConfig) *Client {
	return &Client{Config: cfg}
}

func (c *Client) Connect(ctx context.Context) error {
	dsn := c.Config.GetDSN()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("failed to open sqlite database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping sqlite database: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}
	c.DB = db
	return nil
}

func (c *Client) Close() {
	if c.DB != nil {
		c.DB.Close()
	}
}

func (c *Client) Ping(ctx context.Context) error {
	return c.DB.PingContext(ctx)
}
