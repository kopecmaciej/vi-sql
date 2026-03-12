package postgres

import (
	"github.com/kopecmaciej/vi-sql/internal/config"
	"github.com/kopecmaciej/vi-sql/internal/database"
)

func init() {
	database.Register("postgres", func(cfg *config.SQLConfig) (database.Driver, database.ValueFormatter, error) {
		client := NewClient(cfg)
		if err := client.Connect(); err != nil {
			return nil, nil, err
		}
		if err := client.Ping(); err != nil {
			return nil, nil, err
		}
		return NewDao(client), &Formatter{}, nil
	})
}
