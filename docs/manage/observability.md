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

All logs can be enabled for a subsystem using `--log.subsystems`, which
overrides `--log.level` for the configured subsystems. Such as
`--log.subsystems server.http,metrics,proxy.forwarder`.

`--log.subsystems` enables any subsystem that are an exact match of the given
list. Such as `gossip` will match `gossip` but not `gossip.kite`.

## Metrics
The Pico server exposes Prometheus on the admin port at `/metrics`.

## Status
Pico includes a status CLI to inspect a Pico server. Servers register endpoints
at `/status` on the admin port that `pico status` then queries.

Such as to view the endpoints registers on a server use
`pico status proxy endpoints`. Or to inspect the set of known nodes in the
cluster use `pico status netmap nodes`.

Configure the server URL with `--server`.
