package gossip

import (
	"fmt"

	"github.com/spf13/pflag"
)

type Config struct {
	// BindAddr is the address to bind to listen for gossip traffic.
	BindAddr string `json:"bind_addr" yaml:"bind_addr"`

	// AdvertiseAddr is the address to advertise to other nodes.
	AdvertiseAddr string `json:"advertise_addr" yaml:"advertise_addr"`
}

func (c *Config) Validate() error {
	if c.BindAddr == "" {
		return fmt.Errorf("missing bind addr")
	}
	return nil
}

func (c *Config) RegisterFlags(fs *pflag.FlagSet) {
	fs.StringVar(
		&c.BindAddr,
		"gossip.bind-addr",
		":7000",
		`
The host/port to listen for inter-node gossip traffic.

If the host is unspecified it defaults to all listeners, such as
'--gossip.bind-addr :7000' will listen on '0.0.0.0:7000'`,
	)

	fs.StringVar(
		&c.AdvertiseAddr,
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
}
