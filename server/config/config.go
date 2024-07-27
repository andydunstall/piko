package config

import (
	"fmt"
	"time"

	"github.com/spf13/pflag"

	"github.com/andydunstall/piko/pkg/auth"
	"github.com/andydunstall/piko/pkg/gossip"
	"github.com/andydunstall/piko/pkg/log"
)

// HTTPConfig contains generic configuration for the HTTP servers.
type HTTPConfig struct {
	// ReadTimeout is the maximum duration for reading the entire
	// request, including the body. A zero or negative value means
	// there will be no timeout.
	ReadTimeout time.Duration `json:"read_timeout" yaml:"read_timeout"`

	// ReadHeaderTimeout is the amount of time allowed to read
	// request headers.
	ReadHeaderTimeout time.Duration `json:"read_header_timeout" yaml:"read_header_timeout"`

	// WriteTimeout is the maximum duration before timing out
	// writes of the response.
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`

	// IdleTimeout is the maximum amount of time to wait for the
	// next request when keep-alives are enabled.
	IdleTimeout time.Duration `json:"idle_timeout" yaml:"idle_timeout"`

	// MaxHeaderBytes controls the maximum number of bytes the
	// server will read parsing the request header's keys and
	// values, including the request line.
	MaxHeaderBytes int `json:"max_header_bytes" yaml:"max_header_bytes"`
}

func (c *HTTPConfig) RegisterFlags(fs *pflag.FlagSet, prefix string) {
	if prefix == "" {
		prefix = "http."
	} else {
		prefix = prefix + ".http."
	}

	fs.DurationVar(
		&c.ReadTimeout,
		prefix+"read-timeout",
		c.ReadTimeout,
		`
The maximum duration for reading the entire request, including the body. A
zero or negative value means there will be no timeout.`,
	)
	fs.DurationVar(
		&c.ReadHeaderTimeout,
		prefix+"read-header-timeout",
		c.ReadHeaderTimeout,
		`
The maximum duration for reading the request headers. If zero,
http.read-timeout is used.`,
	)
	fs.DurationVar(
		&c.WriteTimeout,
		prefix+"write-timeout",
		c.WriteTimeout,
		`
The maximum duration before timing out writes of the response.`,
	)
	fs.DurationVar(
		&c.IdleTimeout,
		prefix+"idle-timeout",
		c.IdleTimeout,
		`
The maximum amount of time to wait for the next request when keep-alives are
enabled.`,
	)
	fs.IntVar(
		&c.MaxHeaderBytes,
		prefix+"max-header-bytes",
		c.MaxHeaderBytes,
		`
The maximum number of bytes the server will read parsing the request header's
keys and values, including the request line.`,
	)
}

type ProxyConfig struct {
	// BindAddr is the address to bind to listen for incoming HTTP connections.
	BindAddr string `json:"bind_addr" yaml:"bind_addr"`

	// AdvertiseAddr is the address to advertise to other nodes.
	AdvertiseAddr string `json:"advertise_addr" yaml:"advertise_addr"`

	// Timeout is the timeout to forward incoming requests to the upstream.
	Timeout time.Duration `json:"timeout" yaml:"timeout"`

	// AccessLog indicates whether to log all incoming connections and
	// requests.
	AccessLog bool `json:"access_log" yaml:"access_log"`

	Auth auth.Config `json:"auth" yaml:"auth"`

	HTTP HTTPConfig `json:"http" yaml:"http"`

	TLS TLSConfig `json:"tls" yaml:"tls"`
}

func (c *ProxyConfig) Validate() error {
	if c.BindAddr == "" {
		return fmt.Errorf("missing bind addr")
	}
	if c.Timeout == 0 {
		return fmt.Errorf("missing timeout")
	}
	if err := c.TLS.Validate(); err != nil {
		return fmt.Errorf("tls: %w", err)
	}
	return nil
}

func (c *ProxyConfig) RegisterFlags(fs *pflag.FlagSet) {
	fs.StringVar(
		&c.BindAddr,
		"proxy.bind-addr",
		c.BindAddr,
		`
The host/port to listen for incoming proxy connections.

If the host is unspecified it defaults to all listeners, such as
'--proxy.bind-addr :8000' will listen on '0.0.0.0:8000'`,
	)

	fs.StringVar(
		&c.AdvertiseAddr,
		"proxy.advertise-addr",
		c.AdvertiseAddr,
		`
Address to advertise to other nodes in the cluster. This is the
address other nodes will used to forward proxy connections.

Such as if the listen address is ':8000', the advertised address may be
'10.26.104.45:8000' or 'node1.cluster:8000'.

By default, if the bind address includes an IP to bind to that will be used.
If the bind address does not include an IP (such as ':8000') the nodes
private IP will be used, such as a bind address of ':8000' may have an
advertise address of '10.26.104.14:8000'.`,
	)

	fs.DurationVar(
		&c.Timeout,
		"proxy.timeout",
		c.Timeout,
		`
Timeout when forwarding incoming requests to the upstream.`,
	)

	fs.BoolVar(
		&c.AccessLog,
		"proxy.access-log",
		c.AccessLog,
		`
Whether to log all incoming connections and requests.`,
	)

	c.HTTP.RegisterFlags(fs, "proxy")

	c.Auth.RegisterFlags(fs, "proxy")

	c.TLS.RegisterFlags(fs, "proxy")
}

type UpstreamConfig struct {
	// BindAddr is the address to bind to listen for incoming HTTP connections.
	BindAddr string `json:"bind_addr" yaml:"bind_addr"`

	// AdvertiseAddr is the address to advertise to other nodes.
	AdvertiseAddr string `json:"advertise_addr" yaml:"advertise_addr"`

	Auth auth.Config `json:"auth" yaml:"auth"`

	TLS TLSConfig `json:"tls" yaml:"tls"`
}

func (c *UpstreamConfig) Validate() error {
	if c.BindAddr == "" {
		return fmt.Errorf("missing bind addr")
	}
	if err := c.TLS.Validate(); err != nil {
		return fmt.Errorf("tls: %w", err)
	}
	return nil
}

func (c *UpstreamConfig) RegisterFlags(fs *pflag.FlagSet) {
	fs.StringVar(
		&c.BindAddr,
		"upstream.bind-addr",
		c.BindAddr,
		`
The host/port to listen for incoming upstream connections.

If the host is unspecified it defaults to all listeners, such as
'--upstream.bind-addr :8001' will listen on '0.0.0.0:8001'`,
	)

	fs.StringVar(
		&c.AdvertiseAddr,
		"upstream.advertise-addr",
		c.AdvertiseAddr,
		`
Address to advertise to other nodes in the cluster.

Such as if the listen address is ':8001', the advertised address may be
'10.26.104.45:8001' or 'node1.cluster:8001'.

By default, if the bind address includes an IP to bind to that will be used.
If the bind address does not include an IP (such as ':8000') the nodes
private IP will be used, such as a bind address of ':8000' may have an
advertise address of '10.26.104.14:8000'.`,
	)

	c.Auth.RegisterFlags(fs, "upstream")

	c.TLS.RegisterFlags(fs, "upstream")
}

type AdminConfig struct {
	// BindAddr is the address to bind to listen for incoming HTTP connections.
	BindAddr string `json:"bind_addr" yaml:"bind_addr"`

	// AdvertiseAddr is the address to advertise to other nodes.
	AdvertiseAddr string `json:"advertise_addr" yaml:"advertise_addr"`

	Auth auth.Config `json:"auth" yaml:"auth"`

	TLS TLSConfig `json:"tls" yaml:"tls"`
}

func (c *AdminConfig) Validate() error {
	if c.BindAddr == "" {
		return fmt.Errorf("missing bind addr")
	}
	if err := c.TLS.Validate(); err != nil {
		return fmt.Errorf("tls: %w", err)
	}
	return nil
}

func (c *AdminConfig) RegisterFlags(fs *pflag.FlagSet) {
	fs.StringVar(
		&c.BindAddr,
		"admin.bind-addr",
		c.BindAddr,
		`
The host/port to listen for incoming admin connections.

If the host is unspecified it defaults to all listeners, such as
'--admin.bind-addr :8002' will listen on '0.0.0.0:8002'`,
	)

	fs.StringVar(
		&c.AdvertiseAddr,
		"admin.advertise-addr",
		c.AdvertiseAddr,
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

	c.Auth.RegisterFlags(fs, "admin")

	c.TLS.RegisterFlags(fs, "admin")
}

type ClusterConfig struct {
	// NodeID is a unique identifier for this node in the cluster.
	NodeID string `json:"node_id" yaml:"node_id"`

	// NodeIDPrefix is a node ID prefix, where Piko will generate the rest of
	// the node ID to ensure uniqueness.
	NodeIDPrefix string `json:"node_id_prefix" yaml:"node_id_prefix"`

	// Join contains a list of addresses of members in the cluster to join.
	Join []string `json:"join" yaml:"join"`

	// JoinTimeout is the time to keep trying to join the cluster before
	// failing.
	JoinTimeout time.Duration `json:"join_timeout" yaml:"join_timeout"`

	AbortIfJoinFails bool `json:"abort_if_join_fails" yaml:"abort_if_join_fails"`

	Gossip gossip.Config `json:"gossip" yaml:"gossip"`

	RebalancingThreshold float32 `json:"rebalancing_threshold" yaml:"rebalancing_threshold"`

	RebalancingRate float32 `json:"rebalancing_rate" yaml:"rebalancing_rate"`

	RebalancingCheckInterval time.Duration `json:"rebalancing_check_interval" yaml:"rebalancing_check_interval"`
}

func (c *ClusterConfig) Validate() error {
	if c.NodeID == "" {
		return fmt.Errorf("missing node id")
	}
	if c.JoinTimeout == 0 {
		return fmt.Errorf("missing join timeout")
	}

	if err := c.Gossip.Validate(); err != nil {
		return fmt.Errorf("gossip: %w", err)
	}

	return nil
}

func (c *ClusterConfig) RegisterFlags(fs *pflag.FlagSet) {
	fs.StringVar(
		&c.NodeID,
		"cluster.node-id",
		c.NodeID,
		`
A unique identifier for the node in the cluster.

By default a random ID will be generated for the node.`,
	)

	fs.StringVar(
		&c.NodeIDPrefix,
		"cluster.node-id-prefix",
		c.NodeIDPrefix,
		`
A prefix for the node ID.

Piko will generate a unique random identifier for the node and append it to
the given prefix.

Such as you could use the node or pod  name as a prefix, then add a unique
identifier to ensure the node ID is unique across restarts.`,
	)

	fs.StringSliceVar(
		&c.Join,
		"cluster.join",
		c.Join,
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

	fs.DurationVar(
		&c.JoinTimeout,
		"cluster.join-timeout",
		c.JoinTimeout,
		`
The timeout to attempt to join an existing cluster when 'cluster.join' is
set.`,
	)

	fs.BoolVar(
		&c.AbortIfJoinFails,
		"cluster.abort-if-join-fails",
		c.AbortIfJoinFails,
		`
Whether the server node should abort if it is configured with more than one
node to join (excluding itself) but fails to join any members.`,
	)
	fs.Float32Var(
		&c.RebalancingThreshold,
		"cluster.rebalancing-threshold",
		c.RebalancingThreshold,
		`
Threshold for node startup rebalancing, if the node have 'threshold' more
connections than the cluster average`,
	)

	fs.Float32Var(
		&c.RebalancingRate,
		"cluster.rebalancing-rate",
		c.RebalancingRate,
		`
Rate for node startup rebalancing, shedding 'rate' of connections every second.`,
	)

	fs.DurationVar(
		&c.RebalancingCheckInterval,
		"cluster.rebalancing-check-interval",
		c.RebalancingCheckInterval,
		`
Time interval that checks if rebalancing is needed.`,
	)

	c.Gossip.RegisterFlags(fs, "cluster")
}

type UsageConfig struct {
	// Disable indicates whether to disable anonymous usage collection.
	Disable bool `json:"disable" yaml:"disable"`
}

func (c *UsageConfig) RegisterFlags(fs *pflag.FlagSet) {
	fs.BoolVar(
		&c.Disable,
		"usage.disable",
		c.Disable,
		`
Whether to disable anonymous usage tracking.

The Piko server periodically sends an anonymous report to help understand how
Piko is being used. This report includes the Piko version, host OS, host
architecture, requests processed and upstreams registered.`,
	)
}

type Config struct {
	Proxy ProxyConfig `json:"proxy" yaml:"proxy"`

	Upstream UpstreamConfig `json:"upstream" yaml:"upstream"`

	Admin AdminConfig `json:"admin" yaml:"admin"`

	Cluster ClusterConfig `json:"cluster" yaml:"cluster"`

	Usage UsageConfig `json:"usage" yaml:"usage"`

	Log log.Config `json:"log" yaml:"log"`

	// GracePeriod is the duration to gracefully shutdown the server. During
	// the grace period, listeners and idle connections are closed, then waits
	// for active requests to complete and closes their connections.
	GracePeriod time.Duration `json:"grace_period" yaml:"grace_period"`
}

func Default() *Config {
	return &Config{
		Proxy: ProxyConfig{
			BindAddr:  ":8000",
			Timeout:   time.Second * 30,
			AccessLog: true,
			HTTP: HTTPConfig{
				ReadTimeout:       time.Second * 10,
				ReadHeaderTimeout: time.Second * 10,
				WriteTimeout:      time.Second * 10,
				IdleTimeout:       time.Minute * 5,
				MaxHeaderBytes:    1 << 20,
			},
		},
		Upstream: UpstreamConfig{
			BindAddr: ":8001",
		},
		Admin: AdminConfig{
			BindAddr: ":8002",
		},
		Cluster: ClusterConfig{
			JoinTimeout:      time.Minute,
			AbortIfJoinFails: true,
			Gossip: gossip.Config{
				BindAddr:      ":8003",
				Interval:      time.Millisecond * 100,
				MaxPacketSize: 1400,
			},
			RebalancingThreshold:     0.2,
			RebalancingRate:          0.005,
			RebalancingCheckInterval: time.Second * 5,
		},
		Log: log.Config{
			Level: "info",
		},
		GracePeriod: time.Minute,
	}
}

func (c *Config) Validate() error {
	if err := c.Cluster.Validate(); err != nil {
		return fmt.Errorf("cluster: %w", err)
	}

	if err := c.Proxy.Validate(); err != nil {
		return fmt.Errorf("proxy: %w", err)
	}

	if err := c.Upstream.Validate(); err != nil {
		return fmt.Errorf("upstream: %w", err)
	}

	if err := c.Admin.Validate(); err != nil {
		return fmt.Errorf("admin: %w", err)
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
	c.Cluster.RegisterFlags(fs)

	c.Proxy.RegisterFlags(fs)

	c.Upstream.RegisterFlags(fs)

	c.Admin.RegisterFlags(fs)

	c.Usage.RegisterFlags(fs)

	c.Log.RegisterFlags(fs)

	fs.DurationVar(
		&c.GracePeriod,
		"grace-period",
		c.GracePeriod,
		`
Maximum duration after a shutdown signal is received (SIGTERM or
SIGINT) to gracefully shutdown the server node before terminating.
This includes handling in-progress HTTP requests, gracefully closing
connections to upstream listeners and announcing to the cluster the node is
leaving.`,
	)
}
