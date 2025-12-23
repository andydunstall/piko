package bench

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/dragonflydb/piko/bench/config"
	pikoconfig "github.com/dragonflydb/piko/pkg/config"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bench",
		Short: "benchmark piko",
		Long: `Benchmark Piko.

Each benchmark client registers Piko upstreams that echo received requests,
then sends the configured number of requests to the upstreams via Piko.

Examples:
  # Benchmark 100000 HTTP requests from 50 clients.
  piko bench http --requests 100000 --clients 50
`,
	}

	conf := config.Default()
	var loadConf pikoconfig.Config

	// Register flags and set default values.
	conf.RegisterFlags(cmd.PersistentFlags())
	loadConf.RegisterFlags(cmd.PersistentFlags())

	cmd.PersistentPreRun = func(_ *cobra.Command, _ []string) {
		if err := pikoconfig.Load(conf, loadConf.Path, loadConf.ExpandEnv); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		if err := conf.Validate(); err != nil {
			fmt.Printf("config: %s\n", err.Error())
			os.Exit(1)
		}
	}

	cmd.AddCommand(newHTTPCommand(conf))

	return cmd
}
