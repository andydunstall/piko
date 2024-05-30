package agent

import (
	"fmt"
	"os"

	"github.com/andydunstall/piko/agentv2/config"
	pikoconfig "github.com/andydunstall/piko/pkg/config"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agentv2 [command] [flags]",
		Short: "piko agent",
		Long: `The Piko agent registers endpoints with Piko then forwards
incoming connections for each endpoint to your upstream services.

The agent opens a single outbound connection to the Piko server, which is used
to proxy connections and requests. Therefore the agent never exposes a port.

The agent supports both YAML configuration and command line flags. Configure
a YAML file using '--config.path'. When enabling '--config.expand-env', Piko
will expand environment variables in the loaded YAML configuration.

Examples:
  # Register HTTP endpoint 'my-endpoint' for forward to localhost:3000.
  piko agent http my-endpoint 3000

  # Start all configured endpoints.
  piko agent start --config.file ./agent.yaml

  # Ping the server.
  piko agent ping
`,
		// TODO(andydunstall): Hide while in development.
		Hidden: true,
	}

	var configPath string
	cmd.PersistentFlags().StringVar(
		&configPath,
		"config.path",
		"",
		`
YAML config file path.`,
	)

	var configExpandEnv bool
	cmd.PersistentFlags().BoolVar(
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

	conf := &config.Config{}
	conf.RegisterFlags(cmd.PersistentFlags())

	cmd.AddCommand(newStartCommand(conf))
	cmd.AddCommand(newHTTPCommand(conf))
	cmd.AddCommand(newPingCommand(conf))

	// Load the configuration but don't yet validate.
	cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if configPath != "" {
			if err := pikoconfig.Load(configPath, &conf, configExpandEnv); err != nil {
				fmt.Printf("load config: %s\n", err.Error())
				os.Exit(1)
			}
		}

		if err := conf.Validate(); err != nil {
			fmt.Printf("config: %s\n", err.Error())
			os.Exit(1)
		}
	}

	return cmd
}
