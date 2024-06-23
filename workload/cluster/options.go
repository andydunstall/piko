package cluster

import (
	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/server/auth"
)

type options struct {
	nodes          int
	tls            bool
	verifierConfig *auth.JWTVerifierConfig
	logger         log.Logger
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

type verifierConfigOption struct {
	VerifierConfig *auth.JWTVerifierConfig
}

func (o verifierConfigOption) apply(opts *options) {
	opts.verifierConfig = o.VerifierConfig
}

// WithVerifierConfig configures the upstream JWT verification config.
func WithVerifierConfig(config *auth.JWTVerifierConfig) Option {
	return verifierConfigOption{VerifierConfig: config}
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
