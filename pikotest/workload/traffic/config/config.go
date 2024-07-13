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

type Config struct {
	Protocol string `json:"protocol" yaml:"protocol"`

	Clients int `json:"clients" yaml:"clients"`

	Endpoints int `json:"endpoints" yaml:"endpoints"`

	// Rate is the number of requests/connections per second per client.
	Rate int `json:"rate" yaml:"rate"`

	// RequestSize is the size of each request.
	RequestSize int `json:"request_size" yaml:"request_size"`

	Connect ConnectConfig `json:"server" yaml:"server"`

	Log log.Config `json:"log" yaml:"log"`
}

func Default() *Config {
	return &Config{
		Protocol:    string(agentconfig.ListenerProtocolHTTP),
		Clients:     100,
		Endpoints:   100,
		Rate:        10,
		RequestSize: 1024,
		Connect: ConnectConfig{
			URL:     "http://localhost:8000",
			Timeout: time.Second * 5,
		},
		Log: log.Config{
			Level: "info",
		},
	}
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
		&c.Clients,
		"clients",
		c.Clients,
		`
...`,
	)

	fs.IntVar(
		&c.Endpoints,
		"endpoints",
		c.Endpoints,
		`
...`,
	)

	fs.IntVar(
		&c.Rate,
		"rate",
		c.Rate,
		`
The number of requests/connections per second per client.`,
	)

	fs.IntVar(
		&c.RequestSize,
		"request-size",
		c.RequestSize,
		`
The size of each request. As the upstream echos the response body, the response
will have the same size.`,
	)

	c.Log.RegisterFlags(fs)
}

func (c *Config) Validate() error {
	if c.Protocol == "" {
		return fmt.Errorf("missing protocol")
	}
	if agentconfig.ListenerProtocol(c.Protocol) != agentconfig.ListenerProtocolHTTP &&
		agentconfig.ListenerProtocol(c.Protocol) != agentconfig.ListenerProtocolTCP {
		return fmt.Errorf("unsupported protocol: %s", c.Protocol)
	}

	if c.Clients == 0 {
		return fmt.Errorf("missing clients")
	}

	if c.Endpoints == 0 {
		return fmt.Errorf("missing endpoints")
	}

	if c.Rate == 0 {
		return fmt.Errorf("missing rate")
	}

	if err := c.Connect.Validate(); err != nil {
		return fmt.Errorf("server: %w", err)
	}

	if err := c.Log.Validate(); err != nil {
		return fmt.Errorf("log: %w", err)
	}

	return nil
}
