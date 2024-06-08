package config

import (
	"fmt"
	"net/url"

	"github.com/spf13/pflag"
)

type ServerConfig struct {
	// URL is the server URL.
	URL string `json:"url"`
}

func (c *ServerConfig) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("missing url")
	}
	if _, err := url.Parse(c.URL); err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	return nil
}

type Config struct {
	Server ServerConfig `json:"server"`
}

func (c *Config) Validate() error {
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	return nil
}

func (c *Config) RegisterFlags(fs *pflag.FlagSet) {
	fs.StringVar(
		&c.Server.URL,
		"server.url",
		"http://localhost:8002",
		`
Piko server URL. This URL should point to the server admin port.
`,
	)
}
