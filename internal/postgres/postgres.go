package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

type Client struct {
	Pool   *pgxpool.Pool
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

	dsn := c.Config.GetDSN()
	if c.Config.Password != "" && config.EncryptionKey != "" {
		password, err := util.DecryptPassword(c.Config.Password, config.EncryptionKey)
		if err != nil {
			return err
		}
		sslMode := c.Config.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		dsn = util.BuildDSN(c.Config.Host, c.Config.Port, c.Config.Database, c.Config.Username, password, sslMode)
	}

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("failed to parse DSN: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	c.Pool = pool
	return nil
}

func (c *Client) Close() {
	if c.Pool != nil {
		c.Pool.Close()
	}
}

func (c *Client) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.Config.Timeout)*time.Second)
	defer cancel()
	return c.Pool.Ping(ctx)
}
