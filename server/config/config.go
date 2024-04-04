// Copyright 2024 Andrew Dunstall. All rights reserved.
//
// Use of this source code is governed by a MIT style license that can be
// found in the LICENSE file.

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

type ProxyConfig struct {
	TimeoutSeconds int `json:"timeout_seconds"`
}

func (c *ProxyConfig) Validate() error {
	if c.TimeoutSeconds == 0 {
		return fmt.Errorf("missing timeout seconds")
	}
	return nil
}

type UpstreamConfig struct {
	HeartbeatIntervalSeconds int `json:"heartbeat_interval_seconds"`
	HeartbeatTimeoutSeconds  int `json:"heartbeat_timeout_seconds"`
}

func (c *UpstreamConfig) Validate() error {
	if c.HeartbeatIntervalSeconds == 0 {
		return fmt.Errorf("missing heartbeat interval")
	}
	if c.HeartbeatTimeoutSeconds == 0 {
		return fmt.Errorf("missing heartbeat timeout")
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
	Server   ServerConfig   `json:"server"`
	Proxy    ProxyConfig    `json:"proxy"`
	Upstream UpstreamConfig `json:"upstream"`
	Log      LogConfig      `json:"log"`
}

func (c *Config) Validate() error {
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	if err := c.Proxy.Validate(); err != nil {
		return fmt.Errorf("proxy: %w", err)
	}
	if err := c.Upstream.Validate(); err != nil {
		return fmt.Errorf("upstream: %w", err)
	}
	if err := c.Log.Validate(); err != nil {
		return fmt.Errorf("log: %w", err)
	}
	return nil
}
