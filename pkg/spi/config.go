package spi

import (
	"fmt"
)

// LoadableConfig is an interface that defines the strategy to load a configuration from a given path.
type LoadableConfig[T any] interface {
	Load(configPath string) ([]T, error)
}

// LoadConfig loads the configuration from the given path using the strategy defined by the LoadableConfig implementation.
func LoadConfig[T LoadableConfig[T]](instance T, configPath string) ([]T, error) {
	defs, err := instance.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed loading config for: %w", err)
	}

	return defs, nil
}
