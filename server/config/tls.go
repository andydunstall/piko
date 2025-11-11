package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/spf13/pflag"
)

type TLSConfig struct {
	Cert      string `json:"cert" yaml:"cert"`
	Key       string `json:"key" yaml:"key"`
	ClientCAs string `json:"client_cas" yaml:"client_cas"`

	Client ClientTLSConfig `json:"client" yaml:"client"`
}

func (c *TLSConfig) Validate() error {
	if !c.enabled() {
		return nil
	}

	if c.Cert == "" {
		return fmt.Errorf("missing cert")
	}
	if c.Key == "" {
		return fmt.Errorf("missing key")
	}
	return nil
}

func (c *TLSConfig) RegisterFlags(fs *pflag.FlagSet, prefix string) {
	prefix += ".tls."

	fs.StringVar(
		&c.Cert,
		prefix+"cert",
		c.Cert,
		`
Path to the PEM encoded certificate file.

If given the server will listen on TLS`,
	)
	fs.StringVar(
		&c.Key,
		prefix+"key",
		c.Key,
		`
Path to the PEM encoded key file.`,
	)
	fs.StringVar(
		&c.ClientCAs,
		prefix+"client-cas",
		c.ClientCAs,
		`
A path to a certificate PEM file containing client certificiate authorities to
verify the client certificates.

When set the client must set a valid certificate during the TLS handshake.`,
	)

	c.Client.RegisterFlags(fs, prefix[:len(prefix)-1])
}

func (c *TLSConfig) Load() (*tls.Config, error) {
	if !c.enabled() {
		return nil, nil
	}

	tlsConfig := &tls.Config{}
	cert, err := tls.LoadX509KeyPair(c.Cert, c.Key)
	if err != nil {
		return nil, fmt.Errorf("load key pair: %w", err)
	}
	tlsConfig.Certificates = []tls.Certificate{cert}

	if c.ClientCAs != "" {
		caCert, err := os.ReadFile(c.ClientCAs)
		if err != nil {
			return nil, fmt.Errorf("open client cas: %s: %w", c.ClientCAs, err)
		}
		caCertPool := x509.NewCertPool()
		ok := caCertPool.AppendCertsFromPEM(caCert)
		if !ok {
			return nil, fmt.Errorf("parse client cas: %s: %w", c.ClientCAs, err)
		}
		tlsConfig.ClientCAs = caCertPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tlsConfig, nil
}

func (c *TLSConfig) enabled() bool {
	return c.Cert != "" || c.Key != ""
}

type ClientTLSConfig struct {
	// Cert contains a path to the PEM encoded certificate to present to
	// the server (optional).
	Cert string `json:"cert" yaml:"cert"`

	// Key contains a path to the PEM encoded private key (optional).
	Key string `json:"key" yaml:"key"`

	// RootCAs contains a path to root certificate authorities to validate
	// the TLS connection between the Piko nodes.
	//
	// Defaults to using the host root CAs.
	RootCAs string `json:"root_cas" yaml:"root_cas"`

	// InsecureSkipVerify configures the client to accept any certificate
	// presented by the server and any host name in that certificate.
	//
	// See https://pkg.go.dev/crypto/tls#Config.
	InsecureSkipVerify bool `json:"insecure_skip_verify" yaml:"insecure_skip_verify"`

	// ServerName is used to verify the hostname on the returned certificate.
	// If given, this hostname overrides the hostname used when dialing the
	// connection.
	//
	// See https://pkg.go.dev/crypto/tls#Config.
	ServerName string `json:"server_name" yaml:"server_name"`
}

func (c *ClientTLSConfig) Validate() error {
	if c.Cert != "" && c.Key == "" {
		return fmt.Errorf("missing key")
	}

	_, err := c.Load()
	return err
}

func (c *ClientTLSConfig) RegisterFlags(fs *pflag.FlagSet, prefix string) {
	prefix = prefix + ".client."
	fs.StringVar(
		&c.Cert,
		prefix+"cert",
		c.Cert,
		`
Path to the PEM encoded certificate file to present to the server.

Used for node-to-node communication between Piko servers if mTLS is expected.`,
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

	fs.StringVar(
		&c.ServerName,
		prefix+"server-name",
		c.ServerName,
		`
Server name to use for certificate verification. This overrides the hostname used
when dialing (e.g., when connecting to nodes by IP address but the certificate
is issued for a hostname).

If not set, the hostname from the dial address is used for verification.`,
	)
}

func (c *ClientTLSConfig) Load() (*tls.Config, error) {
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

	if c.ServerName != "" {
		tlsConfig.ServerName = c.ServerName
	}

	return tlsConfig, nil
}
