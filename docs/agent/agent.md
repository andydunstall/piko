# Agent

The Piko agent is a lightweight proxy that runs alongside your upstream
services. It connects to the Piko server and listens for traffic on the
configured endpoints, then forwards incoming requests to your services.
See [How Piko Works](../how-piko-works.md) for details.

Such as `piko agent http my-endpoint 3000` will listen for traffic
from Piko on endpoint `my-endpoint` and forward to your service at
`localhost:3000`.

<p align="center">
  <img src="../../assets/images/agent.png" alt="overview" width="60%"/>
</p>

You can run the agent using either `piko agent http` to start a single HTTP
listener, or `piko agent start` to start listeners configured in a YAML file.
See `piko agent -h` for details.

## Configuration

The Piko agent supports both YAML configuration and command-line flags.

The YAML file path can be set using `--config.path`.

See `piko agent -h` for the available configuration options.

### Variable Substitution

When enabling `--config.expand-env`, Piko will expand environment variables
in the loaded YAML configuration. This will replace references to `${VAR}`
and `$VAR` with the corresponding environment variable.

If the environment variable is not defined, it will be replaced with an empty
string. You can also define a default value using form `${VAR:default}`.

## YAML Configuration

The agent supports the following YAML configuration (where most parameters have
corresponding command line flags):

```
# Listeners contains the set of listeners to register. Each listener has an
# endpoint ID, address to forward connections to, whether to log each request
# and a timeout to forward requests to the upstreams.
listeners:
  - endpoint_id: my-endpoint
    # Address of the upstream, which may be a port, host and port, or URL.
    addr: localhost:3000
    # Whether to log all incoming HTTP requests as 'info'.
    access_log: true
    # Timeout forwarding incoming HTTP requests to the upstream.
    timeout: 15s

connect:
  # The Piko server URL to connect to. Note this must be configured to use the
  # Piko server 'upstream' port.
  url: http://localhost:8001

  # Token is a token to authenticate with the Piko server.
  token: ""

  # Timeout attempting to connect to the Piko server on boot. Note if the agent
  # is disconnected after the initial connection succeeds it will keep trying to
  # reconnect.
  timeout: 30s

  tls:
    # A path to a certificate PEM file containing root certificiate authorities to
    # validate the TLS connection to the Piko server.
    #
    # Defaults to using the host root CAs.
    root_cas: ""

server:
  The host/port to bind the server to.

  If the host is unspecified it defaults to all listeners, such as
  '--server.bind-addr :5000' will listen on '0.0.0.0:5000'.
  bind_addr: ":5000"

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

# Maximum duration after a shutdown signal is received (SIGTERM or
# SIGINT) to gracefully shutdown each listener.
grace_period: 1m0s
```

### TlS

To specify a custom root CA to validate the TLS connection to the Piko server,
use `--connect.tls.root-cas`.

### Authentication

To authenticate the agent, include a JWT in `connect.token`. See
[Server](../server/server.md) for details on JWT authentication with Piko.
