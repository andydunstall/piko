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
