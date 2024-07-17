package gossip

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"
)

type Config struct {
	// BindAddr is the address to bind to listen for gossip traffic.
	BindAddr string `json:"bind_addr" yaml:"bind_addr"`

	// AdvertiseAddr is the address to advertise to other nodes.
	AdvertiseAddr string `json:"advertise_addr" yaml:"advertise_addr"`

	// Interval is the rate to initiate a gossip round.
	Interval time.Duration `json:"interval" yaml:"interval"`

	// MaxPacketSize is the maximum size of any packet sent.
	MaxPacketSize int `json:"max_packet_size" yaml:"max_packet_size"`
}

func (c *Config) Validate() error {
	if c.BindAddr == "" {
		return fmt.Errorf("missing bind addr")
	}
	if c.Interval == 0 {
		return fmt.Errorf("missing interval")
	}
	if c.MaxPacketSize == 0 {
		return fmt.Errorf("missing max packet size")
	}
	return nil
}

func (c *Config) RegisterFlags(fs *pflag.FlagSet, prefix string) {
	prefix = prefix + ".gossip."

	fs.StringVar(
		&c.BindAddr,
		prefix+"bind-addr",
		c.BindAddr,
		`
The host/port to listen for inter-node gossip traffic.

If the host is unspecified it defaults to all listeners, such as
a bind address ':8003' will listen on '0.0.0.0:8003'`,
	)

	fs.StringVar(
		&c.AdvertiseAddr,
		prefix+"advertise-addr",
		c.AdvertiseAddr,
		`
Gossip listen address to advertise to other nodes in the cluster. This is the
address other nodes will used to gossip with the node.

Such as if the listen address is ':8003', the advertised address may be
'10.26.104.45:8003' or 'node1.cluster:8003'.

By default, if the bind address includes an IP to bind to that will be used.
If the bind address does not include an IP (such as ':8003') the nodes
private IP will be used, such as a bind address of ':8003' may have an
advertise address of '10.26.104.14:8003'.`,
	)

	fs.DurationVar(
		&c.Interval,
		prefix+"interval",
		c.Interval,
		`
The interval to initiate rounds of gossip.

Each gossip round selects another known node to synchronize with.`,
	)

	fs.IntVar(
		&c.MaxPacketSize,
		prefix+"max-packet-size",
		c.MaxPacketSize,
		`
The maximum size of any packet sent.

Depending on your networks MTU you may be able to increase to include more data
in each packet.`,
	)
}
