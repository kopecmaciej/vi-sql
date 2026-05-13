package sqlite

import (
	"context"
	"fmt"

	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
	"github.com/kopecmaciej/vi-sql/internal/util"
)

func init() {
	database.RegisterConnector("sqlite", database.ConnectorDef{
		Quoter: util.ANSIQuoter,
		Factory: func(cfg *config.SQLConfig) (database.Driver, database.ValueFormatter, error) {
			client := NewClient(cfg)
			if err := client.Connect(context.Background()); err != nil {
				return nil, nil, err
			}
			return NewDao(client), database.DefaultFormatter{}, nil
		},
		FormSpec: []database.FieldSpec{
			{Kind: database.FieldInput, Label: "Path / URI"},
			{Kind: database.FieldLabel, Label: "Example", Default: "~/db.sqlite, file:/path/db?mode=ro or :memory:"},
		},
		PreFill: func(conn *config.SQLConfig) map[string]string {
			return map[string]string{
				"Path / URI": conn.DSN,
			}
		},
		BuildConfig: buildSQLiteConfig,
	})
}

func buildSQLiteConfig(fields map[string]string, _ *config.SQLConfig) (*config.SQLConfig, error) {
	filePath := fields["Path / URI"]
	if filePath == "" {
		filePath = fields["DSN"]
	}
	if filePath == "" {
		return nil, fmt.Errorf("please enter a SQLite database path, URI or :memory:")
	}
	name := fields["Name"]
	if name == "" {
		name = filePath
	}
	return &config.SQLConfig{
		Driver: "sqlite",
		Name:   name,
		DSN:    filePath,
	}, nil
}
