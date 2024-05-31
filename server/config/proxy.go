package config

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"
)

// ProxyHTTPConfig contains generic configuration for the HTTP servers.
type ProxyHTTPConfig struct {
	// ReadTimeout is the maximum duration for reading the entire
	// request, including the body. A zero or negative value means
	// there will be no timeout.
	ReadTimeout time.Duration `json:"read_timeout" yaml:"read_timeout"`

	// ReadHeaderTimeout is the amount of time allowed to read
	// request headers.
	ReadHeaderTimeout time.Duration `json:"read_header_timeout" yaml:"read_header_timeout"`

	// WriteTimeout is the maximum duration before timing out
	// writes of the response.
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`

	// IdleTimeout is the maximum amount of time to wait for the
	// next request when keep-alives are enabled.
	IdleTimeout time.Duration `json:"idle_timeout" yaml:"idle_timeout"`

	// MaxHeaderBytes controls the maximum number of bytes the
	// server will read parsing the request header's keys and
	// values, including the request line.
	MaxHeaderBytes int `json:"max_header_bytes" yaml:"max_header_bytes"`
}

func (c *ProxyHTTPConfig) RegisterFlags(fs *pflag.FlagSet, prefix string) {
	if prefix == "" {
		prefix = "http."
	} else {
		prefix = prefix + ".http."
	}

	fs.DurationVar(
		&c.ReadTimeout,
		prefix+"read-timeout",
		time.Second*10,
		`
The maximum duration for reading the entire request, including the body. A
zero or negative value means there will be no timeout.`,
	)
	fs.DurationVar(
		&c.ReadHeaderTimeout,
		prefix+"read-header-timeout",
		time.Second*10,
		`
The maximum duration for reading the request headers. If zero,
http.read-timeout is used.`,
	)
	fs.DurationVar(
		&c.WriteTimeout,
		prefix+"write-timeout",
		time.Second*10,
		`
The maximum duration before timing out writes of the response.`,
	)
	fs.DurationVar(
		&c.IdleTimeout,
		prefix+"idle-timeout",
		time.Minute*5,
		`
The maximum amount of time to wait for the next request when keep-alives are
enabled.`,
	)
	fs.IntVar(
		&c.MaxHeaderBytes,
		prefix+"max-header-bytes",
		1<<20,
		`
The maximum number of bytes the server will read parsing the request header's
keys and values, including the request line.`,
	)
}

type ProxyConfig struct {
	// BindAddr is the address to bind to listen for incoming HTTP connections.
	BindAddr string `json:"bind_addr" yaml:"bind_addr"`

	// AdvertiseAddr is the address to advertise to other nodes.
	AdvertiseAddr string `json:"advertise_addr" yaml:"advertise_addr"`

	// GatewayTimeout is the timeout in seconds of forwarding requests to an
	// upstream listener.
	GatewayTimeout time.Duration `json:"gateway_timeout" yaml:"gateway_timeout"`

	HTTP ProxyHTTPConfig `json:"http" yaml:"http"`

	TLS TLSConfig `json:"tls" yaml:"tls"`
}

func (c *ProxyConfig) Validate() error {
	if c.BindAddr == "" {
		return fmt.Errorf("missing bind addr")
	}
	if c.GatewayTimeout == 0 {
		return fmt.Errorf("missing gateway timeout")
	}
	if err := c.TLS.Validate(); err != nil {
		return fmt.Errorf("tls: %w", err)
	}

	return nil
}

func (c *ProxyConfig) RegisterFlags(fs *pflag.FlagSet) {
	fs.StringVar(
		&c.BindAddr,
		"proxy.bind-addr",
		":8000",
		`
The host/port to listen for incoming proxy HTTP requests.

If the host is unspecified it defaults to all listeners, such as
'--proxy.bind-addr :8000' will listen on '0.0.0.0:8000'`,
	)
	fs.StringVar(
		&c.AdvertiseAddr,
		"proxy.advertise-addr",
		"",
		`
Proxy listen address to advertise to other nodes in the cluster. This is the
address other nodes will used to forward proxy requests.

Such as if the listen address is ':8000', the advertised address may be
'10.26.104.45:8000' or 'node1.cluster:8000'.

By default, if the bind address includes an IP to bind to that will be used.
If the bind address does not include an IP (such as ':8000') the nodes
private IP will be used, such as a bind address of ':8000' may have an
advertise address of '10.26.104.14:8000'.`,
	)
	fs.DurationVar(
		&c.GatewayTimeout,
		"proxy.gateway-timeout",
		time.Second*15,
		`
The timeout when sending proxied requests to upstream listeners for forwarding
to other nodes in the cluster.

If the upstream does not respond within the given timeout a
'504 Gateway Timeout' is returned to the client.`,
	)

	c.TLS.RegisterFlags(fs, "proxy")
	c.HTTP.RegisterFlags(fs, "proxy")
}
