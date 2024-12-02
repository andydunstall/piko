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
