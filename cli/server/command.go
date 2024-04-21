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

	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server/config"
	"github.com/andydunstall/pico/server/gossip"
	"github.com/andydunstall/pico/server/netmap"
	"github.com/andydunstall/pico/server/proxy"
	adminserver "github.com/andydunstall/pico/server/server/admin"
	proxyserver "github.com/andydunstall/pico/server/server/proxy"
	"github.com/hashicorp/go-sockaddr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "start a server node",
		Long: `Start a server node.

The Pico server is responsible for proxying requests from downstream clients to
registered upstream listeners.

The server has two ports, a 'proxy' port which accepts connections from both
downstream clients and upstream listeners, and an 'admin' port which is used
to inspect the status of the server.

Pico may run as a cluster of nodes for fault tolerance and scalability. Use
'--cluster.join' to configure addresses of existing members in the cluster
to join.

Examples:
  # Start a Pico server.
  pico server

  # Start a Pico server, listening for proxy connections on :7000 and admin
  // ocnnections on :9000.
  pico server --proxy.bind-addr :8000 --admin.bind-addr :9000

  # Start a Pico server and join an existing cluster by specifying each member.
  pico server --cluster.join 10.26.104.14,10.26.104.75

  # Start a Pico server and join an existing cluster by specifying a domain.
  # The server will resolve the domain and attempt to join each returned
  # member.
  pico server --cluster.join cluster.pico-ns.svc.cluster.local
`,
	}

	var conf config.Config

	cmd.Flags().StringVar(
		&conf.Proxy.BindAddr,
		"proxy.bind-addr",
		":8080",
		`
The host/port to listen for incoming proxy HTTP and WebSocket connections.

If the host is unspecified it defaults to all listeners, such as
'--proxy.bind-addr :8080' will listen on '0.0.0.0:8080'`,
	)
	cmd.Flags().StringVar(
		&conf.Gossip.AdvertiseAddr,
		"proxy.advertise-addr",
		"",
		`
Proxy listen address to advertise to other nodes in the cluster. This is the
address other nodes will used to forward proxy requests.

Such as if the listen address is ':8080', the advertised address may be
'10.26.104.45:8080' or 'node1.cluster:8080'.

By default, if the bind address includes an IP to bind to that will be used.
If the bind address does not include an IP (such as ':8080') the nodes
private IP will be used, such as a bind address of ':8080' may have an
advertise address of '10.26.104.14:8080'.`,
	)

	cmd.Flags().StringVar(
		&conf.Admin.BindAddr,
		"admin.bind-addr",
		":8081",
		`
The host/port to listen for incoming admin connections.

If the host is unspecified it defaults to all listeners, such as
'--admin.bind-addr :8081' will listen on '0.0.0.0:8081'`,
	)
	cmd.Flags().StringVar(
		&conf.Gossip.AdvertiseAddr,
		"admin.advertise-addr",
		"",
		`
Admin listen address to advertise to other nodes in the cluster. This is the
address other nodes will used to forward admin requests.

Such as if the listen address is ':8081', the advertised address may be
'10.26.104.45:8081' or 'node1.cluster:8081'.

By default, if the bind address includes an IP to bind to that will be used.
If the bind address does not include an IP (such as ':8081') the nodes
private IP will be used, such as a bind address of ':8081' may have an
advertise address of '10.26.104.14:8081'.`,
	)

	cmd.Flags().StringVar(
		&conf.Gossip.BindAddr,
		"gossip.bind-addr",
		":7000",
		`
The host/port to listen for inter-node gossip traffic.

If the host is unspecified it defaults to all listeners, such as
'--gossip.bind-addr :7000' will listen on '0.0.0.0:7000'`,
	)

	cmd.Flags().StringVar(
		&conf.Gossip.AdvertiseAddr,
		"gossip.advertise-addr",
		"",
		`
Gossip listen address to advertise to other nodes in the cluster. This is the
address other nodes will used to gossip with the node.

Such as if the listen address is ':7000', the advertised address may be
'10.26.104.45:7000' or 'node1.cluster:7000'.

By default, if the bind address includes an IP to bind to that will be used.
If the bind address does not include an IP (such as ':7000') the nodes
private IP will be used, such as a bind address of ':7000' may have an
advertise address of '10.26.104.14:7000'.`,
	)

	cmd.Flags().StringVar(
		&conf.Cluster.NodeID,
		"cluster.node-id",
		"",
		`
A unique identifier for the node in the cluster.

By default a random ID will be generated for the node.`,
	)
	cmd.Flags().StringSliceVar(
		&conf.Cluster.Join,
		"cluster.join",
		nil,
		`
A list of addresses of members in the cluster to join.

This may be either addresses of specific nodes, such as
'--cluster.join 10.26.104.14,10.26.104.75', or a domain that resolves to
the addresses of the nodes in the cluster (e.g. a Kubernetes headless
service), such as '--cluster.join pico.prod-pico-ns'.

Each address must include the host, and may optionally include a port. If no
port is given, the gossip port of this node is used.

Note each node propagates membership information to the other known nodes,
so the initial set of configured members only needs to be a subset of nodes.`,
	)

	cmd.Flags().StringVar(
		&conf.Log.Level,
		"log.level",
		"info",
		`
Minimum log level to output.

The available levels are 'debug', 'info', 'warn' and 'error'.`,
	)
	cmd.Flags().StringSliceVar(
		&conf.Log.Subsystems,
		"log.subsystems",
		nil,
		`
Each log has a 'subsystem' field where the log occured.

'--log.subsystems' enables all log levels for those given subsystems. This
can be useful to debug a particular subsystem without having to enable all
debug logs.

Such as you can enable 'gossip' logs with '--log.subsystems gossip'.`,
	)

	cmd.Run = func(cmd *cobra.Command, args []string) {
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
			conf.Cluster.NodeID = netmap.GenerateNodeID()
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

		run(&conf, logger)
	}

	return cmd
}

func run(conf *config.Config, logger *log.Logger) {
	logger.Info("starting pico server", zap.Any("conf", conf))

	registry := prometheus.NewRegistry()

	networkMap := netmap.NewNetworkMap(&netmap.Node{
		ID:         conf.Cluster.NodeID,
		Status:     netmap.NodeStatusJoining,
		ProxyAddr:  conf.Proxy.AdvertiseAddr,
		AdminAddr:  conf.Admin.AdvertiseAddr,
		GossipAddr: conf.Gossip.AdvertiseAddr,
	}, logger)
	gossip, err := gossip.NewGossip(
		conf.Gossip.BindAddr,
		networkMap,
		registry,
		logger,
	)
	if err != nil {
		logger.Error("failed to start gossip: %w", zap.Error(err))
		os.Exit(1)
	}
	defer gossip.Close()

	if len(conf.Cluster.Join) > 0 {
		if err := gossip.Join(conf.Cluster.Join); err != nil {
			logger.Error("failed to join cluster: %w", zap.Error(err))
			os.Exit(1)
		}
	}

	adminServer := adminserver.NewServer(
		conf.Admin.BindAddr,
		registry,
		logger,
	)

	networkMapStatus := netmap.NewStatus(networkMap)
	adminServer.AddStatus("/netmap", networkMapStatus)

	proxyServer := proxyserver.NewServer(
		conf.Proxy.BindAddr,
		proxy.NewProxy(networkMap, registry, logger),
		registry,
		logger,
	)

	ctx, cancel := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := adminServer.Serve(); err != nil {
			return fmt.Errorf("admin server serve: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		<-ctx.Done()

		logger.Info("shutting down admin server")

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			// TODO(andydunstall): Add configuration.
			time.Second*30,
		)
		defer cancel()

		if err := adminServer.Shutdown(shutdownCtx); err != nil {
			logger.Warn("failed to gracefully shutdown server", zap.Error(err))
		}
		return nil
	})
	g.Go(func() error {
		if err := proxyServer.Serve(); err != nil {
			return fmt.Errorf("proxy server serve: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		<-ctx.Done()

		logger.Info("shutting down proxy server")

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			// TODO(andydunstall): Add configuration.
			time.Second*30,
		)
		defer cancel()

		if err := proxyServer.Shutdown(shutdownCtx); err != nil {
			logger.Warn("failed to gracefully shutdown server", zap.Error(err))
		}
		return nil
	})

	networkMap.UpdateLocalStatus(netmap.NodeStatusActive)

	if err := g.Wait(); err != nil {
		logger.Error("failed to run server", zap.Error(err))
	}

	if err := gossip.Leave(); err != nil {
		logger.Error("failed to leave gossip", zap.Error(err))
	}

	logger.Info("shutdown complete")
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
