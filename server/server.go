package server

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/andydunstall/piko/pkg/build"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/server/admin"
	"github.com/andydunstall/piko/server/auth"
	"github.com/andydunstall/piko/server/cluster"
	"github.com/andydunstall/piko/server/config"
	"github.com/andydunstall/piko/server/gossip"
	"github.com/andydunstall/piko/server/proxy"
	"github.com/andydunstall/piko/server/upstream"
	"github.com/andydunstall/piko/server/usage"
	"github.com/golang-jwt/jwt/v5"
	"github.com/hashicorp/go-sockaddr"
	rungroup "github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// Server is a Piko server node.
type Server struct {
	clusterState *cluster.State

	proxyLn     net.Listener
	proxyServer *proxy.Server

	upstreamLn     net.Listener
	upstreamServer *upstream.Server

	adminLn     net.Listener
	adminServer *admin.Server

	gossiper *gossip.Gossip

	reporter *usage.Reporter

	conf *config.Config

	closeCh    chan struct{}
	shutdownCh chan struct{}

	logger log.Logger
}

func NewServer(conf *config.Config, logger log.Logger) (*Server, error) {
	logger = logger.WithSubsystem("server")

	var verifier auth.Verifier
	if conf.Auth.AuthEnabled() {
		verifierConf := auth.JWTVerifierConfig{
			HMACSecretKey: []byte(conf.Auth.TokenHMACSecretKey),
			Audience:      conf.Auth.TokenAudience,
			Issuer:        conf.Auth.TokenIssuer,
		}

		if conf.Auth.TokenRSAPublicKey != "" {
			rsaPublicKey, err := jwt.ParseRSAPublicKeyFromPEM(
				[]byte(conf.Auth.TokenRSAPublicKey),
			)
			if err != nil {
				return nil, fmt.Errorf("parse rsa public key: %w", err)
			}
			verifierConf.RSAPublicKey = rsaPublicKey
		}
		if conf.Auth.TokenECDSAPublicKey != "" {
			ecdsaPublicKey, err := jwt.ParseECPublicKeyFromPEM(
				[]byte(conf.Auth.TokenECDSAPublicKey),
			)
			if err != nil {
				return nil, fmt.Errorf("parse ecdsa public key: %w", err)
			}
			verifierConf.ECDSAPublicKey = ecdsaPublicKey
		}
		verifier = auth.NewJWTVerifier(verifierConf)
	}

	registry := prometheus.NewRegistry()

	// Proxy listener.

	proxyLn, err := net.Listen("tcp", conf.Proxy.BindAddr)
	if err != nil {
		return nil, fmt.Errorf("proxy listen: %s: %w", conf.Proxy.BindAddr, err)
	}
	if conf.Proxy.AdvertiseAddr == "" {
		advertiseAddr, err := advertiseAddrFromBindAddr(proxyLn.Addr().String())
		if err != nil {
			// Should never happen.
			panic("invalid listen address: " + err.Error())
		}
		conf.Proxy.AdvertiseAddr = advertiseAddr
	}

	// Upstream listener.

	upstreamLn, err := net.Listen("tcp", conf.Upstream.BindAddr)
	if err != nil {
		return nil, fmt.Errorf("upstream listen: %s: %w", conf.Upstream.BindAddr, err)
	}
	if conf.Upstream.AdvertiseAddr == "" {
		advertiseAddr, err := advertiseAddrFromBindAddr(upstreamLn.Addr().String())
		if err != nil {
			// Should never happen.
			panic("invalid listen address: " + err.Error())
		}
		conf.Upstream.AdvertiseAddr = advertiseAddr
	}

	// Admin listener.

	adminLn, err := net.Listen("tcp", conf.Admin.BindAddr)
	if err != nil {
		return nil, fmt.Errorf("admin listen: %s: %w", conf.Admin.BindAddr, err)
	}
	if conf.Admin.AdvertiseAddr == "" {
		advertiseAddr, err := advertiseAddrFromBindAddr(adminLn.Addr().String())
		if err != nil {
			// Should never happen.
			panic("invalid listen address: " + err.Error())
		}
		conf.Admin.AdvertiseAddr = advertiseAddr
	}

	// Gossip listener.

	gossipStreamLn, err := net.Listen("tcp", conf.Gossip.BindAddr)
	if err != nil {
		return nil, fmt.Errorf("gossip listen: %s: %w", conf.Gossip.BindAddr, err)
	}

	gossipPacketLn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   gossipStreamLn.Addr().(*net.TCPAddr).IP,
		Port: gossipStreamLn.Addr().(*net.TCPAddr).Port,
	})
	if err != nil {
		return nil, fmt.Errorf("gossip listen: %s: %w", conf.Gossip.BindAddr, err)
	}

	if conf.Gossip.AdvertiseAddr == "" {
		advertiseAddr, err := advertiseAddrFromBindAddr(gossipStreamLn.Addr().String())
		if err != nil {
			// Should never happen.
			panic("invalid listen address: " + err.Error())
		}
		conf.Gossip.AdvertiseAddr = advertiseAddr
	}

	// Cluster.

	clusterState := cluster.NewState(&cluster.Node{
		ID:        conf.Cluster.NodeID,
		ProxyAddr: conf.Proxy.AdvertiseAddr,
		AdminAddr: conf.Admin.AdvertiseAddr,
	}, logger)
	clusterState.Metrics().Register(registry)

	upstreams := upstream.NewLoadBalancedManager(clusterState)
	upstreams.Metrics().Register(registry)

	// Proxy server.

	proxyTLSConfig, err := conf.Proxy.TLS.Load()
	if err != nil {
		return nil, fmt.Errorf("proxy tls: %w", err)
	}
	proxyServer := proxy.NewServer(
		upstreams,
		conf.Proxy,
		registry,
		proxyTLSConfig,
		logger,
	)

	// Upstream server.

	upstreamTLSConfig, err := conf.Upstream.TLS.Load()
	if err != nil {
		return nil, fmt.Errorf("upstream tls: %w", err)
	}
	upstreamServer := upstream.NewServer(
		upstreams,
		verifier,
		upstreamTLSConfig,
		logger,
	)

	// Admin server.

	adminTLSConfig, err := conf.Admin.TLS.Load()
	if err != nil {
		return nil, fmt.Errorf("admin tls: %w", err)
	}
	adminServer := admin.NewServer(
		clusterState,
		registry,
		adminTLSConfig,
		logger,
	)
	adminServer.AddStatus("/upstream", upstream.NewStatus(upstreams))
	adminServer.AddStatus("/cluster", cluster.NewStatus(clusterState))

	// Gossip.

	gossiper := gossip.NewGossip(
		clusterState,
		gossipStreamLn,
		gossipPacketLn,
		&conf.Gossip,
		logger,
	)
	gossiper.Metrics().Register(registry)
	adminServer.AddStatus("/gossip", gossip.NewStatus(gossiper))

	// Usage reporting.

	reporter := usage.NewReporter(upstreams.Usage(), logger)

	return &Server{
		clusterState:   clusterState,
		proxyLn:        proxyLn,
		proxyServer:    proxyServer,
		upstreamLn:     upstreamLn,
		upstreamServer: upstreamServer,
		adminLn:        adminLn,
		adminServer:    adminServer,
		gossiper:       gossiper,
		reporter:       reporter,
		conf:           conf,
		closeCh:        make(chan struct{}),
		shutdownCh:     make(chan struct{}),
		logger:         logger,
	}, nil
}

func (s *Server) Config() *config.Config {
	return s.conf
}

func (s *Server) ClusterState() *cluster.State {
	return s.clusterState
}

func (s *Server) Run(ctx context.Context) error {
	s.logger.Info(
		"starting piko server",
		zap.String("node-id", s.conf.Cluster.NodeID),
		zap.String("version", build.Version),
	)
	s.logger.Debug("piko config", zap.Any("config", s.conf))

	// Attempt to join an existing cluster.
	//
	// Note when running on Kubernetes, if this is the first member, as it is
	// not yet ready the service DNS record won't resolve so this may fail.
	// Therefore we attempt to join though continue booting if join fails.
	// Once booted we then attempt to join again with retries.
	nodeIDs, err := s.gossiper.JoinOnBoot(s.conf.Cluster.Join)
	if err != nil {
		s.logger.Warn("failed to join cluster", zap.Error(err))
	}
	if len(nodeIDs) > 0 {
		s.logger.Info(
			"joined cluster",
			zap.Strings("node-ids", nodeIDs),
		)
	}

	var group rungroup.Group

	// Proxy server.

	group.Add(func() error {
		if err := s.proxyServer.Serve(s.proxyLn); err != nil {
			return fmt.Errorf("proxy server serve: %w", err)
		}
		return nil
	}, func(error) {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			s.conf.GracePeriod,
		)
		defer cancel()

		if err := s.proxyServer.Shutdown(shutdownCtx); err != nil {
			s.logger.Warn("failed to gracefully shutdown proxy server", zap.Error(err))
		}

		s.logger.Info("proxy server shut down")
	})

	// Upstream server.

	group.Add(func() error {
		if err := s.upstreamServer.Serve(s.upstreamLn); err != nil {
			return fmt.Errorf("upstream server serve: %w", err)
		}
		return nil
	}, func(error) {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			s.conf.GracePeriod,
		)
		defer cancel()

		if err := s.upstreamServer.Shutdown(shutdownCtx); err != nil {
			s.logger.Warn("failed to gracefully shutdown admin server", zap.Error(err))
		}

		s.logger.Info("upstream server shut down")
	})

	// Admin server.

	group.Add(func() error {
		if err := s.adminServer.Serve(s.adminLn); err != nil {
			return fmt.Errorf("admin server serve: %w", err)
		}
		return nil
	}, func(error) {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			s.conf.GracePeriod,
		)
		defer cancel()

		if err := s.adminServer.Shutdown(shutdownCtx); err != nil {
			s.logger.Warn("failed to gracefully shutdown admin server", zap.Error(err))
		}

		s.logger.Info("admin server shut down")
	})

	// Gossip.

	gossipCtx, gossipCancel := context.WithCancel(context.Background())
	group.Add(func() error {
		if len(nodeIDs) == 0 {
			nodeIDs, err = s.gossiper.JoinOnStartup(gossipCtx, s.conf.Cluster.Join)
			if err != nil {
				if s.conf.Cluster.AbortIfJoinFails {
					return fmt.Errorf("join on startup: %w", err)
				}
				s.logger.Warn("failed to join cluster", zap.Error(err))
			}
			if len(nodeIDs) > 0 {
				s.logger.Info(
					"joined cluster",
					zap.Strings("node-ids", nodeIDs),
				)
			}
		}

		<-gossipCtx.Done()

		leaveCtx, cancel := context.WithTimeout(
			context.Background(),
			s.conf.GracePeriod,
		)
		defer cancel()

		// Leave as soon as we receive the shutdown signal to avoid receiving
		// forward proxy requests.
		if err := s.gossiper.Leave(leaveCtx); err != nil {
			s.logger.Warn("failed to gracefully leave cluster", zap.Error(err))
		} else {
			s.logger.Info("left cluster")
		}

		s.gossiper.Close()

		return nil
	}, func(error) {
		gossipCancel()
	})

	// Usage reporting.

	if !s.conf.Usage.Disable {
		usageCtx, usageCancel := context.WithCancel(context.Background())
		group.Add(func() error {
			s.reporter.Run(usageCtx)
			return nil
		}, func(error) {
			usageCancel()
		})
	}

	// Shutdown handler.

	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	group.Add(func() error {
		select {
		case <-ctx.Done():
			// On shutdown just exit the function and rungroup will shutdown
			// the remaining modules.
			s.logger.Info("received shutdown signal")
		case <-shutdownCtx.Done():
		}

		return nil
	}, func(error) {
		shutdownCancel()
	})

	return group.Run()
}

func advertiseAddrFromBindAddr(bindAddr string) (string, error) {
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
