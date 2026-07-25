package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/util"
	_ "github.com/sijms/go-ora/v2"
)

// Client wraps a database/sql connection to an Oracle database.
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

	db, err := sql.Open("oracle", dsn)
	if err != nil {
		return fmt.Errorf("failed to open Oracle connection: %w", err)
	}

	timeout := time.Duration(c.Config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return fmt.Errorf("failed to connect to Oracle: %w", err)
	}
	c.DB = db
	return nil
}

func (c *Client) buildDSN() (string, error) {
	raw := c.Config.DSN
	if envKey, ok := strings.CutPrefix(strings.TrimSpace(raw), "$"); ok && envKey != "" {
		if resolved := os.Getenv(envKey); resolved != "" {
			return resolved, nil
		}
	}

	if c.Config.Host != "" {
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
			port = 1521
		}
		return util.BuildOracleDSN(c.Config.Host, port, c.Config.Database, c.Config.Username, password), nil
	}

	if strings.HasPrefix(raw, "oracle://") {
		return raw, nil
	}

	return "", fmt.Errorf("no host or DSN configured for Oracle connection")
}

func (c *Client) Close() {
	if c.DB != nil {
		_ = c.DB.Close()
	}
}

func (c *Client) Ping(ctx context.Context) error {
	return c.DB.PingContext(ctx)
}
