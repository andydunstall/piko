package config

import (
	"fmt"
	"time"

	"github.com/andydunstall/piko/pkg/gossip"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/server/auth"
	"github.com/spf13/pflag"
)

type UpstreamConfig struct {
	// BindAddr is the address to bind to listen for incoming HTTP connections.
	BindAddr string `json:"bind_addr" yaml:"bind_addr"`

	// AdvertiseAddr is the address to advertise to other nodes.
	AdvertiseAddr string `json:"advertise_addr" yaml:"advertise_addr"`
}

func (c *UpstreamConfig) Validate() error {
	if c.BindAddr == "" {
		return fmt.Errorf("missing bind addr")
	}
	return nil
}

type AdminConfig struct {
	// BindAddr is the address to bind to listen for incoming HTTP connections.
	BindAddr string `json:"bind_addr" yaml:"bind_addr"`

	// AdvertiseAddr is the address to advertise to other nodes.
	AdvertiseAddr string `json:"advertise_addr" yaml:"advertise_addr"`
}

func (c *AdminConfig) Validate() error {
	if c.BindAddr == "" {
		return fmt.Errorf("missing bind addr")
	}
	return nil
}

type ClusterConfig struct {
	// NodeID is a unique identifier for this node in the cluster.
	NodeID string `json:"node_id" yaml:"node_id"`

	// NodeIDPrefix is a node ID prefix, where Piko will generate the rest of
	// the node ID to ensure uniqueness.
	NodeIDPrefix string `json:"node_id_prefix" yaml:"node_id_prefix"`

	// Join contians a list of addresses of members in the cluster to join.
	Join []string `json:"join" yaml:"join"`

	AbortIfJoinFails bool `json:"abort_if_join_fails" yaml:"abort_if_join_fails"`
}

func (c *ClusterConfig) Validate() error {
	if c.NodeID != "" && c.NodeIDPrefix != "" {
		return fmt.Errorf("cannot specify both node ID and node ID prefix")
	}
	return nil
}

type Config struct {
	Proxy    ProxyConfig    `json:"proxy" yaml:"proxy"`
	Upstream UpstreamConfig `json:"upstream" yaml:"upstream"`
	Admin    AdminConfig    `json:"admin" yaml:"admin"`
	Gossip   gossip.Config  `json:"gossip" yaml:"gossip"`
	Cluster  ClusterConfig  `json:"cluster" yaml:"cluster"`
	Auth     auth.Config    `json:"auth" yaml:"auth"`
	Log      log.Config     `json:"log" yaml:"log"`

	// GracePeriod is the duration to gracefully shutdown the server. During
	// the grace period, listeners and idle connections are closed, then waits
	// for active requests to complete and closes their connections.
	GracePeriod time.Duration `json:"grace_period" yaml:"grace_period"`
}

func (c *Config) Validate() error {
	if err := c.Proxy.Validate(); err != nil {
		return fmt.Errorf("proxy: %w", err)
	}
	if err := c.Upstream.Validate(); err != nil {
		return fmt.Errorf("upstream: %w", err)
	}
	if err := c.Admin.Validate(); err != nil {
		return fmt.Errorf("admin: %w", err)
	}
	if err := c.Gossip.Validate(); err != nil {
		return fmt.Errorf("gossip: %w", err)
	}
	if err := c.Cluster.Validate(); err != nil {
		return fmt.Errorf("cluster: %w", err)
	}
	if err := c.Log.Validate(); err != nil {
		return fmt.Errorf("log: %w", err)
	}

	if c.GracePeriod == 0 {
		return fmt.Errorf("missing grace period")
	}

	return nil
}

func (c *Config) RegisterFlags(fs *pflag.FlagSet) {
	c.Proxy.RegisterFlags(fs)

	fs.StringVar(
		&c.Upstream.BindAddr,
		"upstream.bind-addr",
		":8001",
		`
The host/port to listen for connections from upstream listeners.

If the host is unspecified it defaults to all listeners, such as
'--upstream.bind-addr :8001' will listen on '0.0.0.0:8001'`,
	)
	fs.StringVar(
		&c.Upstream.AdvertiseAddr,
		"upstream.advertise-addr",
		"",
		`
Upstream listen address to advertise to other nodes in the cluster.

Such as if the listen address is ':8001', the advertised address may be
'10.26.104.45:8001' or 'node1.cluster:8001'.

By default, if the bind address includes an IP to bind to that will be used.
If the bind address does not include an IP (such as ':8001') the nodes
private IP will be used, such as a bind address of ':8001' may have an
advertise address of '10.16.104.14:8001'.`,
	)

	fs.StringVar(
		&c.Admin.BindAddr,
		"admin.bind-addr",
		":8002",
		`
The host/port to listen for incoming admin connections.

If the host is unspecified it defaults to all listeners, such as
'--admin.bind-addr :8002' will listen on '0.0.0.0:8002'`,
	)
	fs.StringVar(
		&c.Admin.AdvertiseAddr,
		"admin.advertise-addr",
		"",
		`
Admin listen address to advertise to other nodes in the cluster. This is the
address other nodes will used to forward admin requests.

Such as if the listen address is ':8002', the advertised address may be
'10.26.104.45:8002' or 'node1.cluster:8002'.

By default, if the bind address includes an IP to bind to that will be used.
If the bind address does not include an IP (such as ':8002') the nodes
private IP will be used, such as a bind address of ':8002' may have an
advertise address of '10.26.104.14:8002'.`,
	)

	fs.StringVar(
		&c.Cluster.NodeID,
		"cluster.node-id",
		"",
		`
A unique identifier for the node in the cluster.

By default a random ID will be generated for the node.`,
	)
	fs.StringVar(
		&c.Cluster.NodeIDPrefix,
		"cluster.node-id-prefix",
		"",
		`
A prefix for the node ID.

Piko will generate a unique random identifier for the node and append it to
the given prefix.

Such as you could use the node or pod  name as a prefix, then add a unique
identifier to ensure the node ID is unique across restarts.`,
	)
	fs.StringSliceVar(
		&c.Cluster.Join,
		"cluster.join",
		nil,
		`
A list of addresses of members in the cluster to join.

This may be either addresses of specific nodes, such as
'--cluster.join 10.26.104.14,10.26.104.75', or a domain that resolves to
the addresses of the nodes in the cluster (e.g. a Kubernetes headless
service), such as '--cluster.join piko.prod-piko-ns'.

Each address must include the host, and may optionally include a port. If no
port is given, the gossip port of this node is used.

Note each node propagates membership information to the other known nodes,
so the initial set of configured members only needs to be a subset of nodes.`,
	)
	fs.BoolVar(
		&c.Cluster.AbortIfJoinFails,
		"cluster.abort-if-join-fails",
		true,
		`
Whether the server node should abort if it is configured with more than one
node to join (excluding itself) but fails to join any members.`,
	)

	c.Auth.RegisterFlags(fs)

	c.Gossip.RegisterFlags(fs)

	c.Log.RegisterFlags(fs)

	fs.DurationVar(
		&c.GracePeriod,
		"grace-period",
		time.Minute,
		`
Maximum duration after a shutdown signal is received (SIGTERM or
SIGINT) to gracefully shutdown the server node before terminating.
This includes handling in-progress HTTP requests, gracefully closing
connections to upstream listeners, announcing to the cluster the node is
leaving...`,
	)
}
