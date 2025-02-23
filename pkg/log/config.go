package log

import (
	"fmt"

	"github.com/spf13/pflag"
)

type Config struct {
	// Level is the minimum record level to log. Either 'debug', 'info', 'warn'
	// or 'error'.
	Level string `json:"level" yaml:"level"`

	// Subsystems enables debug logging on log records whose 'subsystem'
	// matches one of the given values (overrides `Level`).
	Subsystems []string `json:"subsystems" yaml:"subsystems"`
}

func (c *Config) Validate() error {
	if c.Level == "" {
		return fmt.Errorf("missing level")
	}
	if _, err := ZapLevelFromString(c.Level); err != nil {
		return err
	}
	return nil
}

func (c *Config) RegisterFlags(fs *pflag.FlagSet) {
	fs.StringVar(
		&c.Level,
		"log.level",
		c.Level,
		`
Minimum log level to output.

The available levels are 'debug', 'info', 'warn' and 'error'.`,
	)
	fs.StringSliceVar(
		&c.Subsystems,
		"log.subsystems",
		c.Subsystems,
		`
Each log has a 'subsystem' field where the log occured.

'--log.subsystems' enables all log levels for those given subsystems. This
can be useful to debug a particular subsystem without having to enable all
debug logs.

Such as you can enable 'gossip' logs with '--log.subsystems gossip'.`,
	)
}

type AccessLogHeaderConfig struct {
	// BlockList contains headers that will be redacted from the audit log.
	//
	// You must only define one of AllowList or BlockList.
	BlockList []string `json:"block_list" yaml:"block_list"`

	// AllowList contains the ONLY headers that will be logged.
	//
	// You must only define one of AllowList or BlockList.
	AllowList []string `json:"allow_list" yaml:"allow_list"`
}

func (c *AccessLogHeaderConfig) Validate() error {
	if len(c.AllowList) > 0 && len(c.BlockList) > 0 {
		return fmt.Errorf("cannot define both allow list and block list")
	}
	return nil
}

func (c *AccessLogHeaderConfig) RegisterFlags(fs *pflag.FlagSet, prefix string) {
	fs.StringSliceVar(
		&c.BlockList,
		prefix+"block-list",
		c.BlockList,
		`
Block these headers from being logged.

You must only define one of block list and allow list.`,
	)
	fs.StringSliceVar(
		&c.AllowList,
		prefix+"allow-list",
		c.AllowList,
		`
The ONLY headers that will be logged.

You must only define one of block list and allow list.`,
	)
}

type AccessLogConfig struct {
	// Level is the record log level for audit log entries. Either 'debug',
	// 'info', 'warn' or 'error'.
	Level string `json:"level" yaml:"level"`

	RequestHeaders AccessLogHeaderConfig `json:"request_headers" yaml:"request_headers"`

	ResponseHeaders AccessLogHeaderConfig `json:"response_headers" yaml:"response_headers"`

	// Disable disables the access log, so requests will not be logged.
	Disable bool `json:"disable" yaml:"disable"`
}

func (c *AccessLogConfig) Validate() error {
	if c.Disable {
		return nil
	}

	if c.Level == "" {
		return fmt.Errorf("missing level")
	}
	if _, err := ZapLevelFromString(c.Level); err != nil {
		return err
	}

	if err := c.RequestHeaders.Validate(); err != nil {
		return fmt.Errorf("request headers: %w", err)
	}

	if err := c.ResponseHeaders.Validate(); err != nil {
		return fmt.Errorf("response headers: %w", err)
	}
	return nil
}

func (c *AccessLogConfig) RegisterFlags(fs *pflag.FlagSet, prefix string) {
	if len(prefix) > 0 {
		prefix = prefix + ".access-log."
	} else {
		prefix = "access-log."
	}
	fs.StringVar(
		&c.Level,
		prefix+"level",
		c.Level,
		`
The record log level for audit log entries.

The available levels are 'debug', 'info', 'warn' and 'error'.`,
	)
	c.RequestHeaders.RegisterFlags(fs, prefix+"request-headers.")
	c.ResponseHeaders.RegisterFlags(fs, prefix+"response-headers.")
	fs.BoolVar(
		&c.Disable,
		prefix+"disable",
		c.Disable,
		`
Disable the access log, so requests will not be logged.`,
	)
}
