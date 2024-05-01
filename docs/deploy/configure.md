# Configure

Pico server and agent support both YAML configuration and command-line flags.

The YAML file path can be set using `--config.path`.

## Variable Substitution

When enabling `--config.expand-env`, Pico will expand environment variables
in the loaded YAML configuration. This will replace references to `${VAR}`
and `$VAR` with the corresponding environment variable.

If the environment variable is not defined, it will be replaced with an empty
string. You can also defined a default value using form `${VAR:default}`.
