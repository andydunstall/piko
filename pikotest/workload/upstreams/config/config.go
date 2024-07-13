package config

import (
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/pflag"

	agentconfig "github.com/andydunstall/piko/agent/config"
	"github.com/andydunstall/piko/pkg/log"
)

type ConnectConfig struct {
	// URL is the server URL.
	URL string `json:"url" yaml:"url"`

	// Timeout is the timeout attempting to connect to the Piko server on
	// boot.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

func (c *ConnectConfig) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("missing url")
	}
	if _, err := url.Parse(c.URL); err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if c.Timeout == 0 {
		return fmt.Errorf("missing timeout")
	}
	return nil
}

func (c *ConnectConfig) RegisterFlags(fs *pflag.FlagSet) {
	fs.StringVar(
		&c.URL,
		"connect.url",
		c.URL,
		`
Piko server URL.

Note Piko connects to the server with WebSockets, so will replace http/https
with ws/wss (you can configure either).`,
	)

	fs.DurationVar(
		&c.Timeout,
		"connect.timeout",
		c.Timeout,
		`
Timeout attempting to connect to the Piko server on boot. Note if the agent
is disconnected after the initial connection succeeds it will keep trying to
reconnect.`,
	)
}

type Config struct {
	Protocol string `json:"protocol" yaml:"protocol"`

	// Upstreams is the number of upstream servers to register.
	Upstreams int `json:"upstreams" yaml:"upstreams"`

	// Endpoints is the number of endpoint IDs to register.
	Endpoints int `json:"endpoints" yaml:"endpoints"`

	Connect ConnectConfig `json:"server" yaml:"server"`

	Log log.Config `json:"log" yaml:"log"`
}

func Default() *Config {
	return &Config{
		Protocol:  string(agentconfig.ListenerProtocolHTTP),
		Upstreams: 1000,
		Endpoints: 100,
		Connect: ConnectConfig{
			URL:     "http://localhost:8001",
			Timeout: time.Second * 5,
		},
		Log: log.Config{
			Level: "info",
		},
	}
}

func (c *Config) Validate() error {
	if c.Protocol == "" {
		return fmt.Errorf("missing protocol")
	}
	if agentconfig.ListenerProtocol(c.Protocol) != agentconfig.ListenerProtocolHTTP &&
		agentconfig.ListenerProtocol(c.Protocol) != agentconfig.ListenerProtocolTCP {
		return fmt.Errorf("unsupported protocol: %s", c.Protocol)
	}

	if c.Upstreams == 0 {
		return fmt.Errorf("missing upstreams")
	}
	if c.Endpoints == 0 {
		return fmt.Errorf("missing endpoints")
	}
	if c.Endpoints > c.Upstreams {
		return fmt.Errorf("upstreams must be greater than or equal to endpoints")
	}

	if err := c.Connect.Validate(); err != nil {
		return fmt.Errorf("server: %w", err)
	}

	if err := c.Log.Validate(); err != nil {
		return fmt.Errorf("log: %w", err)
	}

	return nil
}

func (c *Config) RegisterFlags(fs *pflag.FlagSet) {
	fs.StringVar(
		&c.Protocol,
		"protocol",
		string(c.Protocol),
		`
Upstream listener protocol (HTTP or TCP).`,
	)

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

	c.Connect.RegisterFlags(fs)

	c.Log.RegisterFlags(fs)
}
