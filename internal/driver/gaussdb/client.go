package gaussdb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/HuaweiCloudDeveloper/gaussdb-go/stdlib"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/util"
	"github.com/rs/zerolog/log"
)

type Client struct {
	DB     *sql.DB
	Config *config.SQLConfig
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
	return nil
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
