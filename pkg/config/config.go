package config

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func Load(path string, conf interface{}) error {
	buf, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %s: %w", path, err)
	}

	dec := yaml.NewDecoder(bytes.NewReader(buf))
	dec.KnownFields(true)

	if err := dec.Decode(conf); err != nil {
		return fmt.Errorf("parse config: %s: %w", path, err)
	}

	return nil
}
