package cluster

import (
	"github.com/andydunstall/piko/pkg/log"
)

type options struct {
	nodes  int
	tls    bool
	logger log.Logger
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
