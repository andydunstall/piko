package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/spf13/pflag"
)

type EndpointConfig struct {
	// ID is the endpoint ID to register.
	ID string `json:"id" yaml:"id"`

	// Addr is the address of the upstream service to forward to.
	Addr string `json:"addr" yaml:"addr"`

	// AccessLog indicates whether to log all incoming connections and requests
	// for the endpoint.
	AccessLog bool `json:"access_log" yaml:"access_log"`
}

func (c *EndpointConfig) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("missing id")
	}
	if c.Addr == "" {
		return fmt.Errorf("missing addr")
	}
	if _, ok := ParseAddrToURL(c.Addr); !ok {
		return fmt.Errorf("invalid addr")
	}
	return nil
}

type ConnectConfig struct {
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

func (c *ConnectConfig) Validate() error {
	if c.Timeout == 0 {
		return fmt.Errorf("missing timeout")
	}
	return nil
}

func (c *ConnectConfig) RegisterFlags(fs *pflag.FlagSet) {
	fs.DurationVar(
		&c.Timeout,
		"connect.timeout",
		time.Second*30,
		`
Timeout attempting to connect to the Piko server on boot. Note if the agent
is disconnected after the initial connection succeeds it will keep trying to
reconnect.`,
	)
}

type Config struct {
	Endpoints []EndpointConfig `json:"endpoints" yaml:"endpoints"`

	// Token is used to authenticate the agent with the server.
	Token string `json:"token" yaml:"token"`

	Connect ConnectConfig `json:"connect" yaml:"connect"`

	Log log.Config `json:"log" yaml:"log"`

	// GracePeriod is the duration to gracefully shutdown the agent. During
	// the grace period, listeners and idle connections are closed, then waits
	// for active requests to complete and closes their connections.
	GracePeriod time.Duration `json:"grace_period" yaml:"grace_period"`
}

func (c *Config) Validate() error {
	// Note don't validate the number of endpoints, as some commands don't
	// require any.
	for _, e := range c.Endpoints {
		if err := e.Validate(); err != nil {
			if e.ID != "" {
				return fmt.Errorf("endpoint: %s: %w", e.ID, err)
			}
			return fmt.Errorf("endpoint: %w", err)
		}
	}

	if err := c.Connect.Validate(); err != nil {
		return fmt.Errorf("connect: %w", err)
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
	fs.StringVar(
		&c.Token,
		"token",
		"",
		`
A token to authenticate the connection to Piko.`,
	)

	c.Connect.RegisterFlags(fs)

	c.Log.RegisterFlags(fs)

	fs.DurationVar(
		&c.GracePeriod,
		"grace-period",
		time.Minute,
		`
Maximum duration after a shutdown signal is received (SIGTERM or
SIGINT) to gracefully shutdown the server node before terminating.
This includes handling in-progress HTTP requests, gracefully closing
connections to upstream listeners, announcing to the cluster the node is
leaving...`,
	)
}

// ParseAddrToURL parses the given upstream address into a URL. Return false
// if the address is invalid.
//
// The addr may be either a full URL, a host and port or just a port.
func ParseAddrToURL(addr string) (*url.URL, bool) {
	// Port only.
	port, err := strconv.Atoi(addr)
	if err == nil && port >= 0 && port < 0xffff {
		return &url.URL{
			Scheme: "http",
			Host:   "localhost:" + addr,
		}, true
	}

	// Host and port.
	host, portStr, err := net.SplitHostPort(addr)
	if err == nil {
		return &url.URL{
			Scheme: "http",
			Host:   net.JoinHostPort(host, portStr),
		}, true
	}

	// URL.
	u, err := url.Parse(addr)
	if err == nil && u.Scheme != "" && u.Host != "" {
		return u, true
	}

	return nil, false
}
