package config

import (
	"crypto/tls"
	"fmt"

	"github.com/spf13/pflag"
)

type TLSConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Cert    string `json:"cert" yaml:"cert"`
	Key     string `json:"key" yaml:"key"`
}

func (c *TLSConfig) Validate() error {
	if !c.Enabled {
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

	fs.BoolVar(
		&c.Enabled,
		prefix+"enabled",
		c.Enabled,
		`
Whether to enable TLS on the listener.

If enabled must configure the cert and key.`,
	)
	fs.StringVar(
		&c.Cert,
		prefix+"cert",
		c.Cert,
		`
Path to the PEM encoded certificate file.`,
	)
	fs.StringVar(
		&c.Key,
		prefix+"key",
		c.Key,
		`
Path to the PEM encoded key file.`,
	)
}

func (c *TLSConfig) Load() (*tls.Config, error) {
	if !c.Enabled {
		return nil, nil
	}

	tlsConfig := &tls.Config{}
	cert, err := tls.LoadX509KeyPair(c.Cert, c.Key)
	if err != nil {
		return nil, fmt.Errorf("load key pair: %w", err)
	}
	tlsConfig.Certificates = []tls.Certificate{cert}

	return tlsConfig, nil
}
