package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/andydunstall/piko/cli/server/status"
	pikoconfig "github.com/andydunstall/piko/pkg/config"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/server"
	"github.com/andydunstall/piko/server/cluster"
	"github.com/andydunstall/piko/server/config"
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

	conf := config.Default()
	var loadConf pikoconfig.Config

	// Register flags and set default values.
	conf.RegisterFlags(cmd.Flags())
	loadConf.RegisterFlags(cmd.Flags())

	var logger log.Logger

	cmd.PreRun = func(_ *cobra.Command, _ []string) {
		if err := loadConf.Load(conf); err != nil {
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
	}

	cmd.Run = func(_ *cobra.Command, _ []string) {
		if err := runServer(conf, logger); err != nil {
			logger.Error("failed to run server", zap.Error(err))
			os.Exit(1)
		}
	}

	cmd.AddCommand(status.NewCommand())

	return cmd
}

func runServer(conf *config.Config, logger log.Logger) error {
	ctx, cancel := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	server, err := server.NewServer(conf, logger)
	if err != nil {
		return err
	}

	if err := server.Start(); err != nil {
		return err
	}

	if !server.Wait(ctx) {
		os.Exit(1)
	}

	return nil
}
