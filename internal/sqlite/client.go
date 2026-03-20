package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// resolvePath expands ~ and cleans the file path.
func resolvePath(p string) (string, error) {
	if strings.HasPrefix(p, "~/") || p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not resolve home directory: %w", err)
		}
		p = filepath.Join(home, p[1:])
	}
	return filepath.Clean(p), nil
}

func (c *Client) Connect(ctx context.Context) error {
	dsn := c.Config.GetDSN()

	// file: URIs and :memory: are passed directly to the driver.
	if !strings.HasPrefix(dsn, "file:") && dsn != ":memory:" {
		resolved, err := resolvePath(dsn)
		if err != nil {
			return err
		}
		dsn = resolved

		// Ensure parent directory exists so SQLite can create the file.
		dir := filepath.Dir(dsn)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create database directory %q: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("failed to open sqlite database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to open sqlite database %q: %w", dsn, err)
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
