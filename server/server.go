package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server/auth"
	"github.com/andydunstall/pico/server/cluster"
	"github.com/andydunstall/pico/server/config"
	"github.com/andydunstall/pico/server/gossip"
	"github.com/andydunstall/pico/server/proxy"
	adminserver "github.com/andydunstall/pico/server/server/admin"
	proxyserver "github.com/andydunstall/pico/server/server/proxy"
	upstreamserver "github.com/andydunstall/pico/server/server/upstream"
	"github.com/golang-jwt/jwt/v5"
	"github.com/hashicorp/go-sockaddr"
	rungroup "github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// Server manages setting up and running a Pico server node.
type Server struct {
	proxyLn    net.Listener
	upstreamLn net.Listener
	adminLn    net.Listener

	gossipStreamLn net.Listener
	gossipPacketLn net.PacketConn

	conf *config.Config

	logger log.Logger
}

func NewServer(conf *config.Config, logger log.Logger) (*Server, error) {
	adminLn, err := net.Listen("tcp", conf.Admin.BindAddr)
	if err != nil {
		return nil, fmt.Errorf("admin listen: %s: %w", conf.Admin.BindAddr, err)
	}

	proxyLn, err := net.Listen("tcp", conf.Proxy.BindAddr)
	if err != nil {
		return nil, fmt.Errorf("proxy listen: %s: %w", conf.Proxy.BindAddr, err)
	}

	upstreamLn, err := net.Listen("tcp", conf.Upstream.BindAddr)
	if err != nil {
		return nil, fmt.Errorf("upstream listen: %s: %w", conf.Upstream.BindAddr, err)
	}

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

	if conf.Cluster.NodeID == "" {
		nodeID := cluster.GenerateNodeID()
		if conf.Cluster.NodeIDPrefix != "" {
			nodeID = conf.Cluster.NodeIDPrefix + nodeID
		}
		conf.Cluster.NodeID = nodeID
	}

	// Incase the address has port 0, set the bind address to the listen
	// address.
	conf.Proxy.BindAddr = proxyLn.Addr().String()
	conf.Upstream.BindAddr = upstreamLn.Addr().String()
	conf.Admin.BindAddr = adminLn.Addr().String()
	conf.Gossip.BindAddr = gossipStreamLn.Addr().String()

	if conf.Proxy.AdvertiseAddr == "" {
		advertiseAddr, err := advertiseAddrFromBindAddr(conf.Proxy.BindAddr)
		if err != nil {
			logger.Error("invalid configuration", zap.Error(err))
			os.Exit(1)
		}
		conf.Proxy.AdvertiseAddr = advertiseAddr
	}
	if conf.Upstream.AdvertiseAddr == "" {
		advertiseAddr, err := advertiseAddrFromBindAddr(conf.Upstream.BindAddr)
		if err != nil {
			logger.Error("invalid configuration", zap.Error(err))
			os.Exit(1)
		}
		conf.Upstream.AdvertiseAddr = advertiseAddr
	}
	if conf.Admin.AdvertiseAddr == "" {
		advertiseAddr, err := advertiseAddrFromBindAddr(conf.Admin.BindAddr)
		if err != nil {
			logger.Error("invalid configuration", zap.Error(err))
			os.Exit(1)
		}
		conf.Admin.AdvertiseAddr = advertiseAddr
	}
	if conf.Gossip.AdvertiseAddr == "" {
		advertiseAddr, err := advertiseAddrFromBindAddr(conf.Gossip.BindAddr)
		if err != nil {
			logger.Error("invalid configuration", zap.Error(err))
			os.Exit(1)
		}
		conf.Gossip.AdvertiseAddr = advertiseAddr
	}

	return &Server{
		proxyLn:        proxyLn,
		upstreamLn:     upstreamLn,
		adminLn:        adminLn,
		gossipStreamLn: gossipStreamLn,
		gossipPacketLn: gossipPacketLn,
		conf:           conf,
		logger:         logger,
	}, nil
}

func (s *Server) Run(ctx context.Context) error {
	var verifier auth.Verifier
	if s.conf.Auth.AuthEnabled() {
		verifierConf := auth.JWTVerifierConfig{
			HMACSecretKey: []byte(s.conf.Auth.TokenHMACSecretKey),
			Audience:      s.conf.Auth.TokenAudience,
			Issuer:        s.conf.Auth.TokenIssuer,
		}

		if s.conf.Auth.TokenRSAPublicKey != "" {
			rsaPublicKey, err := jwt.ParseRSAPublicKeyFromPEM(
				[]byte(s.conf.Auth.TokenRSAPublicKey),
			)
			if err != nil {
				return fmt.Errorf("parse rsa public key: %w", err)
			}
			verifierConf.RSAPublicKey = rsaPublicKey
		}
		if s.conf.Auth.TokenECDSAPublicKey != "" {
			ecdsaPublicKey, err := jwt.ParseECPublicKeyFromPEM(
				[]byte(s.conf.Auth.TokenECDSAPublicKey),
			)
			if err != nil {
				return fmt.Errorf("parse ecdsa public key: %w", err)
			}
			verifierConf.ECDSAPublicKey = ecdsaPublicKey
		}
		verifier = auth.NewJWTVerifier(verifierConf)
	}

	s.logger.Info("starting pico server", zap.Any("conf", s.conf))

	registry := prometheus.NewRegistry()

	adminServer := adminserver.NewServer(
		s.adminLn,
		registry,
		s.logger,
	)

	clusterState := cluster.NewState(&cluster.Node{
		ID:        s.conf.Cluster.NodeID,
		ProxyAddr: s.conf.Proxy.AdvertiseAddr,
		AdminAddr: s.conf.Admin.AdvertiseAddr,
	}, s.logger)
	clusterState.Metrics().Register(registry)
	adminServer.AddStatus("/cluster", cluster.NewStatus(clusterState))

	gossiper := gossip.NewGossip(
		clusterState,
		s.gossipStreamLn,
		s.gossipPacketLn,
		&s.conf.Gossip,
		s.logger,
	)
	defer gossiper.Close()
	adminServer.AddStatus("/gossip", gossip.NewStatus(gossiper))

	// Attempt to join an existing cluster.
	//
	// Note when running on Kubernetes, if this is the first member, as it is
	// not yet ready the service DNS record won't resolve so this may fail.
	// Therefore we attempt to join though continue booting if join fails.
	// Once booted we then attempt to join again with retries.
	nodeIDs, err := gossiper.JoinOnBoot(s.conf.Cluster.Join)
	if err != nil {
		s.logger.Warn("failed to join cluster", zap.Error(err))
	}
	if len(nodeIDs) > 0 {
		s.logger.Info(
			"joined cluster",
			zap.Strings("node-ids", nodeIDs),
		)
	}

	p := proxy.NewProxy(clusterState, proxy.WithLogger(s.logger))
	p.Metrics().Register(registry)
	adminServer.AddStatus("/proxy", proxy.NewStatus(p))

	proxyServer := proxyserver.NewServer(
		s.proxyLn,
		p,
		&s.conf.Proxy,
		registry,
		s.logger,
	)

	upstreamServer := upstreamserver.NewServer(
		s.upstreamLn,
		p,
		verifier,
		registry,
		s.logger,
	)

	var group rungroup.Group

	// Termination handler.
	shutdownCtx, shutdownCancel := context.WithCancel(ctx)
	group.Add(func() error {
		select {
		case <-ctx.Done():
		case <-shutdownCtx.Done():
		}
		return nil
	}, func(error) {
		shutdownCancel()
	})

	// Proxy server.
	group.Add(func() error {
		if err := proxyServer.Serve(); err != nil {
			return fmt.Errorf("proxy server serve: %w", err)
		}
		return nil
	}, func(error) {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			s.conf.Server.GracefulShutdownTimeout,
		)
		defer cancel()

		if err := proxyServer.Shutdown(shutdownCtx); err != nil {
			s.logger.Warn("failed to gracefully shutdown proxy server", zap.Error(err))
		}

		s.logger.Info("proxy server shut down")
	})

	// Upstream server.
	group.Add(func() error {
		if err := upstreamServer.Serve(); err != nil {
			return fmt.Errorf("upstream server serve: %w", err)
		}
		return nil
	}, func(error) {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			s.conf.Server.GracefulShutdownTimeout,
		)
		defer cancel()

		if err := upstreamServer.Shutdown(shutdownCtx); err != nil {
			s.logger.Warn("failed to gracefully shutdown upstream server", zap.Error(err))
		}

		s.logger.Info("upstream server shut down")
	})

	// Admin server.
	group.Add(func() error {
		if err := adminServer.Serve(); err != nil {
			return fmt.Errorf("admin server serve: %w", err)
		}
		return nil
	}, func(error) {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			s.conf.Server.GracefulShutdownTimeout,
		)
		defer cancel()

		if err := adminServer.Shutdown(shutdownCtx); err != nil {
			s.logger.Warn("failed to gracefully shutdown server", zap.Error(err))
		}

		s.logger.Info("admin server shut down")
	})

	// Gossip.
	gossipCtx, gossipCancel := context.WithCancel(ctx)
	group.Add(func() error {
		if len(nodeIDs) == 0 {
			nodeIDs, err = gossiper.JoinOnStartup(gossipCtx, s.conf.Cluster.Join)
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
			s.conf.Server.GracefulShutdownTimeout,
		)
		defer cancel()

		// Leave as soon as we receive the shutdown signal to avoid receiving
		// forward proxy requests.
		if err := gossiper.Leave(leaveCtx); err != nil {
			s.logger.Warn("failed to gracefully leave cluster", zap.Error(err))
		} else {
			s.logger.Info("left cluster")
		}

		return nil
	}, func(error) {
		gossipCancel()
	})

	if err := group.Run(); err != nil {
		return err
	}

	s.logger.Info("shutdown complete")

	return nil
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
