package proxy

import (
	"github.com/andydunstall/pico/pkg/forwarder"
	"github.com/andydunstall/pico/pkg/log"
)

type options struct {
	forwarder forwarder.Forwarder
	logger    log.Logger
}

type Option interface {
	apply(*options)
}

func defaultOptions() options {
	return options{
		forwarder: forwarder.NewForwarder(),
		logger:    log.NewNopLogger(),
	}
}

type forwarderOption struct {
	Forwarder forwarder.Forwarder
}

func (o forwarderOption) apply(opts *options) {
	opts.forwarder = o.Forwarder
}

func WithForwarder(f forwarder.Forwarder) Option {
	return forwarderOption{Forwarder: f}
}

type loggerOption struct {
	Logger log.Logger
}

func (o loggerOption) apply(opts *options) {
	opts.logger = o.Logger
}

func WithLogger(l log.Logger) Option {
	return loggerOption{Logger: l}
}
