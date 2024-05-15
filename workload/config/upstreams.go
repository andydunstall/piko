package config

import (
	"fmt"
	"net/url"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/spf13/pflag"
)

type ServerConfig struct {
	// URL is the server URL.
	URL string `json:"url" yaml:"url"`
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

type UpstreamsConfig struct {
	// Upstreams is the number of upstream servers to register.
	Upstreams int `json:"upstreams" yaml:"upstreams"`

	// Endpoints is the number of endpoint IDs to register.
	Endpoints int `json:"endpoints" yaml:"endpoints"`

	Server ServerConfig `json:"server" yaml:"server"`

	Log log.Config `json:"log" yaml:"log"`
}

func (c *UpstreamsConfig) Validate() error {
	if c.Upstreams == 0 {
		return fmt.Errorf("missing upstreams")
	}
	if c.Endpoints == 0 {
		return fmt.Errorf("missing endpoints")
	}
	if c.Endpoints > c.Upstreams {
		return fmt.Errorf("upstreams must be greater than or equal to endpoints")
	}

	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	if err := c.Log.Validate(); err != nil {
		return fmt.Errorf("log: %w", err)
	}
	return nil
}

func (c *UpstreamsConfig) RegisterFlags(fs *pflag.FlagSet) {
	fs.IntVar(
		&c.Upstreams,
		"upstreams",
		1000,
		`
The number of upstream servers to register.

Each upstream server registers with Piko using an endpoint ID selected from
the number endpoints to register.`,
	)

	fs.IntVar(
		&c.Endpoints,
		"endpoints",
		100,
		`
The number of available endpoint IDs to register.

Endpoint IDs will be assigned to upstreams from the number of endpoints. Such
as if you have 1000 upstreams and 100 endpoints, then you'll have 10 upstream
servers per endpoint.

Therefore 'endpoints' must be greater than or equal to 'upstreams'.`,
	)

	fs.StringVar(
		&c.Server.URL,
		"server.url",
		"http://localhost:8001",
		`
Piko server URL.

Note Piko connects to the server with WebSockets, so will replace http/https
with ws/wss (you can configure either).`,
	)

	c.Log.RegisterFlags(fs)
}
