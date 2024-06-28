package cluster

import (
	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/server/auth"
)

type options struct {
	nodes      int
	authConfig auth.Config
	tls        bool
	logger     log.Logger
}

type nodesOption int

func (o nodesOption) apply(opts *options) {
	opts.nodes = int(o)
}

func WithNodes(nodes int) Option {
	return nodesOption(nodes)
}

type tlsOption bool

func (o tlsOption) apply(opts *options) {
	opts.tls = bool(o)
}

// WithTLS configures the node ports to use TLS.
func WithTLS(tls bool) Option {
	return tlsOption(tls)
}

type authConfigOption struct {
	AuthConfig auth.Config
}

func (o authConfigOption) apply(opts *options) {
	opts.authConfig = o.AuthConfig
}

// WithAuthConfig configures the upstream authentication config.
func WithAuthConfig(config auth.Config) Option {
	return authConfigOption{AuthConfig: config}
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

type Option interface {
	apply(*options)
}
