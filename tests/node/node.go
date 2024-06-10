package node

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/server/cluster"
	"github.com/andydunstall/piko/server/config"
	"github.com/andydunstall/piko/server/proxy"
	"github.com/andydunstall/piko/server/upstream"
)

type options struct {
	logger log.Logger
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

type Node struct {
	nodeID string

	proxyLn    net.Listener
	upstreamLn net.Listener

	proxyServer    *proxy.Server
	upstreamServer *upstream.Server

	options options

	wg sync.WaitGroup
}

func New(opts ...Option) (*Node, error) {
	options := options{
		logger: log.NewNopLogger(),
	}
	for _, o := range opts {
		o.apply(&options)
	}

	proxyLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("proxy listen: %w", err)
	}

	upstreamLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("upstream listen: %w", err)
	}

	return &Node{
		nodeID:     "my-node",
		proxyLn:    proxyLn,
		upstreamLn: upstreamLn,
		options:    options,
	}, nil
}

func (n *Node) Start() {
	clusterState := cluster.NewState(&cluster.Node{
		ID:        n.nodeID,
		ProxyAddr: n.proxyLn.Addr().String(),
	}, n.options.logger)

	upstreams := upstream.NewLoadBalancedManager(clusterState)

	n.proxyServer = proxy.NewServer(
		upstreams,
		config.ProxyConfig{},
		nil,
		nil,
		n.options.logger,
	)
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		_ = n.proxyServer.Serve(n.proxyLn)
	}()

	n.upstreamServer = upstream.NewServer(
		upstreams,
		nil,
		nil,
		n.options.logger,
	)
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		_ = n.upstreamServer.Serve(n.upstreamLn)
	}()
}

func (n *Node) Stop() {
	n.upstreamServer.Shutdown(context.Background())
	n.proxyServer.Shutdown(context.Background())
	n.wg.Wait()
}

func (n *Node) ProxyAddr() string {
	return n.proxyLn.Addr().String()
}

func (n *Node) UpstreamAddr() string {
	return n.upstreamLn.Addr().String()
}