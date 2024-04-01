# Observability

## Logging
This section describes logging in the Pico server and agent.

### Output
Pico uses structured logs, where logs are written to `stderr` formatted as
JSON.

### Log Levels
Each log record has a `level` field of either:
* `debug`: Verbose logs for debugging
* `info`: Important state changes
* `warn`: An unexpected event occured though Pico can continue to work
* `error`: An unexpected error that prevents Pico from functioning
properly

The minimum level to log can be configured with `--log.level`.

### Subsystem
Each log record has a `subsystem` field indicating where in the system the
log occured.

Subsystem fields can contain segments separated by periods. Such as
`rpc`, `rpc.conn`, `rpc.handler`, ...

All logs can be enabled for a subsystem using `--log.subsystems`, which
overrides `--log.level` for the configured subsystems. Each entry matches
the given subsystem and any subsystems that match the given prefix.

Such as `--log.subsystems rpc,metrics` will match logs with subsystem `rpc`,
`metrics`, `rpc.conn`, `metrics.prometheus`, ...
