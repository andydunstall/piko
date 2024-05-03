package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	picoconfig "github.com/andydunstall/pico/pkg/config"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server/config"
	"github.com/andydunstall/pico/server/gossip"
	"github.com/andydunstall/pico/server/netmap"
	proxy "github.com/andydunstall/pico/server/proxy"
	adminserver "github.com/andydunstall/pico/server/server/admin"
	proxyserver "github.com/andydunstall/pico/server/server/proxy"
	upstreamserver "github.com/andydunstall/pico/server/server/upstream"
	"github.com/hashicorp/go-sockaddr"
	rungroup "github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "start a server node",
		Long: `Start a server node.

The Pico server is responsible for proxying requests from downstream clients to
registered upstream listeners.

Pico may run as a cluster of nodes for fault tolerance and scalability. Use
'--cluster.join' to configure addresses of existing members in the cluster
to join.

Examples:
  # Start a Pico server.
  pico server

  # Start a Pico server, listening for proxy connections on :7000, upstream
  # connections on :7001 and admin connections on :7002.
  pico server --proxy.bind-addr :7000 --upstream.bind-addr :7001 --admin.bind-addr :7002

  # Start a Pico server and join an existing cluster by specifying each member.
  pico server --cluster.join 10.26.104.14,10.26.104.75

  # Start a Pico server and join an existing cluster by specifying a domain.
  # The server will resolve the domain and attempt to join each returned
  # member.
  pico server --cluster.join cluster.pico-ns.svc.cluster.local
`,
	}

	var conf config.Config

	var configPath string
	cmd.Flags().StringVar(
		&configPath,
		"config.path",
		"",
		`
YAML config file path.`,
	)

	var configExpandEnv bool
	cmd.Flags().BoolVar(
		&configExpandEnv,
		"config.expand-env",
		false,
		`
Whether to expand environment variables in the config file.

This will replaces references to ${VAR} or $VAR with the corresponding
environment variable. The replacement is case-sensitive.

References to undefined variables will be replaced with an empty string. A
default value can be given using form ${VAR:default}.`,
	)

	// Register flags and set default values.
	conf.RegisterFlags(cmd.Flags())

	cmd.Run = func(cmd *cobra.Command, args []string) {
		if configPath != "" {
			if err := picoconfig.Load(configPath, &conf, configExpandEnv); err != nil {
				fmt.Printf("load config: %s\n", err.Error())
				os.Exit(1)
			}
		}

		if err := conf.Validate(); err != nil {
			fmt.Printf("invalid config: %s\n", err.Error())
			os.Exit(1)
		}

		logger, err := log.NewLogger(conf.Log.Level, conf.Log.Subsystems)
		if err != nil {
			fmt.Printf("failed to setup logger: %s\n", err.Error())
			os.Exit(1)
		}

		if conf.Cluster.NodeID == "" {
			nodeID := netmap.GenerateNodeID()
			if conf.Cluster.NodeIDPrefix != "" {
				nodeID = conf.Cluster.NodeIDPrefix + nodeID
			}
			conf.Cluster.NodeID = nodeID
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

		if err := run(&conf, logger); err != nil {
			logger.Error("failed to run server", zap.Error(err))
			os.Exit(1)
		}
	}

	return cmd
}

func run(conf *config.Config, logger log.Logger) error {
	logger.Info("starting pico server", zap.Any("conf", conf))

	registry := prometheus.NewRegistry()

	adminLn, err := net.Listen("tcp", conf.Admin.BindAddr)
	if err != nil {
		return fmt.Errorf("admin listen: %s: %w", conf.Admin.BindAddr, err)
	}
	adminServer := adminserver.NewServer(
		adminLn,
		registry,
		logger,
	)

	networkMap := netmap.NewNetworkMap(&netmap.Node{
		ID:        conf.Cluster.NodeID,
		ProxyAddr: conf.Proxy.AdvertiseAddr,
		AdminAddr: conf.Admin.AdvertiseAddr,
	}, logger)
	networkMap.Metrics().Register(registry)
	adminServer.AddStatus("/netmap", netmap.NewStatus(networkMap))

	gossiper, err := gossip.NewGossip(networkMap, conf.Gossip, logger)
	if err != nil {
		return fmt.Errorf("gossip: %w", err)
	}
	defer gossiper.Close()
	adminServer.AddStatus("/gossip", gossip.NewStatus(gossiper))

	// Attempt to join an existing cluster. Note if 'join' is a domain that
	// doesn't map to any entries (except ourselves), then join will succeed
	// since it means we're the first member.
	nodeIDs, err := gossiper.Join(conf.Cluster.Join)
	if err != nil {
		return fmt.Errorf("join cluster: %w", err)
	}
	if len(nodeIDs) > 0 {
		logger.Info(
			"joined cluster",
			zap.Strings("node-ids", nodeIDs),
		)
	}

	p := proxy.NewProxy(networkMap, proxy.WithLogger(logger))
	adminServer.AddStatus("/proxy", proxy.NewStatus(p))

	proxyLn, err := net.Listen("tcp", conf.Proxy.BindAddr)
	if err != nil {
		return fmt.Errorf("proxy listen: %s: %w", conf.Proxy.BindAddr, err)
	}
	proxyServer := proxyserver.NewServer(
		proxyLn,
		p,
		&conf.Proxy,
		registry,
		logger,
	)

	upstreamLn, err := net.Listen("tcp", conf.Upstream.BindAddr)
	if err != nil {
		return fmt.Errorf("upstream listen: %s: %w", conf.Upstream.BindAddr, err)
	}
	upstreamServer := upstreamserver.NewServer(
		upstreamLn,
		p,
		registry,
		logger,
	)

	var group rungroup.Group

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

			leaveCtx, cancel := context.WithTimeout(
				context.Background(),
				time.Duration(conf.Server.GracefulShutdownTimeout)*time.Second,
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
		case <-signalCtx.Done():
			return nil
		}
	}, func(error) {
		signalCancel()
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
			time.Duration(conf.Server.GracefulShutdownTimeout)*time.Second,
		)
		defer cancel()

		if err := proxyServer.Shutdown(shutdownCtx); err != nil {
			logger.Warn("failed to gracefully shutdown proxy server", zap.Error(err))
		}

		logger.Info("proxy server shut down")
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
			time.Duration(conf.Server.GracefulShutdownTimeout)*time.Second,
		)
		defer cancel()

		if err := upstreamServer.Shutdown(shutdownCtx); err != nil {
			logger.Warn("failed to gracefully shutdown upstream server", zap.Error(err))
		}

		logger.Info("upstream server shut down")
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
			time.Duration(conf.Server.GracefulShutdownTimeout)*time.Second,
		)
		defer cancel()

		if err := adminServer.Shutdown(shutdownCtx); err != nil {
			logger.Warn("failed to gracefully shutdown server", zap.Error(err))
		}

		logger.Info("admin server shut down")
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

	if host == "0.0.0.0" {
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
