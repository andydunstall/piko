package piko

import (
	"crypto/tls"

	"github.com/andydunstall/piko/pkg/log"
)

type connectOptions struct {
	token     string
	url       string
	tlsConfig *tls.Config
	logger    log.Logger
}

type ConnectOption interface {
	apply(*connectOptions)
}

type tokenOption string

func (o tokenOption) apply(opts *connectOptions) {
	opts.token = string(o)
}

// WithToken configures the API key to authenticate the client.
func WithToken(key string) ConnectOption {
	return tokenOption(key)
}

type urlOption string

func (o urlOption) apply(opts *connectOptions) {
	opts.url = string(o)
}

// WithURL configures the Piko server URL. Such as
// 'https://piko.example.com:8001'.
func WithURL(url string) ConnectOption {
	return urlOption(url)
}

type tlsConfigOption struct {
	TLSConfig *tls.Config
}

func (o tlsConfigOption) apply(opts *connectOptions) {
	opts.tlsConfig = o.TLSConfig
}

// WithTLSConfig sets the TLS client configuration.
func WithTLSConfig(config *tls.Config) ConnectOption {
	return tlsConfigOption{TLSConfig: config}
}

type loggerOption struct {
	Logger log.Logger
}

func (o loggerOption) apply(opts *connectOptions) {
	opts.logger = o.Logger
}

// WithLogger configures the logger. Defaults to no output.
func WithLogger(logger log.Logger) ConnectOption {
	return loggerOption{Logger: logger}
}
