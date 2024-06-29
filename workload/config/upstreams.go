package config

import (
	"fmt"
	"net/url"
	"time"

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

type ChurnConfig struct {
	// Interval specifies how often each upstream should 'churn'
	// (disconnect and reconnect).
	Interval time.Duration `json:"interval" yaml:"interval"`

	// Delay is the duration to wait before reconnecting when churning.
	Delay time.Duration `json:"delay" yaml:"delay"`
}

func (c *ChurnConfig) RegisterFlags(fs *pflag.FlagSet) {
	fs.DurationVar(
		&c.Interval,
		"churn.interval",
		c.Interval,
		`
How often each upstream should 'churn' (disconnect and reconnect).`,
	)

	fs.DurationVar(
		&c.Delay,
		"churn.delay",
		c.Delay,
		`
Duration to wait before reconnecting when an upstream churns.`,
	)
}

type UpstreamsConfig struct {
	// Upstreams is the number of upstream servers to register.
	Upstreams int `json:"upstreams" yaml:"upstreams"`

	// Endpoints is the number of endpoint IDs to register.
	Endpoints int `json:"endpoints" yaml:"endpoints"`

	Churn ChurnConfig `json:"churn" yaml:"churn"`

	Server ServerConfig `json:"server" yaml:"server"`

	Log log.Config `json:"log" yaml:"log"`
}

func DefaultUpstreamsConfig() *UpstreamsConfig {
	return &UpstreamsConfig{
		Upstreams: 1000,
		Endpoints: 100,
		Churn: ChurnConfig{
			Interval: 0,
			Delay:    0,
		},
		Server: ServerConfig{
			URL: "http://localhost:8001",
		},
		Log: log.Config{
			Level: "info",
		},
	}
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
		c.Upstreams,
		`
The number of upstream servers to register.

Each upstream server registers with Piko using an endpoint ID selected from
the number endpoints to register.`,
	)

	fs.IntVar(
		&c.Endpoints,
		"endpoints",
		c.Endpoints,
		`
The number of available endpoint IDs to register.

Endpoint IDs will be assigned to upstreams from the number of endpoints. Such
as if you have 1000 upstreams and 100 endpoints, then you'll have 10 upstream
servers per endpoint.

Therefore 'endpoints' must be greater than or equal to 'upstreams'.`,
	)

	c.Churn.RegisterFlags(fs)

	fs.StringVar(
		&c.Server.URL,
		"server.url",
		c.Server.URL,
		`
Piko server URL.

Note Piko connects to the server with WebSockets, so will replace http/https
with ws/wss (you can configure either).`,
	)

	c.Log.RegisterFlags(fs)
}
