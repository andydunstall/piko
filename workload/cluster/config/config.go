package config

import (
	"fmt"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/spf13/pflag"
)

type Config struct {
	Nodes int `json:"nodes" yaml:"nodes"`

	Log log.Config `json:"log" yaml:"log"`
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
