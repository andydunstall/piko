package cluster

import (
	"github.com/dragonflydb/piko/pkg/auth"
	"github.com/dragonflydb/piko/pkg/log"
)

type options struct {
	join       []string
	authConfig auth.Config
	tls        bool
	logger     log.Logger
}

type joinOption struct {
	Join []string
}

func (o joinOption) apply(opts *options) {
	opts.join = o.Join
}

// WithJoin configures the nodes to join.
func WithJoin(join []string) Option {
	return joinOption{Join: join}
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

type tlsOption bool

func (o tlsOption) apply(opts *options) {
	opts.tls = bool(o)
}

// WithTLS configures the node ports to use TLS.
func WithTLS(tls bool) Option {
	return tlsOption(tls)
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
