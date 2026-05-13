package mariadb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

// Client wraps a database/sql connection to a MariaDB database.
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

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open MariaDB connection: %w", err)
	}

	timeout := time.Duration(c.Config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return fmt.Errorf("failed to connect to MariaDB: %w", err)
	}
	c.DB = db
	return nil
}

func (c *Client) buildDSN() (string, error) {
	if raw := c.Config.DSN; raw != "" {
		if after, ok := strings.CutPrefix(raw, "$"); ok {
			if resolved := os.Getenv(after); resolved != "" {
				raw = resolved
			}
		}
		if strings.HasPrefix(raw, "mariadb://") && c.Config.Host == "" {
			parsed, err := util.ParseMySQLDSN(raw)
			if err != nil || parsed.Host == "" {
				return "", fmt.Errorf("could not parse mariadb DSN URL: %w", err)
			}
			port, _ := strconv.Atoi(parsed.Port)
			if port == 0 {
				port = 3306
			}
			cfg := mysqldrv.NewConfig()
			cfg.User = parsed.Username
			cfg.Passwd = parsed.Password
			cfg.Net = "tcp"
			cfg.Addr = fmt.Sprintf("%s:%d", parsed.Host, port)
			cfg.DBName = parsed.Database
			if parsed.SSLMode != "" && parsed.SSLMode != "false" {
				cfg.TLSConfig = parsed.SSLMode
			}
			applyConnectionDefaults(cfg)
			return cfg.FormatDSN(), nil
		}
		if !strings.HasPrefix(raw, "mariadb://") {
			cfg, err := mysqldrv.ParseDSN(raw)
			if err != nil {
				return "", fmt.Errorf("could not parse mariadb DSN: %w", err)
			}
			applyConnectionDefaults(cfg)
			return cfg.FormatDSN(), nil
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
		port = 3306
	}

	cfg := mysqldrv.NewConfig()
	cfg.User = c.Config.Username
	cfg.Passwd = password
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%d", c.Config.Host, port)
	cfg.DBName = c.Config.Database

	if c.Config.SSLMode != "" && c.Config.SSLMode != "false" {
		cfg.TLSConfig = c.Config.SSLMode
	}

	applyConnectionDefaults(cfg)
	return cfg.FormatDSN(), nil
}

// applyConnectionDefaults sets per-connection options that must apply to every
// pooled connection, regardless of whether the DSN came from form fields or a
// user-supplied raw string. User values in Params win.
func applyConnectionDefaults(cfg *mysqldrv.Config) {
	cfg.ParseTime = true
	cfg.MultiStatements = true
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	// Default is 1024 bytes; raise so GROUP_CONCAT in schema/FK/index queries
	// doesn't silently truncate and corrupt the final entry on strings.Split.
	if _, ok := cfg.Params["group_concat_max_len"]; !ok {
		cfg.Params["group_concat_max_len"] = "1048576"
	}
}

func (c *Client) Close() {
	if c.DB != nil {
		_ = c.DB.Close()
	}
}

func (c *Client) Ping(ctx context.Context) error {
	return c.DB.PingContext(ctx)
}
