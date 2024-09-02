package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
)

// Load loads the configuration from the given path.
func Load(instance any, configPath string) error {
	rv := reflect.ValueOf(instance)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return errors.New("must be a non-nil pointer")
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("could not read config file [%s]: %w", configPath, err)
	}

	err = json.Unmarshal(content, instance)
	if err != nil {
		return fmt.Errorf("could not parse json content of [%s]: %w", configPath, err)
	}

	return nil
}
