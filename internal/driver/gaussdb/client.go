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
	CompatOracle CompatMode = "A" // Oracle-compatible (A-Compatibility)
	CompatMySQL  CompatMode = "M" // MySQL-compatible (M-Compatibility)
)

// ServerCapabilities describes what the connected GaussDB instance supports.
// Feature flags are derived from the detected compatibility mode so callers
// gate on capabilities rather than on the mode itself.
type ServerCapabilities struct {
	CompatMode CompatMode
	Version    string

	SupportsReturning    bool
	SupportsExplainJSON  bool
	SupportsExplainPerf  bool
	SupportsChangeColumn bool
	SupportsLastInsertID bool
}

type Client struct {
	DB           *sql.DB
	Config       *config.SQLConfig
	Capabilities ServerCapabilities
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
		dsn = util.BuildGaussDBDSN(c.Config.Host, c.Config.Port, c.Config.Database, c.Config.Username, password, c.Config.DriverOptions)
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
	c.detectCapabilities(ctx)
	return nil
}

// detectCapabilities determines the compatibility mode (so metadata queries
// use the matching information_schema dialect) and fills in the feature
// flags the server supports.
func (c *Client) detectCapabilities(ctx context.Context) {
	caps := ServerCapabilities{}

	if err := c.DB.QueryRowContext(ctx, "SELECT version()").Scan(&caps.Version); err != nil {
		log.Warn().Err(err).Msg("Failed to get GaussDB version")
	}

	mode := CompatOracle

	// The database's compatibility mode is stored in pg_database.datcompatibility
	// (M/MYSQL/B are MySQL-compatible; A/PG/ORA/C/TD use the PG-flavored catalogs).
	var compat *string
	if err := c.DB.QueryRowContext(ctx, `
		SELECT datcompatibility FROM pg_database WHERE datname = current_database()`).Scan(&compat); err == nil && compat != nil {
		switch strings.ToUpper(strings.TrimSpace(*compat)) {
		case "M", "MYSQL", "B":
			mode = CompatMySQL
		}
		// A-mode (or unknown) databases fall back to the default below.
	} else {
		// Fallback: probe the information_schema dialect directly. MySQL
		// compatible mode exposes column_key / column_type / extra which do
		// not exist in the PG-flavored information_schema; otherwise consult
		// the sql_compatibility GUC.
		var count int
		if err := c.DB.QueryRowContext(ctx, `
			SELECT count(*) FROM information_schema.columns
			WHERE table_name = 'columns' AND column_name = 'column_key'`).Scan(&count); err == nil && count > 0 {
			mode = CompatMySQL
		} else {
			var guc string
			if err := c.DB.QueryRowContext(ctx, "SHOW sql_compatibility").Scan(&guc); err == nil {
				switch strings.ToUpper(strings.TrimSpace(guc)) {
				case "M", "MYSQL", "B":
					mode = CompatMySQL
				}
			}
		}
	}

	caps.CompatMode = mode
	caps.SupportsReturning = mode == CompatOracle // M-mode rejects RETURNING on INSERT (0A000)
	caps.SupportsExplainJSON = true               // EXPLAIN (FORMAT JSON) is inherited
	caps.SupportsExplainPerf = true               // EXPLAIN (ANALYZE, FORMAT JSON) is inherited
	caps.SupportsChangeColumn = mode == CompatMySQL
	caps.SupportsLastInsertID = mode == CompatMySQL

	c.Capabilities = caps
	log.Info().
		Str("compat_mode", string(mode)).
		Str("version", caps.Version).
		Msg("Detected GaussDB compatibility mode")
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
