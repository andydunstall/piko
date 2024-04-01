package config

import "fmt"

type ServerConfig struct {
	Addr string `json:"addr"`
	// GracePeriodSeconds is the maximum number of seconds to gracefully
	// shutdown after receiving a shutdown signal.
	GracePeriodSeconds int `json:"grace_period_seconds"`
}

func (c *ServerConfig) Validate() error {
	if c.Addr == "" {
		return fmt.Errorf("missing addr")
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
	Server ServerConfig `json:"server"`
	Log    LogConfig    `json:"log"`
}

func (c *Config) Validate() error {
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	if err := c.Log.Validate(); err != nil {
		return fmt.Errorf("log: %w", err)
	}
	return nil
}
