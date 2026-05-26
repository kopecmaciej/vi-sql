package sqlserver

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/util"
	_ "github.com/microsoft/go-mssqldb"
)

// Client wraps a database/sql connection to a SQL Server database.
type Client struct {
	DB     *sql.DB
	Config *config.SQLConfig
}

func NewClient(cfg *config.SQLConfig) *Client {
	return &Client{Config: cfg}
}

func (c *Client) Connect(ctx context.Context) error {
	dsn, err := c.buildDSN()
	if err != nil {
		return err
	}

	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		return fmt.Errorf("failed to open SQL Server connection: %w", err)
	}

	timeout := time.Duration(c.Config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return fmt.Errorf("failed to connect to SQL Server: %w", err)
	}
	c.DB = db
	return nil
}

func (c *Client) buildDSN() (string, error) {
	raw := c.Config.DSN
	if raw != "" {
		if envKey, ok := strings.CutPrefix(raw, "$"); ok {
			if resolved := os.Getenv(envKey); resolved != "" {
				raw = resolved
			}
		}
		if strings.HasPrefix(raw, "sqlserver://") {
			return raw, nil
		}
	}

	password := c.Config.Password
	if util.IsEncrypted(password) {
		if config.EncryptionKey == "" {
			return "", fmt.Errorf("connection has an encrypted password but no encryption key is loaded")
		}
		decrypted, _, err := util.DecryptPasswordWithMethod(password, config.EncryptionKey)
		if err != nil {
			return "", err
		}
		password = decrypted
	}

	port := c.Config.Port
	if port == 0 {
		port = 1433
	}
	encrypt := c.Config.SSLMode
	if encrypt == "" {
		encrypt = "disable"
	}

	return util.BuildSQLServerDSN(c.Config.Host, port, c.Config.Database, c.Config.Username, password, encrypt), nil
}

func (c *Client) Close() {
	if c.DB != nil {
		_ = c.DB.Close()
	}
}

func (c *Client) Ping(ctx context.Context) error {
	return c.DB.PingContext(ctx)
}
