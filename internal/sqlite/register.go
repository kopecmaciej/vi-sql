package sqlite

import (
	"context"

	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
)

func init() {
	database.Register("sqlite", func(cfg *config.SQLConfig) (database.Driver, database.ValueFormatter, error) {
		client := NewClient(cfg)
		if err := client.Connect(context.Background()); err != nil {
			return nil, nil, err
		}
		return NewDao(client), &Formatter{}, nil
	})
}
