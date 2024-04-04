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

Subsystem fields can contain segments separated by periods. Such as
`server`, `server.http`, `server.http.route`, ... This is used to separate
types of logs so you don't have to enable all logs for a given subsystem.

`--log.subsystems` enables any subsystem whose starting segments match the
given filter. Such as `--log.subsystems server` will match logs with subsystem
`server`, `server.http`, `server.http.route`..., though
`--log.subsystems server.http.route` will only match `server.http.route`.

## Metrics
Both the Pico server and agent expose Prometheus metrics at /pico/v1/metrics.

### Available Metrics
| Metric                             | Type      | Labels        | Description                                        |
| ---------------------------------- | --------- | ------------- | -------------------------------------------------- |
| proxy_requests_total               | Counter   | status        | Proxied requests total                             |
| proxy_request_latency_seconds      | Histogram | status        | Proxied request latency histogram                  |
| proxy_errors_total                 | Counter   |               | Errors forwarding proxied requests                 |
| proxy_listeners                    | Gauge     |               | Number of registered upstream listeners            |
| http_requests_total                | Counter   | status        | HTTP requests total                                |
| http_request_latency_seconds       | Histogram | status        | HTTP request latency histogram                     |
