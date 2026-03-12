package database

import (
	"fmt"

	"github.com/kopecmaciej/vi-sql/internal/config"
)

// DriverFactory creates a Driver and ValueFormatter from a connection config.
type DriverFactory func(cfg *config.SQLConfig) (Driver, ValueFormatter, error)

var registry = map[string]DriverFactory{}

// Register associates a driver name with its factory function.
func Register(name string, factory DriverFactory) {
	registry[name] = factory
}

// NewDriver looks up the driver registered under cfg.GetDriver() and calls its factory.
func NewDriver(cfg *config.SQLConfig) (Driver, ValueFormatter, error) {
	name := cfg.GetDriver()
	factory, ok := registry[name]
	if !ok {
		return nil, nil, fmt.Errorf("unknown driver %q — did you import its package?", name)
	}
	return factory(cfg)
}
