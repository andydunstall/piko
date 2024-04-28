package config

import "fmt"

type ProxyConfig struct {
	// BindAddr is the address to bind to listen for incoming HTTP connections.
	BindAddr string `json:"bind_addr"`

	// AdvertiseAddr is the address to advertise to other nodes.
	AdvertiseAddr string `json:"advertise_addr"`

	// GatewayTimeout is the timeout in seconds of forwarding requests to an
	// upstream listener.
	GatewayTimeout int `json:"gateway_timeout"`
}

func (c *ProxyConfig) Validate() error {
	if c.BindAddr == "" {
		return fmt.Errorf("missing bind addr")
	}
	if c.GatewayTimeout == 0 {
		return fmt.Errorf("missing gateway timeout")
	}
	return nil
}

type UpstreamConfig struct {
	// BindAddr is the address to bind to listen for incoming HTTP connections.
	BindAddr string `json:"bind_addr"`
}

func (c *UpstreamConfig) Validate() error {
	if c.BindAddr == "" {
		return fmt.Errorf("missing bind addr")
	}
	return nil
}

type AdminConfig struct {
	// BindAddr is the address to bind to listen for incoming HTTP connections.
	BindAddr string `json:"bind_addr"`

	// AdvertiseAddr is the address to advertise to other nodes.
	AdvertiseAddr string `json:"advertise_addr"`
}

func (c *AdminConfig) Validate() error {
	if c.BindAddr == "" {
		return fmt.Errorf("missing bind addr")
	}
	return nil
}

type GossipConfig struct {
	// BindAddr is the address to bind to listen for gossip traffic.
	BindAddr string `json:"bind_addr"`

	// AdvertiseAddr is the address to advertise to other nodes.
	AdvertiseAddr string `json:"advertise_addr"`
}

func (c *GossipConfig) Validate() error {
	if c.BindAddr == "" {
		return fmt.Errorf("missing bind addr")
	}
	return nil
}

type ClusterConfig struct {
	// NodeID is a unique identifier for this node in the cluster.
	NodeID string `json:"node_id"`

	// Join contians a list of addresses of members in the cluster to join.
	Join []string `json:"join"`
}

type ServerConfig struct {
	// GracefulShutdownTimeout is the timeout to allow for graceful shutdown
	// of the server in seconds.
	GracefulShutdownTimeout int `json:"graceful_shutdown_timeout"`
}

func (c *ServerConfig) Validate() error {
	if c.GracefulShutdownTimeout == 0 {
		return fmt.Errorf("missing grafeful shutdown timeout")
	}
	return nil
}

type LogConfig struct {
	Level string `json:"level"`
	// Subsystems enables debug logging on logs the given subsystems (which
	// overrides level).
	Subsystems []string `json:"subsystems"`
}

func (c *LogConfig) Validate() error {
	if c.Level == "" {
		return fmt.Errorf("missing level")
	}
	return nil
}

type Config struct {
	Proxy    ProxyConfig    `json:"proxy"`
	Upstream UpstreamConfig `json:"upstream"`
	Admin    AdminConfig    `json:"admin"`
	Gossip   GossipConfig   `json:"gossip"`
	Cluster  ClusterConfig  `json:"cluster"`
	Server   ServerConfig   `json:"server"`
	Log      LogConfig      `json:"log"`
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
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	if err := c.Log.Validate(); err != nil {
		return fmt.Errorf("log: %w", err)
	}
	return nil
}
