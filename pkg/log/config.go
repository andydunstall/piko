package log

import (
	"fmt"
	"net/http"

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
	if _, err := zapLevelFromString(c.Level); err != nil {
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
	// Prevent these headers from being logged.
	// You can only define one of Allowlist or Blocklist.
	Blocklist []string `json:"blocklist" yaml:"blocklist"`

	// Log only these headers.
	// You can only define one of Allowlist or Blocklist.
	Allowlist []string `json:"allowlist" yaml:"allowlist"`

	// In-memory map that is composed right after arguments have
	// been validated. This is used to speed up runtime lookups
	// for headers.
	allowList map[string]string
}

func (c *AccessLogHeaderConfig) Validate() error {
	if len(c.Allowlist) > 0 && len(c.Blocklist) > 0 {
		return fmt.Errorf("cannot define both allowlist and blocklist")
	}

	// Create the allow-list header map used to filter headers at runtime.
	if len(c.Allowlist) > 0 {
		c.allowList = make(map[string]string)
		for _, el := range c.Allowlist {
			c.allowList[el] = el
		}
	}
	return nil
}

func (c *AccessLogHeaderConfig) Filter(h http.Header) http.Header {
	if len(c.Allowlist) > 0 {
		for name := range h {
			// Use the map created during validation to hasten lookups.
			if _, ok := c.allowList[name]; !ok {
				h.Del(name)
			}
		}
		return h
	}

	if len(c.Blocklist) > 0 {
		for _, blocked := range c.Blocklist {
			h.Del(blocked)
		}
		return h
	}

	return h
}

func (c *AccessLogHeaderConfig) RegisterFlags(fs *pflag.FlagSet, prefix string) {
	fs.StringSliceVar(
		&c.Allowlist,
		prefix+"allowlist",
		c.Allowlist,
		`
Log only these headers`,
	)
	fs.StringSliceVar(
		&c.Blocklist,
		prefix+"blocklist",
		c.Blocklist,
		`
Block these headers from being logged`,
	)
}

type AccessLogConfig struct {
	// If disabled, logs will be emitted with the 'debug' log level,
	// while respecting the header allow and block lists.
	Disable bool `json:"disable" yaml:"disable"`

	RequestHeaders AccessLogHeaderConfig `json:"request_headers" yaml:"request_headers"`

	ResponseHeaders AccessLogHeaderConfig `json:"response_headers" yaml:"response_headers"`
}

func (c *AccessLogConfig) Validate() error {
	if err := c.RequestHeaders.Validate(); err != nil {
		return fmt.Errorf("request headers: %w", err)
	}

	if err := c.ResponseHeaders.Validate(); err != nil {
		return fmt.Errorf("response headers: %w", err)
	}
	return nil
}

func (c *AccessLogConfig) RegisterFlags(fs *pflag.FlagSet, prefix string) {
	prefix = prefix + ".access-log."
	fs.BoolVar(
		&c.Disable,
		prefix+"disable",
		true,
		`
If Access logging is disabled`,
	)
	c.RequestHeaders.RegisterFlags(fs, prefix+"request-headers.")
	c.ResponseHeaders.RegisterFlags(fs, prefix+"response-headers.")
}
