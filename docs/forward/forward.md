# Forward

Piko forward listens on a local port and forwards connections to the configured
upstream endpoint.

Such as you may listen on port 3000 and forward connections to endpoint
'my-endpoint'.

When using HTTP(S), you can connect to the Piko server directly using the
`Host` or `x-piko-endpoint` header to identify the endpoint ID, though when
using raw TCP you must first proxy the connection on the client to connect
to the configured endpoint.

Such as `piko forward tcp 3000 my-endpoint` listens locally on port `3000` then
forwards connections to endpoint `my-endpoint`.

<p align="center">
  <img src="../../assets/images/forward.png" alt="overview" width="60%"/>
</p>

## Configuration

Piko supports both YAML configuration and command-line flags.

The YAML file path can be set using `--config.path`.

See `piko agent -h` for the available configuration options.

## YAML Configuration

Piko forward supports the following YAML configuration (where most parameters
have corresponding command line flags):

```
# Ports contains the set of ports to listen on and the endpoint to forward to.
ports:
- addr: "3000"
  endpoint_id: my-endpoint

connect:
  # The Piko server URL to connect to. Note this must be configured to use the
  # Piko server 'proxy' port.
  url: http://localhost:8000

  # Timeout attempting to connect to the Piko server.
  timeout: 30s

  tls:
    # A path to a certificate PEM file containing root certificiate authorities to
    # validate the TLS connection to the Piko server.
    #
    # Defaults to using the host root CAs.
    root_cas: ""

log:
    # Minimum log level to output.
    #
    # The available levels are 'debug', 'info', 'warn' and 'error'.
    level: info

    # Each log has a 'subsystem' field where the log occured.
    #
    # '--log.subsystems' enables all log levels for those given subsystems. This
    # can be useful to debug a particular subsystem without having to enable all
    # debug logs.
    #
    # Such as you can enable 'gossip' logs with '--log.subsystems gossip'.
    subsystems: []
```

### TlS

To specify a custom root CA to validate the TLS connection to the Piko server,
use `--connect.tls.root-cas`.
