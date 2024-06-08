package config

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Path      string `json:"path" yaml:"path"`
	ExpandEnv bool   `json:"expand_env" yaml:"expand_env"`
}

func (c *Config) RegisterFlags(fs *pflag.FlagSet) {
	fs.StringVar(
		&c.Path,
		"config.path",
		"",
		`
YAML config file path.`,
	)

	fs.BoolVar(
		&c.ExpandEnv,
		"config.expand-env",
		false,
		`
Whether to expand environment variables in the config file.

This will replaces references to ${VAR} or $VAR with the corresponding
environment variable. The replacement is case-sensitive.

References to undefined variables will be replaced with an empty string. A
default value can be given using form ${VAR:default}.`,
	)
}

// Load load the YAML configuration from the file at the given path.
func (c *Config) Load(conf interface{}) error {
	if c.Path == "" {
		return nil
	}

	buf, err := os.ReadFile(c.Path)
	if err != nil {
		return fmt.Errorf("read file: %s: %w", c.Path, err)
	}

	if c.ExpandEnv {
		buf = []byte(expandEnv(string(buf)))
	}

	dec := yaml.NewDecoder(bytes.NewReader(buf))
	dec.KnownFields(true)

	if err := dec.Decode(conf); err != nil {
		return fmt.Errorf("parse config: %s: %w", c.Path, err)
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
