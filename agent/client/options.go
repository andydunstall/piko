package client

import (
	"crypto/tls"

	"github.com/andydunstall/piko/pkg/log"
)

type options struct {
	token       string
	upstreamURL string
	tlsConfig   *tls.Config
	logger      log.Logger
}

type Option interface {
	apply(*options)
}

type tokenOption string

func (o tokenOption) apply(opts *options) {
	opts.token = string(o)
}

// WithToken configures the API key to authenticate the client.
func WithToken(key string) Option {
	return tokenOption(key)
}

type upstreamURLOption string

func (o upstreamURLOption) apply(opts *options) {
	opts.upstreamURL = string(o)
}

// WithUpstreamURL configures the Piko server upsteam port URL. Such as
// 'https://piko.example.com:8001'.
func WithUpstreamURL(url string) Option {
	return upstreamURLOption(url)
}

type tlsConfigOption struct {
	TLSConfig *tls.Config
}

func (o tlsConfigOption) apply(opts *options) {
	opts.tlsConfig = o.TLSConfig
}

// WithTLSConfig sets the TLS client configuration.
func WithTLSConfig(config *tls.Config) Option {
	return tlsConfigOption{TLSConfig: config}
}

type loggerOption struct {
	Logger log.Logger
}

func (o loggerOption) apply(opts *options) {
	opts.logger = o.Logger
}

// WithLogger configures the logger. Defaults to no output.
func WithLogger(logger log.Logger) Option {
	return loggerOption{Logger: logger}
}
