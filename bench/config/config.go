package config

import (
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/pflag"

	"github.com/andydunstall/piko/pkg/log"
)

type ConnectConfig struct {
	// ProxyURL is the server proxy URL.
	ProxyURL string `json:"proxy_url" yaml:"proxy_url"`

	// UpstreamURL is the server upstream URL.
	UpstreamURL string `json:"upstream_url" yaml:"upstream_url"`

	// Timeout is the timeout attempting to connect to the Piko server on
	// boot.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

func (c *ConnectConfig) Validate() error {
	if c.ProxyURL == "" {
		return fmt.Errorf("missing proxy url")
	}
	if _, err := url.Parse(c.ProxyURL); err != nil {
		return fmt.Errorf("invalid proxy url: %w", err)
	}
	if c.UpstreamURL == "" {
		return fmt.Errorf("missing upstream url")
	}
	if _, err := url.Parse(c.UpstreamURL); err != nil {
		return fmt.Errorf("invalid upstream url: %w", err)
	}
	if c.Timeout == 0 {
		return fmt.Errorf("missing timeout")
	}
	return nil
}

func (c *ConnectConfig) RegisterFlags(fs *pflag.FlagSet) {
	fs.StringVar(
		&c.ProxyURL,
		"connect.proxy-url",
		c.ProxyURL,
		`
Piko server proxy URL.

Note Piko connects to the server with WebSockets, so will replace http/https
with ws/wss (you can configure either).`,
	)
	fs.StringVar(
		&c.UpstreamURL,
		"connect.upstream-url",
		c.UpstreamURL,
		`
Piko server upstream URL.

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
	Requests int `json:"requests" yaml:"requests"`

	Clients int `json:"clients" yaml:"clients"`

	Endpoints int `json:"endpoints" yaml:"endpoints"`

	Upstreams int `json:"upstreams" yaml:"upstreams"`

	Size int `json:"size" yaml:"size"`

	Connect ConnectConfig `json:"server" yaml:"server"`

	Log log.Config `json:"log" yaml:"log"`
}

func Default() *Config {
	return &Config{
		Requests:  100000,
		Clients:   100,
		Endpoints: 100,
		Upstreams: 100,
		Size:      1024,
		Connect: ConnectConfig{
			ProxyURL:    "http://localhost:8000",
			UpstreamURL: "http://localhost:8001",
			Timeout:     time.Second * 5,
		},
		Log: log.Config{
			Level: "info",
		},
	}
}

func (c *Config) RegisterFlags(fs *pflag.FlagSet) {
	fs.IntVar(
		&c.Requests,
		"requests",
		c.Requests,
		`
Number of requests to send.`,
	)
	fs.IntVar(
		&c.Clients,
		"clients",
		c.Clients,
		`
Number of clients to connect.`,
	)
	fs.IntVar(
		&c.Endpoints,
		"endpoints",
		c.Endpoints,
		`
Number of endpoints to register.`,
	)
	fs.IntVar(
		&c.Upstreams,
		"upstreams",
		c.Upstreams,
		`
Number of upstream listeners to register.`,
	)
	fs.IntVar(
		&c.Size,
		"size",
		c.Size,
		`
Request payload size.`,
	)

	c.Connect.RegisterFlags(fs)

	c.Log.RegisterFlags(fs)
}

func (c *Config) Validate() error {
	if c.Requests == 0 {
		return fmt.Errorf("missing requests")
	}
	if c.Clients == 0 {
		return fmt.Errorf("missing clients")
	}
	if c.Endpoints == 0 {
		return fmt.Errorf("missing endpoints")
	}
	if c.Upstreams == 0 {
		return fmt.Errorf("missing upstreams")
	}
	if c.Size == 0 {
		return fmt.Errorf("missing size")
	}

	if err := c.Connect.Validate(); err != nil {
		return fmt.Errorf("server: %w", err)
	}

	if err := c.Log.Validate(); err != nil {
		return fmt.Errorf("log: %w", err)
	}

	return nil
}
