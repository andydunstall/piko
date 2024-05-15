package status

import (
	"fmt"
	"net/url"
	"os"

	"github.com/andydunstall/piko/status/client"
	"github.com/andydunstall/piko/status/config"
	yaml "github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

func newProxyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proxy",
		Short: "inspect proxy status",
	}

	cmd.AddCommand(newProxyEndpointsCommand())

	return cmd
}

func newProxyEndpointsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "endpoints",
		Short: "inspect proxy endpoints",
		Long: `Inspect proxy endpoints.

Queries the server for the set of endpoints with connected upstream listeners
connected to this node. The output contains the endpoint IDs and set of
listeners for that endpoint.

Examples:
  piko status proxy endpoints
`,
	}

	var conf config.Config
	conf.RegisterFlags(cmd.Flags())

	cmd.Run = func(cmd *cobra.Command, args []string) {
		if err := conf.Validate(); err != nil {
			fmt.Printf("invalid config: %s\n", err.Error())
			os.Exit(1)
		}

		showProxyEndpoints(&conf)
	}

	return cmd
}

type proxyEndpointsOutput struct {
	Endpoints map[string][]string `json:"endpoints"`
}

func showProxyEndpoints(conf *config.Config) {
	// The URL has already been validated in conf.
	url, _ := url.Parse(conf.Server.URL)
	client := client.NewClient(url, conf.Forward)
	defer client.Close()

	endpoints, err := client.ProxyEndpoints()
	if err != nil {
		fmt.Printf("failed to get proxy endpoints: %s\n", err.Error())
		os.Exit(1)
	}

	output := proxyEndpointsOutput{
		Endpoints: endpoints,
	}
	b, _ := yaml.Marshal(output)
	fmt.Println(string(b))
}
