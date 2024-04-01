// Copyright 2024 Andrew Dunstall. All rights reserved.
//
// Use of this source code is governed by a MIT style license that can be
// found in the LICENSE file.

package config

import (
	"fmt"
	"strings"
)

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

	if err := c.Log.Validate(); err != nil {
		return fmt.Errorf("log: %w", err)
	}
	return nil
}
