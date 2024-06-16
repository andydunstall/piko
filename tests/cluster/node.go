package cluster

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"sync"
	"time"

	pikogossip "github.com/andydunstall/piko/pkg/gossip"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/pkg/testutil"
	"github.com/andydunstall/piko/server/cluster"
	"github.com/andydunstall/piko/server/config"
	"github.com/andydunstall/piko/server/gossip"
	"github.com/andydunstall/piko/server/proxy"
	"github.com/andydunstall/piko/server/upstream"
)

type Node struct {
	nodeID string

	proxyLn        net.Listener
	upstreamLn     net.Listener
	gossipStreamLn net.Listener
	gossipPacketLn net.PacketConn

	proxyServer    *proxy.Server
	upstreamServer *upstream.Server
	gossiper       *gossip.Gossip

	clusterState *cluster.State

	tlsConfig  *tls.Config
	rootCAPool *x509.CertPool

	options options

	wg sync.WaitGroup
}

func NewNode(nodeID string, opts ...Option) (*Node, error) {
	options := options{
		tls:    false,
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

	gossipStreamLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("gossip listen: %w", err)
	}

	gossipPacketLn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   gossipStreamLn.Addr().(*net.TCPAddr).IP,
		Port: gossipStreamLn.Addr().(*net.TCPAddr).Port,
	})
	if err != nil {
		return nil, fmt.Errorf("gossip listen: %w", err)
	}

	var tlsConfig *tls.Config
	var rootCAPool *x509.CertPool
	if options.tls {
		pool, cert, err := testutil.LocalTLSServerCert()
		if err != nil {
			return nil, fmt.Errorf("tls cert: %w", err)
		}

		tlsConfig = &tls.Config{}
		tlsConfig.Certificates = []tls.Certificate{cert}
		rootCAPool = pool
	}

	clusterState := cluster.NewState(&cluster.Node{
		ID:        nodeID,
		ProxyAddr: proxyLn.Addr().String(),
		AdminAddr: "127.0.0.1:12345",
	}, options.logger)

	return &Node{
		nodeID:         nodeID,
		proxyLn:        proxyLn,
		upstreamLn:     upstreamLn,
		gossipStreamLn: gossipStreamLn,
		gossipPacketLn: gossipPacketLn,
		clusterState:   clusterState,
		tlsConfig:      tlsConfig,
		rootCAPool:     rootCAPool,
		options:        options,
	}, nil
}

func (n *Node) Start() {
	upstreams := upstream.NewLoadBalancedManager(n.clusterState)

	n.proxyServer = proxy.NewServer(
		upstreams,
		config.ProxyConfig{},
		nil,
		n.tlsConfig,
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
		n.tlsConfig,
		n.options.logger,
	)
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		_ = n.upstreamServer.Serve(n.upstreamLn)
	}()

	n.gossiper = gossip.NewGossip(
		n.clusterState,
		n.gossipStreamLn,
		n.gossipPacketLn,
		&pikogossip.Config{
			BindAddr:      n.gossipStreamLn.Addr().String(),
			AdvertiseAddr: n.gossipStreamLn.Addr().String(),
			Interval:      time.Millisecond * 10,
			MaxPacketSize: 1400,
		},
		n.options.logger,
	)
}

func (n *Node) Stop() {
	n.gossiper.Close()
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

func (n *Node) GossipAddr() string {
	return n.gossipStreamLn.Addr().String()
}

func (n *Node) ClusterState() *cluster.State {
	return n.clusterState
}

func (n *Node) RootCAPool() *x509.CertPool {
	return n.rootCAPool
}
