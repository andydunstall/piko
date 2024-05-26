package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/spf13/pflag"
)

type ServerConfig struct {
	// URL is the server URL.
	URL                 string        `json:"url" yaml:"url"`
	HeartbeatInterval   time.Duration `json:"heartbeat_interval" yaml:"heartbeat_interval"`
	HeartbeatTimeout    time.Duration `json:"heartbeat_timeout" yaml:"heartbeat_timeout"`
	ReconnectMinBackoff time.Duration `json:"reconnect_min_backoff" yaml:"reconnect_min_backoff"`
	ReconnectMaxBackoff time.Duration `json:"reconnect_max_backoff" yaml:"reconnect_max_backoff"`
}

func (c *ServerConfig) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("missing url")
	}
	if _, err := url.Parse(c.URL); err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if c.HeartbeatInterval == 0 {
		return fmt.Errorf("missing heartbeat interval")
	}
	if c.HeartbeatTimeout == 0 {
		return fmt.Errorf("missing heartbeat timeout")
	}
	if c.ReconnectMinBackoff == 0 {
		return fmt.Errorf("missing reconnect min backoff")
	}
	if c.ReconnectMaxBackoff == 0 {
		return fmt.Errorf("missing reconnect max backoff")
	}
	return nil
}

func (c *ServerConfig) RegisterFlags(fs *pflag.FlagSet) {
	fs.StringVar(
		&c.URL,
		"server.url",
		"http://localhost:8001",
		`
Piko server URL.

The listener will add path /piko/v1/listener/:endpoint_id to the given URL,
so if you include a path it will be used as a prefix.

Note Piko connects to the server with WebSockets, so will replace http/https
with ws/wss (you can configure either).`,
	)
	fs.DurationVar(
		&c.HeartbeatInterval,
		"server.heartbeat-interval",
		time.Second*10,
		`
Heartbeat interval.

To verify the connection to the server is ok, the listener sends a
heartbeat to the upstream at the '--server.heartbeat-interval'
interval, with a timeout of '--server.heartbeat-timeout'.`,
	)
	fs.DurationVar(
		&c.HeartbeatTimeout,
		"server.heartbeat-timeout",
		time.Second*10,
		`
Heartbeat timeout.

To verify the connection to the server is ok, the listener sends a
heartbeat to the upstream at the '--server.heartbeat-interval'
interval, with a timeout of '--server.heartbeat-timeout'.`,
	)
	fs.DurationVar(
		&c.ReconnectMinBackoff,
		"server.reconnect-min-backoff",
		time.Millisecond*500,
		`
Minimum backoff when reconnecting to the server.`,
	)
	fs.DurationVar(
		&c.ReconnectMaxBackoff,
		"server.reconnect-max-backoff",
		time.Second*15,
		`
Maximum backoff when reconnecting to the server.`,
	)
}

type AuthConfig struct {
	APIKey string `json:"api_key" yaml:"api_key"`
}

// ForwarderConfig contains the configuration for how to forward requests
// from Piko.
type ForwarderConfig struct {
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

type AdminConfig struct {
	// BindAddr is the address to bind to listen for incoming HTTP connections.
	BindAddr string `json:"bind_addr" yaml:"bind_addr"`
}

func (c *AdminConfig) Validate() error {
	if c.BindAddr == "" {
		return fmt.Errorf("missing bind addr")
	}
	return nil
}

type Config struct {
	// Endpoints is a list of endpoints and forward addresses to register.
	//
	// Each endpoint has format '<endpoint ID>/<forward addr>', such
	// as 'd3934d4f/localhost:3000'.
	Endpoints []string        `json:"endpoints" yaml:"endpoints"`
	Server    ServerConfig    `json:"server" yaml:"server"`
	Auth      AuthConfig      `json:"auth" yaml:"auth"`
	Forwarder ForwarderConfig `json:"forwarder" yaml:"forwarder"`
	Admin     AdminConfig     `json:"admin" yaml:"admin"`
	Log       log.Config      `json:"log" yaml:"log"`
}

func (c *Config) Validate() error {
	if len(c.Endpoints) == 0 {
		return fmt.Errorf("must have at least one endpoint")
	}
	for _, ln := range c.Endpoints {
		if len(strings.Split(ln, "/")) != 2 {
			return fmt.Errorf("invalid endpoint: %s", ln)
		}
	}

	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	if err := c.Admin.Validate(); err != nil {
		return fmt.Errorf("admin: %w", err)
	}
	if err := c.Log.Validate(); err != nil {
		return fmt.Errorf("log: %w", err)
	}
	return nil
}

func (c *Config) RegisterFlags(fs *pflag.FlagSet) {
	fs.StringSliceVar(
		&c.Endpoints,
		"endpoints",
		nil,
		`
The endpoints to register with the Piko server.

Each endpoint has an ID and forwarding address. The agent will register the
endpoint with the Piko server then receive incoming requests for that endpoint
and forward them to the configured address.

'--endpoints' is a comma separated list of endpoints with format:
'<endpoint ID>/<forward addr>'. Such as '--endpoints 6ae6db60/localhost:3000'
will register the endpoint '6ae6db60' then forward incoming requests to
'localhost:3000'.

You may register multiple endpoints which have their own connection to Piko,
such as '--endpoints 6ae6db60/localhost:3000,941c3c2e/localhost:4000'.`,
	)

	c.Server.RegisterFlags(fs)

	fs.StringVar(
		&c.Auth.APIKey,
		"auth.api-key",
		"",
		`
An API key to authenticate the connection to Piko.`,
	)

	fs.DurationVar(
		&c.Forwarder.Timeout,
		"forwarder.timeout",
		time.Second*10,
		`
Forwarder timeout.

This is the timeout between a listener receiving a request from Piko then
forwarding it to the configured forward address, and receiving a response.

If the upstream does not respond within the given timeout a
'504 Gateway Timeout' is returned to the client.`,
	)

	fs.StringVar(
		&c.Admin.BindAddr,
		"admin.bind-addr",
		":9000",
		`
The host/port to listen for incoming admin connections.

If the host is unspecified it defaults to all listeners, such as
'--admin.bind-addr :9000' will listen on '0.0.0.0:9000'`,
	)

	c.Log.RegisterFlags(fs)
}
