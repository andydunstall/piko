package config

import (
	"fmt"

	"github.com/spf13/pflag"

	"github.com/andydunstall/piko/pkg/log"
)

type Config struct {
	Nodes int `json:"nodes" yaml:"nodes"`

	Log log.Config `json:"log" yaml:"log"`
}

func Default() *Config {
	return &Config{
		Nodes: 3,
		Log: log.Config{
			Level: "info",
		},
	}
}

func (c *Config) Validate() error {
	if c.Nodes == 0 {
		return fmt.Errorf("missing nodes")
	}

	if err := c.Log.Validate(); err != nil {
		return fmt.Errorf("log: %w", err)
	}
	return nil
}

func (c *Config) RegisterFlags(fs *pflag.FlagSet) {
	fs.IntVar(
		&c.Nodes,
		"nodes",
		3,
		`
The number of cluster nodes to start.`,
	)

	c.Log.RegisterFlags(fs)
}
