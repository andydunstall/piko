package server

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-sockaddr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"go.uber.org/atomic"
	"go.uber.org/zap"

	"github.com/andydunstall/piko/pkg/auth"
	"github.com/andydunstall/piko/pkg/build"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/server/admin"
	"github.com/andydunstall/piko/server/cluster"
	"github.com/andydunstall/piko/server/config"
	"github.com/andydunstall/piko/server/gossip"
	"github.com/andydunstall/piko/server/proxy"
	"github.com/andydunstall/piko/server/upstream"
	"github.com/andydunstall/piko/server/usage"
)

// Server is a Piko server node.
type Server struct {
	clusterState *cluster.State

	proxyLn     net.Listener
	proxyServer *proxy.Server

	upstreamLn     net.Listener
	upstreamServer *upstream.Server

	rebalanceCtx    context.Context
	rebalanceCancel context.CancelFunc

	adminLn     net.Listener
	adminServer *admin.Server

	gossiper *gossip.Gossip

	reporter *usage.Reporter

	conf *config.Config

	// fatalCh triggers a shutdown when a fatal error occurs.
	fatalCh chan struct{}
	// fatalOnce ensures only a single goroutine closes fatalCh.
	fatalOnce sync.Once

	// shutdown indicates whether a server shutdown has been requested.
	shutdown *atomic.Bool

	// wg waits for background goroutines to exit.
	wg sync.WaitGroup

	registry *prometheus.Registry

	logger log.Logger
}

// NewServer creates a server node with the given configuration.
//
// This loads the server configuration and open the server TCP listens, though
// won't start accepting traffic.
func NewServer(conf *config.Config, logger log.Logger) (*Server, error) {
	logger = logger.WithSubsystem("server")

	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewGoCollector())

	s := &Server{
		fatalCh:  make(chan struct{}),
		shutdown: atomic.NewBool(false),
		conf:     conf,
		registry: registry,
		logger:   logger,
	}

	// Proxy listener.

	proxyLn, err := s.proxyListen()
	if err != nil {
		return nil, fmt.Errorf("proxy listen: %w", err)
	}
	s.proxyLn = proxyLn

	// Upstream listener.

	upstreamLn, err := s.upstreamListen()
	if err != nil {
		return nil, fmt.Errorf("upstream listen: %w", err)
	}
	s.upstreamLn = upstreamLn

	rebalanceCtx, rebalanceCancel := context.WithCancel(context.Background())
	s.rebalanceCtx = rebalanceCtx
	s.rebalanceCancel = rebalanceCancel

	// Admin listener.

	adminLn, err := s.adminListen()
	if err != nil {
		return nil, fmt.Errorf("admin listen: %w", err)
	}
	s.adminLn = adminLn

	// Cluster.

	s.clusterState = cluster.NewState(&cluster.Node{
		ID:        conf.Cluster.NodeID,
		ProxyAddr: conf.Proxy.AdvertiseAddr,
		AdminAddr: conf.Admin.AdvertiseAddr,
	}, logger)
	s.clusterState.Metrics().Register(registry)

	upstreams := upstream.NewLoadBalancedManager(s.clusterState)
	upstreams.Metrics().Register(registry)

	// Proxy server.

	var proxyVerifier *auth.MultiTenantVerifier
	if conf.Proxy.Auth.Enabled() {
		verifierConf, err := conf.Proxy.Auth.Load()
		if err != nil {
			return nil, fmt.Errorf("proxy: load auth: %w", err)
		}
		proxyVerifier = auth.NewMultiTenantVerifier(
			auth.NewJWTVerifier(verifierConf), nil,
		)
	}
	proxyTLSConfig, err := conf.Proxy.TLS.Load()
	if err != nil {
		return nil, fmt.Errorf("proxy tls: %w", err)
	}
	s.proxyServer = proxy.NewServer(
		upstreams,
		conf.Proxy,
		registry,
		proxyVerifier,
		proxyTLSConfig,
		logger,
	)

	// Upstream server.

	var upstreamVerifier *auth.MultiTenantVerifier
	if conf.Upstream.Auth.Enabled() || len(conf.Upstream.Tenants) > 0 {
		verifierConf, err := conf.Upstream.Auth.Load()
		if err != nil {
			return nil, fmt.Errorf("upstream: load auth: %w", err)
		}
		defaultUpstreamVerifier := auth.NewJWTVerifier(verifierConf)

		upstreamTenantVerifiers := make(map[string]auth.Verifier)
		for _, tenantConf := range conf.Upstream.Tenants {
			tenantVerifierConf, err := tenantConf.Auth.Load()
			if err != nil {
				return nil, fmt.Errorf("upstream: tenant %s: load auth: %w", tenantConf.ID, err)
			}
			upstreamTenantVerifiers[tenantConf.ID] = auth.NewJWTVerifier(tenantVerifierConf)
		}

		upstreamVerifier = auth.NewMultiTenantVerifier(
			defaultUpstreamVerifier, upstreamTenantVerifiers,
		)
	}
	upstreamTLSConfig, err := conf.Upstream.TLS.Load()
	if err != nil {
		return nil, fmt.Errorf("upstream: load tls: %w", err)
	}
	s.upstreamServer = upstream.NewServer(
		upstreams,
		upstreamVerifier,
		upstreamTLSConfig,
		s.clusterState,
		conf.Upstream,
		logger,
	)

	// Admin server.

	var adminVerifier *auth.MultiTenantVerifier
	if conf.Admin.Auth.Enabled() {
		verifierConf, err := conf.Admin.Auth.Load()
		if err != nil {
			return nil, fmt.Errorf("admin: load auth: %w", err)
		}
		adminVerifier = auth.NewMultiTenantVerifier(
			auth.NewJWTVerifier(verifierConf), nil,
		)
	}
	adminTLSConfig, err := conf.Admin.TLS.Load()
	if err != nil {
		return nil, fmt.Errorf("admin tls: %w", err)
	}
	s.adminServer = admin.NewServer(
		s.clusterState,
		registry,
		adminVerifier,
		adminTLSConfig,
		logger,
	)
	s.adminServer.AddStatus("/upstream", upstream.NewStatus(upstreams))
	s.adminServer.AddStatus("/cluster", cluster.NewStatus(s.clusterState))

	// Usage reporting.

	s.reporter = usage.NewReporter(upstreams.Usage(), logger)

	return s, nil
}

// Start starts the Piko node.
func (s *Server) Start() error {
	s.logger.Info(
		"starting piko server",
		zap.String("node-id", s.conf.Cluster.NodeID),
		zap.String("version", build.Version),
	)
	s.logger.Debug("piko config", zap.Any("config", s.conf))

	// Start the admin server. This includes a '/ready' route that will be
	// false until the server has started.
	s.startAdminServer()

	// Usage reporting.

	if !s.conf.Usage.Disable {
		s.startUsageReporting()
	}

	// Start listening for gossip traffic for other node. This won't actively
	// attempt to join the cluster yet, though accepts other nodes attempting
	// to join us.
	//
	// As we haven't started the upstream server, the node won't have any
	// upstream connections so won't receive any proxy requests from other
	// nodes in the cluster.
	if err := s.startGossip(); err != nil {
		return fmt.Errorf("gossip: %w", err)
	}

	// Attempt to join the cluster.
	//
	// When running on Kubernetes using a headless DNS record for service
	// discovery, if this is the first pod in the service DNS resolution will
	// fail as the pod isn't ready.
	//
	// Therefore this will attempt to join once, but continue booting if we
	// fail to join the cluster, then try again once this pod is ready.
	nodeIDs, err := s.gossiper.JoinOnBoot(s.conf.Cluster.Join)
	if err != nil {
		s.logger.Warn("failed to join cluster", zap.Error(err))
	}
	if len(nodeIDs) > 0 {
		s.logger.Info("joined cluster", zap.Strings("node-ids", nodeIDs))
	}

	// Now we've attempted to join the cluster, we can start the upstream
	// server and proxy server.
	s.startUpstreamServer()
	s.startProxyServer()

	// Now we've joined the cluster and started all servers, mark the server
	// as ready to begin accepting requests.
	s.adminServer.SetReady(true)

	// If we couldn't join the cluster on the first attempt, now the node is
	// ready we can retry.
	if len(nodeIDs) == 0 {
		joinCtx, cancel := context.WithTimeout(
			context.Background(), s.conf.Cluster.JoinTimeout,
		)
		defer cancel()

		nodeIDs, err := s.gossiper.JoinOnStartup(joinCtx, s.conf.Cluster.Join)
		if err != nil {
			if s.conf.Cluster.AbortIfJoinFails {
				return fmt.Errorf("cluster join: %w", err)
			}
			s.logger.Warn("failed to join cluster", zap.Error(err))
		}
		if len(nodeIDs) > 0 {
			s.logger.Info("joined cluster", zap.Strings("node-ids", nodeIDs))
		}
	}

	return nil
}

// Shutdown gracefully stops the server node.
func (s *Server) Shutdown() {
	if !s.shutdown.CompareAndSwap(false, true) {
		s.logger.Warn("server already being shutdown")
	}

	s.logger.Info("starting shutdown")

	ctx, cancel := context.WithTimeout(
		context.Background(), s.conf.GracePeriod,
	)
	defer cancel()

	// Set the ready to false to stop incoming traffic.
	s.adminServer.SetReady(false)

	// Shutdown the upstream server and close active upstream connections.
	//
	// We close upstream connections first since as long as we have upstream
	// connections, we'll receive requests from other nodes in the cluster
	// routing requests to our upstreams.
	//
	// We could still get requests from the proxy server but they'll be routed
	// to other nodes.
	s.shutdownUpstreamServer(ctx)

	// Now we no longer have any connected upstreams, we'll no longer get
	// requests from other cluster nodes so can shut down the proxy server.
	s.shutdownProxyServer(ctx)

	// Leave the cluster.
	if err := s.gossiper.Leave(ctx); err != nil {
		s.logger.Warn("failed to leave cluster", zap.Error(err))
	} else {
		s.logger.Info("left cluster")
	}

	// Now we've left the cluster we can safely close the gossip listeners.
	s.gossiper.Close()

	s.shutdownAdminServer(ctx)

	s.shutdownUsageReporting()

	s.wg.Wait()

	s.logger.Info("shutdown complete")
}

func (s *Server) Config() *config.Config {
	return s.conf
}

func (s *Server) ClusterState() *cluster.State {
	return s.clusterState
}

// Wait waits for the server to be shutdown, either due to the given context
// being cancelled or a fatal error in the server. Returns whether the server
// exited due to being gracefully shutdown or a fatal error.
func (s *Server) Wait(ctx context.Context) bool {
	ok := true
	select {
	case <-ctx.Done():
	case <-s.fatalCh:
		ok = false
	}

	s.Shutdown()
	return ok
}

func (s *Server) startGossip() error {
	gossipStreamLn, err := net.Listen("tcp", s.conf.Cluster.Gossip.BindAddr)
	if err != nil {
		return fmt.Errorf("listen: %s: %w", s.conf.Cluster.Gossip.BindAddr, err)
	}

	gossipPacketLn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   gossipStreamLn.Addr().(*net.TCPAddr).IP,
		Port: gossipStreamLn.Addr().(*net.TCPAddr).Port,
	})
	if err != nil {
		return fmt.Errorf("listen: %s: %w", s.conf.Cluster.Gossip.BindAddr, err)
	}

	if s.conf.Cluster.Gossip.AdvertiseAddr == "" {
		advertiseAddr, err := advertiseAddrFromListenAddr(
			gossipStreamLn.Addr().String(),
		)
		if err != nil {
			// Should never happen.
			panic("invalid listen address: " + err.Error())
		}
		s.conf.Cluster.Gossip.AdvertiseAddr = advertiseAddr
	}

	s.gossiper = gossip.NewGossip(
		s.clusterState,
		gossipStreamLn,
		gossipPacketLn,
		&s.conf.Cluster.Gossip,
		s.logger,
	)
	s.gossiper.Metrics().Register(s.registry)
	s.adminServer.AddStatus("/gossip", gossip.NewStatus(s.gossiper))

	return nil
}

func (s *Server) startProxyServer() {
	s.runGoroutine(func() {
		if err := s.proxyServer.Serve(s.proxyLn); err != nil {
			s.logger.Error("failed to run proxy server", zap.Error(err))
		}
	})
}

func (s *Server) startUpstreamServer() {
	s.runGoroutine(func() {
		if err := s.upstreamServer.Serve(s.upstreamLn); err != nil {
			s.logger.Error("failed to run upstream server", zap.Error(err))
		}
	})
	if s.conf.Upstream.Rebalance.Threshold != 0 {
		s.runGoroutine(func() {
			s.upstreamRebalance()
		})
	}
}

func (s *Server) startAdminServer() {
	s.runGoroutine(func() {
		if err := s.adminServer.Serve(s.adminLn); err != nil {
			s.logger.Error("failed to run admin server", zap.Error(err))
		}
	})
}

func (s *Server) startUsageReporting() {
	s.runGoroutine(func() {
		s.reporter.Start()
	})
}

func (s *Server) shutdownProxyServer(ctx context.Context) {
	if err := s.proxyServer.Shutdown(ctx); err != nil {
		s.logger.Error("failed to shutdown proxy server", zap.Error(err))
	}
	s.logger.Info("shutdown proxy server")
}

func (s *Server) shutdownUsageReporting() {
	s.reporter.Stop()
}

func (s *Server) shutdownUpstreamServer(ctx context.Context) {
	s.rebalanceCancel()
	if err := s.upstreamServer.Shutdown(ctx); err != nil {
		s.logger.Error("failed to shutdown upstream server", zap.Error(err))
	}
	s.logger.Info("shutdown upstream server")
}

func (s *Server) shutdownAdminServer(ctx context.Context) {
	if err := s.adminServer.Shutdown(ctx); err != nil {
		s.logger.Error("failed to shutdown admin server", zap.Error(err))
	}
	s.logger.Info("shutdown admin server")
}

func (s *Server) proxyListen() (net.Listener, error) {
	ln, err := net.Listen("tcp", s.conf.Proxy.BindAddr)
	if err != nil {
		return nil, fmt.Errorf("listen: %s: %w", s.conf.Proxy.BindAddr, err)
	}
	// If the advertise address is not set, infer it from the listen address.
	// Note using listen address rather than the configured bind address to
	// support port 0.
	if s.conf.Proxy.AdvertiseAddr == "" {
		advertiseAddr, err := advertiseAddrFromListenAddr(ln.Addr().String())
		if err != nil {
			// Should never happen.
			panic("invalid listen address: " + err.Error())
		}
		s.conf.Proxy.AdvertiseAddr = advertiseAddr
	}

	return ln, nil
}

func (s *Server) upstreamListen() (net.Listener, error) {
	ln, err := net.Listen("tcp", s.conf.Upstream.BindAddr)
	if err != nil {
		return nil, fmt.Errorf("listen: %s: %w", s.conf.Upstream.BindAddr, err)
	}
	// If the advertise address is not set, infer it from the listen address.
	// Note using listen address rather than the configured bind address to
	// support port 0.
	if s.conf.Upstream.AdvertiseAddr == "" {
		advertiseAddr, err := advertiseAddrFromListenAddr(ln.Addr().String())
		if err != nil {
			// Should never happen.
			panic("invalid listen address: " + err.Error())
		}
		s.conf.Upstream.AdvertiseAddr = advertiseAddr
	}

	return ln, nil
}

func (s *Server) adminListen() (net.Listener, error) {
	ln, err := net.Listen("tcp", s.conf.Admin.BindAddr)
	if err != nil {
		return nil, fmt.Errorf("listen: %s: %w", s.conf.Admin.BindAddr, err)
	}
	// If the advertise address is not set, infer it from the listen address.
	// Note using listen address rather than the configured bind address to
	// support port 0.
	if s.conf.Admin.AdvertiseAddr == "" {
		advertiseAddr, err := advertiseAddrFromListenAddr(ln.Addr().String())
		if err != nil {
			// Should never happen.
			panic("invalid listen address: " + err.Error())
		}
		s.conf.Admin.AdvertiseAddr = advertiseAddr
	}

	return ln, nil
}

// upstreamRebalance rebalances the upstream server connections every second.
func (s *Server) upstreamRebalance() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.upstreamServer.Rebalance()
		case <-s.rebalanceCtx.Done():
			return
		}
	}
}

// runGoroutine runs the given function as a background goroutine. If the
// function returns before the server is shutdown, it is considered a fatal
// error and the server is forcefully shutdown.
func (s *Server) runGoroutine(f func()) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		f()

		// If the server hasn't been shutdown, we consider it a fatal error
		// and trigger a shutdown.
		if !s.shutdown.Load() {
			s.fatalOnce.Do(func() {
				close(s.fatalCh)
			})
		}
	}()
}

func advertiseAddrFromListenAddr(bindAddr string) (string, error) {
	if strings.HasPrefix(bindAddr, ":") {
		bindAddr = "0.0.0.0" + bindAddr
	}

	host, port, err := net.SplitHostPort(bindAddr)
	if err != nil {
		return "", fmt.Errorf("invalid bind addr: %s: %w", bindAddr, err)
	}

	if host == "0.0.0.0" || host == "::" {
		ip, err := sockaddr.GetPrivateIP()
		if err != nil {
			return "", fmt.Errorf("get interface addr: %w", err)
		}
		if ip == "" {
			return "", fmt.Errorf("no private ip found")
		}
		return ip + ":" + port, nil
	}
	return bindAddr, nil
}
