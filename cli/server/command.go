package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/andydunstall/piko/cli/server/status"
	"github.com/andydunstall/piko/pkg/build"
	pikoconfig "github.com/andydunstall/piko/pkg/config"
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
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server [flags]",
		Short: "start a server node",
		Long: `The Piko server is responsible for routing incoming proxy
requests and connections to upstream services. Upstream services listen for
traffic on a particular endpoint by opening an outbound-only connection to the
server. Piko then routes traffic for each endpoint to an appropriate upstream
connection.

Use '--cluster.join' to run the server as a cluster of nodes, where you can
specify either a list of addresses of existing members, or a domain that
resolves to the addresses of existing members.

The server exposes 4 ports:
- Proxy port: Receives HTTP(S) requests from proxy clients which are routed
to an upstream service
- Upstream port: Accepts connections from upstream services
- Admin port: Exposes metrics and a status API to inspect the server state
- Gossip port: Used for inter-node gossip traffic

The server supports both YAML configuration and command line flags. Configure
a YAML file using '--config.path'. When enabling '--config.expand-env', Piko
will expand environment variables in the loaded YAML configuration.

Examples:
  # Start a Piko server node.
  piko server

  # Load configuration from YAML.
  piko server --config.path ./server.yaml

  # Start a Piko server and join an existing cluster by specifying each member.
  piko server --cluster.join 10.26.104.14,10.26.104.75

  # Start a Piko server and join an existing cluster by specifying a domain.
  # The server will resolve the domain and attempt to join each returned
  # member.
  piko server --cluster.join cluster.piko-ns.svc.cluster.local
`,
	}

	var conf config.Config
	var loadConf pikoconfig.Config

	// Register flags and set default values.
	conf.RegisterFlags(cmd.Flags())
	loadConf.RegisterFlags(cmd.Flags())

	var logger log.Logger

	cmd.PreRun = func(_ *cobra.Command, _ []string) {
		if err := loadConf.Load(&conf); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		if conf.Cluster.NodeID == "" {
			nodeID := cluster.GenerateNodeID()
			if conf.Cluster.NodeIDPrefix != "" {
				nodeID = conf.Cluster.NodeIDPrefix + nodeID
			}
			conf.Cluster.NodeID = nodeID
		}

		if err := conf.Validate(); err != nil {
			fmt.Printf("config: %s\n", err.Error())
			os.Exit(1)
		}

		var err error
		logger, err = log.NewLogger(conf.Log.Level, conf.Log.Subsystems)
		if err != nil {
			fmt.Printf("failed to setup logger: %s\n", err.Error())
			os.Exit(1)
		}

		if conf.Proxy.AdvertiseAddr == "" {
			advertiseAddr, err := advertiseAddrFromBindAddr(conf.Proxy.BindAddr)
			if err != nil {
				logger.Error("invalid configuration", zap.Error(err))
				os.Exit(1)
			}
			conf.Proxy.AdvertiseAddr = advertiseAddr
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
	}

	cmd.Run = func(_ *cobra.Command, _ []string) {
		if err := runServer(&conf, logger); err != nil {
			logger.Error("failed to run agent", zap.Error(err))
			os.Exit(1)
		}
	}

	cmd.AddCommand(status.NewCommand())

	return cmd
}

func runServer(conf *config.Config, logger log.Logger) error {
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
				return fmt.Errorf("parse rsa public key: %w", err)
			}
			verifierConf.RSAPublicKey = rsaPublicKey
		}
		if conf.Auth.TokenECDSAPublicKey != "" {
			ecdsaPublicKey, err := jwt.ParseECPublicKeyFromPEM(
				[]byte(conf.Auth.TokenECDSAPublicKey),
			)
			if err != nil {
				return fmt.Errorf("parse ecdsa public key: %w", err)
			}
			verifierConf.ECDSAPublicKey = ecdsaPublicKey
		}
		verifier = auth.NewJWTVerifier(verifierConf)
	}

	logger.Info(
		"starting piko server",
		zap.String("node-id", conf.Cluster.NodeID),
		zap.String("version", build.Version),
	)
	logger.Debug("piko config", zap.Any("config", conf))

	registry := prometheus.NewRegistry()

	clusterState := cluster.NewState(&cluster.Node{
		ID:        conf.Cluster.NodeID,
		ProxyAddr: conf.Proxy.AdvertiseAddr,
		AdminAddr: conf.Admin.AdvertiseAddr,
	}, logger)
	clusterState.Metrics().Register(registry)

	upstreams := upstream.NewLoadBalancedManager(clusterState)
	upstreams.Metrics().Register(registry)

	var group rungroup.Group

	// Gossip.

	gossipStreamLn, err := net.Listen("tcp", conf.Gossip.BindAddr)
	if err != nil {
		return fmt.Errorf("gossip listen: %s: %w", conf.Gossip.BindAddr, err)
	}

	gossipPacketLn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   gossipStreamLn.Addr().(*net.TCPAddr).IP,
		Port: gossipStreamLn.Addr().(*net.TCPAddr).Port,
	})
	if err != nil {
		return fmt.Errorf("gossip listen: %s: %w", conf.Gossip.BindAddr, err)
	}

	gossiper := gossip.NewGossip(
		clusterState,
		gossipStreamLn,
		gossipPacketLn,
		&conf.Gossip,
		logger,
	)
	defer gossiper.Close()
	gossiper.Metrics().Register(registry)

	// Attempt to join an existing cluster.
	//
	// Note when running on Kubernetes, if this is the first member, as it is
	// not yet ready the service DNS record won't resolve so this may fail.
	// Therefore we attempt to join though continue booting if join fails.
	// Once booted we then attempt to join again with retries.
	nodeIDs, err := gossiper.JoinOnBoot(conf.Cluster.Join)
	if err != nil {
		logger.Warn("failed to join cluster", zap.Error(err))
	}
	if len(nodeIDs) > 0 {
		logger.Info(
			"joined cluster",
			zap.Strings("node-ids", nodeIDs),
		)
	}

	gossipCtx, gossipCancel := context.WithCancel(context.Background())
	group.Add(func() error {
		if len(nodeIDs) == 0 {
			nodeIDs, err = gossiper.JoinOnStartup(gossipCtx, conf.Cluster.Join)
			if err != nil {
				if conf.Cluster.AbortIfJoinFails {
					return fmt.Errorf("join on startup: %w", err)
				}
				logger.Warn("failed to join cluster", zap.Error(err))
			}
			if len(nodeIDs) > 0 {
				logger.Info(
					"joined cluster",
					zap.Strings("node-ids", nodeIDs),
				)
			}
		}

		<-gossipCtx.Done()

		leaveCtx, cancel := context.WithTimeout(
			context.Background(),
			conf.GracePeriod,
		)
		defer cancel()

		// Leave as soon as we receive the shutdown signal to avoid receiving
		// forward proxy requests.
		if err := gossiper.Leave(leaveCtx); err != nil {
			logger.Warn("failed to gracefully leave cluster", zap.Error(err))
		} else {
			logger.Info("left cluster")
		}

		return nil
	}, func(error) {
		gossipCancel()
	})

	// Proxy server.
	proxyLn, err := net.Listen("tcp", conf.Proxy.BindAddr)
	if err != nil {
		return fmt.Errorf("proxy listen: %s: %w", conf.Proxy.BindAddr, err)
	}
	proxyTLSConfig, err := conf.Proxy.TLS.Load()
	if err != nil {
		return fmt.Errorf("proxy tls: %w", err)
	}
	proxyServer := proxy.NewServer(
		upstreams,
		conf.Proxy,
		registry,
		proxyTLSConfig,
		logger,
	)

	group.Add(func() error {
		if err := proxyServer.Serve(proxyLn); err != nil {
			return fmt.Errorf("proxy server serve: %w", err)
		}
		return nil
	}, func(error) {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			conf.GracePeriod,
		)
		defer cancel()

		if err := proxyServer.Shutdown(shutdownCtx); err != nil {
			logger.Warn("failed to gracefully shutdown server", zap.Error(err))
		}

		logger.Info("proxy server shut down")
	})

	// Upstream server.
	upstreamLn, err := net.Listen("tcp", conf.Upstream.BindAddr)
	if err != nil {
		return fmt.Errorf("upstream listen: %s: %w", conf.Upstream.BindAddr, err)
	}
	upstreamTLSConfig, err := conf.Upstream.TLS.Load()
	if err != nil {
		return fmt.Errorf("upstream tls: %w", err)
	}
	upstreamServer := upstream.NewServer(
		upstreams,
		verifier,
		upstreamTLSConfig,
		logger,
	)

	group.Add(func() error {
		if err := upstreamServer.Serve(upstreamLn); err != nil {
			return fmt.Errorf("upstream server serve: %w", err)
		}
		return nil
	}, func(error) {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			conf.GracePeriod,
		)
		defer cancel()

		if err := upstreamServer.Shutdown(shutdownCtx); err != nil {
			logger.Warn("failed to gracefully shutdown server", zap.Error(err))
		}

		logger.Info("upstream server shut down")
	})

	// Admin Server.
	adminLn, err := net.Listen("tcp", conf.Admin.BindAddr)
	if err != nil {
		return fmt.Errorf("admin listen: %s: %w", conf.Admin.BindAddr, err)
	}
	adminTLSConfig, err := conf.Admin.TLS.Load()
	if err != nil {
		return fmt.Errorf("admin tls: %w", err)
	}
	adminServer := admin.NewServer(
		clusterState,
		registry,
		adminTLSConfig,
		logger,
	)
	adminServer.AddStatus("/upstream", upstream.NewStatus(upstreams))
	adminServer.AddStatus("/cluster", cluster.NewStatus(clusterState))
	adminServer.AddStatus("/gossip", gossip.NewStatus(gossiper))

	group.Add(func() error {
		if err := adminServer.Serve(adminLn); err != nil {
			return fmt.Errorf("admin server serve: %w", err)
		}
		return nil
	}, func(error) {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			conf.GracePeriod,
		)
		defer cancel()

		if err := adminServer.Shutdown(shutdownCtx); err != nil {
			logger.Warn("failed to gracefully shutdown server", zap.Error(err))
		}

		logger.Info("admin server shut down")
	})

	// Usage.
	reporter := usage.NewReporter(upstreams.Usage(), logger)
	if !conf.Usage.Disable {
		usageCtx, usageCancel := context.WithCancel(context.Background())
		group.Add(func() error {
			reporter.Run(usageCtx)
			return nil
		}, func(error) {
			usageCancel()
		})
	}

	// Termination handler.
	signalCtx, signalCancel := context.WithCancel(context.Background())
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	group.Add(func() error {
		select {
		case sig := <-signalCh:
			logger.Info(
				"received shutdown signal",
				zap.String("signal", sig.String()),
			)
			return nil
		case <-signalCtx.Done():
			return nil
		}
	}, func(error) {
		signalCancel()
	})

	if err := group.Run(); err != nil {
		return err
	}

	logger.Info("shutdown complete")

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
