package gaussdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/HuaweiCloudDeveloper/gaussdb-go/stdlib"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog/log"
)

type CompatMode string

const (
	CompatA     CompatMode = "A" // PostgreSQL-compatible
	CompatMySQL CompatMode = "M" // MySQL-compatible (M-Compatibility)
)

type Client struct {
	DB         *sql.DB
	Config     *config.SQLConfig
	CompatMode CompatMode
}

func (c *Client) IsMySQLCompat() bool {
	return c.CompatMode == CompatMySQL
}

func NewClient(cfg *config.SQLConfig) *Client {
	return &Client{
		Config: cfg,
	}
}

func (c *Client) Connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.Config.Timeout)*time.Second)
	defer cancel()

	dsn := c.Config.GetDecryptedDSN()
	if c.Config.Password != "" && c.Config.Host != "" {
		password := c.Config.Password
		if util.IsEncrypted(password) {
			if config.EncryptionKey == "" {
				return fmt.Errorf("connection has an encrypted password but no encryption key is loaded")
			}
			decrypted, _, err := util.DecryptPasswordWithMethod(password, config.EncryptionKey)
			if err != nil {
				return err
			}
			password = decrypted
		}
		sslMode := c.Config.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		dsn = util.BuildGaussDBDSN(c.Config.Host, c.Config.Port, c.Config.Database, c.Config.Username, password, sslMode)
	}

	log.Info().Str("host", c.Config.Host).Int("port", c.Config.Port).Str("database", c.Config.Database).Msg("Connecting to GaussDB")

	db, err := sql.Open("gaussdb", dsn)
	if err != nil {
		return fmt.Errorf("failed to open GaussDB connection: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to connect to GaussDB: %w", err)
	}

	log.Info().Str("host", c.Config.Host).Int("port", c.Config.Port).Str("database", c.Config.Database).Msg("Connected to GaussDB")
	c.DB = db
	c.detectCompatMode(ctx)
	return nil
}

// detectCompatMode determines whether the connected database runs in
// MySQL-compatible (M) or PostgreSQL-compatible (A) mode so that metadata
// queries can use the matching information_schema dialect.
func (c *Client) detectCompatMode(ctx context.Context) {
	mode := CompatA

	// Probe the information_schema dialect directly: MySQL-compatible mode
	// exposes column_key / column_type / extra which do not exist in the
	// PG-flavored information_schema.
	var count int
	if err := c.DB.QueryRowContext(ctx, `
		SELECT count(*) FROM information_schema.columns
		WHERE table_name = 'columns' AND column_name = 'column_key'`).Scan(&count); err == nil && count > 0 {
		mode = CompatMySQL
	} else {
		// Fallback to the sql_compatibility GUC when the probe is not
		// conclusive (value set varies across versions: M/MYSQL/B are
		// MySQL-compatible; A/PG/ORA/C/TD use the PG-flavored catalogs).
		var compat string
		if err := c.DB.QueryRowContext(ctx, "SHOW sql_compatibility").Scan(&compat); err == nil {
			switch strings.ToUpper(strings.TrimSpace(compat)) {
			case "M", "MYSQL", "B":
				mode = CompatMySQL
			}
		}
	}

	c.CompatMode = mode
	log.Info().Str("compat_mode", string(mode)).Msg("Detected GaussDB compatibility mode")
}

func (c *Client) Close() {
	if c.DB != nil {
		c.DB.Close()
	}
}

func (c *Client) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.Config.Timeout)*time.Second)
	defer cancel()
	return c.DB.PingContext(ctx)
}
