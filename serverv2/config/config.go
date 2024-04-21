package config

import "fmt"

type ProxyConfig struct {
	// BindAddr is the address to bind to listen for incoming HTTP connections.
	BindAddr string `json:"http_addr"`
}

func (c *ProxyConfig) Validate() error {
	if c.BindAddr == "" {
		return fmt.Errorf("missing bind addr")
	}
	return nil
}

type AdminConfig struct {
	// BindAddr is the address to bind to listen for incoming HTTP connections.
	BindAddr string `json:"http_addr"`
}

func (c *AdminConfig) Validate() error {
	if c.BindAddr == "" {
		return fmt.Errorf("missing bind addr")
	}
	return nil
}

type ClusterConfig struct {
	// NodeID is a unique identifier for this node in the cluster.
	NodeID string `json:"node_id"`
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
	Proxy   ProxyConfig   `json:"proxy"`
	Admin   AdminConfig   `json:"admin"`
	Cluster ClusterConfig `json:"cluster"`
	Log     LogConfig     `json:"log"`
}

func (c *Config) Validate() error {
	if err := c.Proxy.Validate(); err != nil {
		return fmt.Errorf("proxy: %w", err)
	}
	if err := c.Admin.Validate(); err != nil {
		return fmt.Errorf("admin: %w", err)
	}
	if err := c.Log.Validate(); err != nil {
		return fmt.Errorf("log: %w", err)
	}
	return nil
}
