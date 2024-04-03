// Copyright 2024 Andrew Dunstall. All rights reserved.
//
// Use of this source code is governed by a MIT style license that can be
// found in the LICENSE file.

package config

import (
	"fmt"
	"net/url"
	"strings"
)

type ServerConfig struct {
	// URL is the server URL.
	URL                      string `json:"url"`
	HeartbeatIntervalSeconds int    `json:"heartbeat_interval_seconds"`
	HeartbeatTimeoutSeconds  int    `json:"heartbeat_timeout_seconds"`
}

func (c *ServerConfig) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("missing url")
	}
	if _, err := url.Parse(c.URL); err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if c.HeartbeatIntervalSeconds == 0 {
		return fmt.Errorf("missing heartbeat interval")
	}
	if c.HeartbeatTimeoutSeconds == 0 {
		return fmt.Errorf("missing heartbeat timeout")
	}
	return nil
}

// ForwarderConfig contains the configuration for how to forward requests
// from Pico.
type ForwarderConfig struct {
	TimeoutSeconds int `json:"timeout_seconds"`
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
	// Listeners is a list of endpoints and forward addresses to register.
	//
	// Each listener has format '<endpoint ID>/<forward addr>', such
	// as 'd3934d4f/localhost:3000'.
	Listeners []string `json:"listeners"`

	Server ServerConfig `json:"server"`

	Forwarder ForwarderConfig `json:"forwarder"`

	Log LogConfig `json:"log"`
}

func (c *Config) Validate() error {
	if len(c.Listeners) == 0 {
		return fmt.Errorf("must have at least one listener")
	}
	for _, ln := range c.Listeners {
		if len(strings.Split(ln, "/")) != 2 {
			return fmt.Errorf("invalid listener: %s", ln)
		}
	}

	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	if err := c.Log.Validate(); err != nil {
		return fmt.Errorf("log: %w", err)
	}
	return nil
}
