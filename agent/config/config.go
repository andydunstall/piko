package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/spf13/pflag"
)

type ListenerProtocol string

const (
	ListenerProtocolHTTP ListenerProtocol = "http"
	ListenerProtocolTCP  ListenerProtocol = "tcp"
)

type ListenerConfig struct {
	// EndpointID is the endpoint ID to register.
	EndpointID string `json:"endpoint_id" yaml:"endpoint_id"`

	// Addr is the address of the upstream service to forward to.
	Addr string `json:"addr" yaml:"addr"`

	// Protocol is the protocol to listen on. Supports "http" and "tcp".
	// Defaults to "http".
	Protocol ListenerProtocol `json:"protocol" yaml:"protocol"`

	// AccessLog indicates whether to log all incoming connections and requests
	// for the endpoint.
	AccessLog bool `json:"access_log" yaml:"access_log"`

	// Timeout is the timeout to forward incoming requests to the upstream.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// Host parses the given upstream address into a host and port. Return false if
// the address is invalid.
//
// The addr may be either a a host and port or just a port.
func (c *ListenerConfig) Host() (string, bool) {
	// Port only.
	port, err := strconv.Atoi(c.Addr)
	if err == nil && port >= 0 && port < 0xffff {
		return "localhost:" + c.Addr, true
	}

	// Host and port.
	_, _, err = net.SplitHostPort(c.Addr)
	if err == nil {
		return c.Addr, true
	}

	return "", false
}

// URL parses the given upstream address into a URL. Return false if the
// address is invalid.
//
// The addr may be either a full URL, a host and port or just a port.
func (c *ListenerConfig) URL() (*url.URL, bool) {
	// Port only.
	port, err := strconv.Atoi(c.Addr)
	if err == nil && port >= 0 && port < 0xffff {
		return &url.URL{
			Scheme: "http",
			Host:   "localhost:" + c.Addr,
		}, true
	}

	// Host and port.
	host, portStr, err := net.SplitHostPort(c.Addr)
	if err == nil {
		return &url.URL{
			Scheme: "http",
			Host:   net.JoinHostPort(host, portStr),
		}, true
	}

	// URL.
	u, err := url.Parse(c.Addr)
	if err == nil && u.Scheme != "" && u.Host != "" {
		return u, true
	}

	return nil, false
}

func (c *ListenerConfig) Validate() error {
	if c.EndpointID == "" {
		return fmt.Errorf("missing endpoint id")
	}
	if c.Addr == "" {
		return fmt.Errorf("missing addr")
	}
	if c.Protocol == "" || c.Protocol == ListenerProtocolHTTP {
		if _, ok := c.URL(); !ok {
			return fmt.Errorf("invalid addr")
		}
	} else if c.Protocol != ListenerProtocolTCP {
		if _, ok := c.Host(); !ok {
			return fmt.Errorf("invalid addr")
		}
	} else {
		return fmt.Errorf("unsupported protocol")
	}
	if c.Timeout == 0 {
		return fmt.Errorf("missing timeout")
	}
	return nil
}

type TLSConfig struct {
	// RootCAs contains a path to root certificate authorities to validate
	// the TLS connection to the Piko server.
	//
	// Defaults to using the host root CAs.
	RootCAs string `json:"root_cas" yaml:"root_cas"`
}

func (c *TLSConfig) RegisterFlags(fs *pflag.FlagSet, prefix string) {
	prefix = prefix + ".tls."
	fs.StringVar(
		&c.RootCAs,
		prefix+"root-cas",
		c.RootCAs,
		`
A path to a certificate PEM file containing root certificiate authorities to
validate the TLS connection to the Piko server.

Defaults to using the host root CAs.`,
	)
}

func (c *TLSConfig) Load() (*tls.Config, error) {
	if c.RootCAs == "" {
		return nil, nil
	}

	tlsConfig := &tls.Config{}

	caCert, err := os.ReadFile(c.RootCAs)
	if err != nil {
		return nil, fmt.Errorf("open root cas: %s: %w", c.RootCAs, err)
	}
	caCertPool := x509.NewCertPool()
	ok := caCertPool.AppendCertsFromPEM(caCert)
	if !ok {
		return nil, fmt.Errorf("parse root cas: %s: %w", c.RootCAs, err)
	}
	tlsConfig.RootCAs = caCertPool

	return tlsConfig, nil
}

type ConnectConfig struct {
	// URL is the Piko server URL to connect to.
	URL string

	// Token is a token to authenticate with the Piko server.
	Token string

	// Timeout is the timeout attempting to connect to the Piko server on
	// boot.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	TLS TLSConfig `json:"tls" yaml:"tls"`
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
The Piko server URL to connect to. Note this must be configured to use the
Piko server 'upstream' port.`,
	)

	fs.StringVar(
		&c.Token,
		"connect.token",
		c.Token,
		`
Token is a token to authenticate with the Piko server.`,
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

	c.TLS.RegisterFlags(fs, "connect")
}

type ServerConfig struct {
	// BindAddr is the address to bind to listen for incoming HTTP connections.
	BindAddr string `json:"bind_addr" yaml:"bind_addr"`
}

func (c *ServerConfig) Validate() error {
	if c.BindAddr == "" {
		return fmt.Errorf("missing bind addr")
	}
	return nil
}

func (c *ServerConfig) RegisterFlags(fs *pflag.FlagSet) {
	fs.StringVar(
		&c.BindAddr,
		"server.bind-addr",
		c.BindAddr,
		`
The host/port to bind the server to.

If the host is unspecified it defaults to all listeners, such as
'--server.bind-addr :5000' will listen on '0.0.0.0:5000'.`,
	)
}

type Config struct {
	Listeners []ListenerConfig `json:"listeners" yaml:"listeners"`

	Connect ConnectConfig `json:"connect" yaml:"connect"`

	Server ServerConfig `json:"server" yaml:"server"`

	Log log.Config `json:"log" yaml:"log"`

	// GracePeriod is the duration to gracefully shutdown the agent. During
	// the grace period, listeners and idle connections are closed, then waits
	// for active requests to complete and closes their connections.
	GracePeriod time.Duration `json:"grace_period" yaml:"grace_period"`
}

func Default() *Config {
	return &Config{
		Connect: ConnectConfig{
			URL:     "http://localhost:8001",
			Timeout: time.Second * 30,
		},
		Server: ServerConfig{
			BindAddr: ":5000",
		},
		Log: log.Config{
			Level: "info",
		},
		GracePeriod: time.Minute,
	}
}

func (c *Config) Validate() error {
	// Note don't validate the number of listeners, as some commands don't
	// require any.
	for _, e := range c.Listeners {
		if err := e.Validate(); err != nil {
			if e.EndpointID != "" {
				return fmt.Errorf("listener: %s: %w", e.EndpointID, err)
			}
			return fmt.Errorf("listener: %w", err)
		}
	}

	if err := c.Connect.Validate(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server: %w", err)
	}

	if err := c.Log.Validate(); err != nil {
		return fmt.Errorf("log: %w", err)
	}

	if c.GracePeriod == 0 {
		return fmt.Errorf("missing grace period")
	}

	return nil
}

func (c *Config) RegisterFlags(fs *pflag.FlagSet) {
	c.Connect.RegisterFlags(fs)
	c.Server.RegisterFlags(fs)
	c.Log.RegisterFlags(fs)

	fs.DurationVar(
		&c.GracePeriod,
		"grace-period",
		c.GracePeriod,
		`
Maximum duration after a shutdown signal is received (SIGTERM or
SIGINT) to gracefully shutdown each listener.
`,
	)

}
