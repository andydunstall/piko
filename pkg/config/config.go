package config

import (
	"github.com/spf13/pflag"
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
