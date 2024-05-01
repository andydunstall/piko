package config

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load load the YAML configuration from the file at the given path.
//
// This will expand environment VARiables if expand is true.
func Load(path string, conf interface{}, expand bool) error {
	buf, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read file: %s: %w", path, err)
	}

	if expand {
		buf = []byte(expandEnv(string(buf)))
	}

	dec := yaml.NewDecoder(bytes.NewReader(buf))
	dec.KnownFields(true)

	if err := dec.Decode(conf); err != nil {
		return fmt.Errorf("parse config: %s: %w", path, err)
	}

	return nil
}

// expandEnv replaces ${VAR} or $VAR in the given string with the corresponding
// environment variable. The replacement is case-sensitive.
//
// References to undefined variables are replaced with an empty string. A
// default value can be given using form ${VAR:default}
func expandEnv(s string) string {
	return os.Expand(s, func(v string) string {
		elems := strings.SplitN(v, ":", 2)
		key := elems[0]

		env := os.Getenv(key)
		if env == "" && len(elems) == 2 {
			// If no env exists use the default.
			return elems[1]
		}
		return env
	})
}
