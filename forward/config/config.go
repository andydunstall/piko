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

	"github.com/spf13/pflag"

	"github.com/dragonflydb/piko/pkg/log"
)

type PortConfig struct {
	// Addr is the address to listen on.
	Addr string `json:"addr" yaml:"addr"`

	// EndpointID is the endpoint ID to connect to.
	EndpointID string `json:"endpoint_id" yaml:"endpoint_id"`
}

// Host parses the given upstream address into a host and port. Return false if
// the address is invalid.
//
// The addr may be either a a host and port or just a port.
func (c *PortConfig) Host() (string, bool) {
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

func (c *PortConfig) Validate() error {
	if c.Addr == "" {
		return fmt.Errorf("missing addr")
	}
	if _, ok := c.Host(); !ok {
		return fmt.Errorf("invalid addr")
	}
	if c.EndpointID == "" {
		return fmt.Errorf("missing endpoint id")
	}
	return nil
}

type TLSConfig struct {
	// Cert contains a path to the PEM encoded certificate to present to
	// the server (optional).
	Cert string `json:"cert" yaml:"cert"`

	// Key contains a path to the PEM encoded private key (optional).
	Key string `json:"key" yaml:"key"`

	// RootCAs contains a path to root certificate authorities to validate
	// the TLS connection to the Piko server.
	//
	// Defaults to using the host root CAs.
	RootCAs string `json:"root_cas" yaml:"root_cas"`

	// InsecureSkipVerify configures the client to accept any certificate
	// presented by the server and any host name in that certificate.
	//
	// See https://pkg.go.dev/crypto/tls#Config.
	InsecureSkipVerify bool `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`
}

func (c *TLSConfig) Validate() error {
	if c.Cert != "" && c.Key == "" {
		return fmt.Errorf("missing key")
	}

	_, err := c.Load()
	return err
}

func (c *TLSConfig) RegisterFlags(fs *pflag.FlagSet, prefix string) {
	prefix = prefix + ".tls."
	fs.StringVar(
		&c.Cert,
		prefix+"cert",
		c.Cert,
		`
Path to the PEM encoded certificate file to present to the server.`,
	)
	fs.StringVar(
		&c.Key,
		prefix+"key",
		c.Key,
		`
Path to the PEM encoded key file.`,
	)

	fs.StringVar(
		&c.RootCAs,
		prefix+"root-cas",
		c.RootCAs,
		`
A path to a certificate PEM file containing root certificiate authorities to
validate the TLS connection to the Piko server.

Defaults to using the host root CAs.`,
	)

	fs.BoolVar(
		&c.InsecureSkipVerify,
		prefix+"insecure-skip-verify",
		c.InsecureSkipVerify,
		`
Configures the client to accept any certificate presented by the server and any
host name in that certificate.`,
	)
}

func (c *TLSConfig) Load() (*tls.Config, error) {
	tlsConfig := &tls.Config{}

	if c.Cert != "" {
		cert, err := tls.LoadX509KeyPair(c.Cert, c.Key)
		if err != nil {
			return nil, fmt.Errorf("load key pair: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if c.RootCAs != "" {
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
	}

	tlsConfig.InsecureSkipVerify = c.InsecureSkipVerify

	return tlsConfig, nil
}

type ConnectConfig struct {
	// URL is the Piko server URL to connect to.
	URL string

	// Token is a token to authenticate with the Piko server.
	Token string

	// Timeout is the timeout attempting to connect to the Piko server.
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
	if err := c.TLS.Validate(); err != nil {
		return fmt.Errorf("tls: %w", err)
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
Piko server 'proxy' port.`,
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
Timeout attempting to connect to the Piko server.`,
	)

	c.TLS.RegisterFlags(fs, "connect")
}

type Config struct {
	Ports []PortConfig `json:"ports" yaml:"ports"`

	Connect ConnectConfig `json:"connect" yaml:"connect"`

	Log log.Config `json:"log" yaml:"log"`
}

func Default() *Config {
	return &Config{
		Connect: ConnectConfig{
			URL:     "http://localhost:8000",
			Timeout: time.Second * 30,
		},
		Log: log.Config{
			Level: "info",
		},
	}
}

func (c *Config) Validate() error {
	// Note don't validate the number of ports, as some commands don't
	// require any.
	for _, e := range c.Ports {
		if err := e.Validate(); err != nil {
			if e.EndpointID != "" {
				return fmt.Errorf("port: %s: %w", e.EndpointID, err)
			}
			return fmt.Errorf("port: %w", err)
		}
	}

	if err := c.Connect.Validate(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	if err := c.Log.Validate(); err != nil {
		return fmt.Errorf("log: %w", err)
	}

	return nil
}

func (c *Config) RegisterFlags(fs *pflag.FlagSet) {
	c.Connect.RegisterFlags(fs)
	c.Log.RegisterFlags(fs)
}
