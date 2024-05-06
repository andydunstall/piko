# Configure

## Sources

Pico server and agent support both YAML configuration and command-line flags.

The YAML file path can be set using `--config.path`.

See `pico server -h` and `pico agent -h` for the available configuration
options.

### Variable Substitution

When enabling `--config.expand-env`, Pico will expand environment variables
in the loaded YAML configuration. This will replace references to `${VAR}`
and `$VAR` with the corresponding environment variable.

If the environment variable is not defined, it will be replaced with an empty
string. You can also defined a default value using form `${VAR:default}`.

## Server

The Pico server is run using `pico server`. It has the following configuration:
```
proxy:
    # The host/port to listen for incoming proxy HTTP requests.
    # 
    # If the host is unspecified it defaults to all listeners, such as
    # '--proxy.bind-addr :8000' will listen on '0.0.0.0:8000'.
    bind_addr: :8000

    # Proxy listen address to advertise to other nodes in the cluster. This is the
    # address other nodes will used to forward proxy requests.
    # 
    # Such as if the listen address is ':8000', the advertised address may be
    # '10.26.104.45:8000' or 'node1.cluster:8000'.
    # 
    # By default, if the bind address includes an IP to bind to that will be used.
    # If the bind address does not include an IP (such as ':8000') the nodes
    # private IP will be used, such as a bind address of ':8000' may have an
    # advertise address of '10.26.104.14:8000'.
    advertise_addr: ""

    # The timeout when sending proxied requests to upstream listeners for forwarding
    # to other nodes in the cluster.
    #
    # If the upstream does not respond within the given timeout a
    # '504 Gateway Timeout' is returned to the client.
    gateway_timeout: 15

upstream:
    # The host/port to listen for connections from upstream listeners.
    # 
    # If the host is unspecified it defaults to all listeners, such as
    # '--upstream.bind-addr :8001' will listen on '0.0.0.0:8001'.
    bind_addr: :8001

    # Upstream listen address to advertise to other nodes in the cluster.
    # 
    # Such as if the listen address is ':8001', the advertised address may be
    # '10.26.104.45:8001' or 'node1.cluster:8001'.
    # 
    # By default, if the bind address includes an IP to bind to that will be used.
    # If the bind address does not include an IP (such as ':8001') the nodes
    # private IP will be used, such as a bind address of ':8001' may have an
    # advertise address of '10.16.104.14:8001'.
    advertise_addr: ""

admin:
    # The host/port to listen for incoming admin connections.
    # 
    # If the host is unspecified it defaults to all listeners, such as
    # '--admin.bind-addr :8002' will listen on '0.0.0.0:8002'.
    bind_addr: :8002

    # Admin listen address to advertise to other nodes in the cluster. This is the
    # address other nodes will used to forward admin requests.
    # 
    # Such as if the listen address is ':8002', the advertised address may be
    # '10.26.104.45:8002' or 'node1.cluster:8002'.
    # 
    # By default, if the bind address includes an IP to bind to that will be used.
    # If the bind address does not include an IP (such as ':8002') the nodes
    # private IP will be used, such as a bind address of ':8002' may have an
    # advertise address of '10.26.104.14:8002'.
    advertise_addr: ""

gossip:
    # The host/port to listen for inter-node gossip traffic.
    # 
    # If the host is unspecified it defaults to all listeners, such as
    # '--gossip.bind-addr :8003' will listen on '0.0.0.0:8003'.
    bind_addr: :8003

    # Gossip listen address to advertise to other nodes in the cluster. This is the
    # address other nodes will used to gossip with the node.
    # 
    # Such as if the listen address is ':8003', the advertised address may be
    # '10.26.104.45:8003' or 'node1.cluster:8003'.
    # 
    # By default, if the bind address includes an IP to bind to that will be used.
    # If the bind address does not include an IP (such as ':8003') the nodes
    # private IP will be used, such as a bind address of ':8003' may have an
    # advertise address of '10.26.104.14:8003'.
    advertise_addr: ""

cluster:
    # A unique identifier for the node in the cluster.
    # 
    # By default a random ID will be generated for the node.
    node_id: ""

    # A prefix for the node ID.
    # 
    # Pico will generate a unique random identifier for the node and append it to
    # the given prefix.
    # 
    # Such as you could use the node or pod  name as a prefix, then add a unique
    # identifier to ensure the node ID is unique across restarts.
    node_id_prefix: ""

    # A list of addresses of members in the cluster to join.
    # 
    # This may be either addresses of specific nodes, such as
    # '--cluster.join 10.26.104.14,10.26.104.75', or a domain that resolves to
    # the addresses of the nodes in the cluster (e.g. a Kubernetes headless
    # service), such as '--cluster.join pico.prod-pico-ns'.
    # 
    # Each address must include the host, and may optionally include a port. If no
    # port is given, the gossip port of this node is used.
    # 
    # Note each node propagates membership information to the other known nodes,
    # so the initial set of configured members only needs to be a subset of nodes.
    join: []

server:
    # Maximum number of seconds after a shutdown signal is received (SIGTERM or
    # SIGINT) to gracefully shutdown the server node before terminating.
    # This includes handling in-progress HTTP requests, gracefully closing
    # connections to upstream listeners, announcing to the cluster the node is
    # leaving...
    graceful_shutdown_timeout: 60

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

## Agent

The Pico agent is run using `pico agent`. It has the following configuration:
```
# The endpoints to register with the Pico server.
# 
# Each endpoint has an ID and forwarding address. The agent will register the
# endpoint with the Pico server then receive incoming requests for that endpoint
# and forward them to the configured address.
# 
# '--endpoints' is a comma separated list of endpoints with format:
# '<endpoint ID>/<forward addr>'. Such as '--endpoints 6ae6db60/localhost:3000'
# will register the endpoint '6ae6db60' then forward incoming requests to
# 'localhost:3000'.
# 
# You may register multiple endpoints which have their own connection to Pico,
# such as '--endpoints 6ae6db60/localhost:3000,941c3c2e/localhost:4000'.
#
# (Required).
endpoints: []

server:
    # Pico server URL.
    # 
    # The listener will add path /pico/v1/listener/:endpoint_id to the given URL,
    # so if you include a path it will be used as a prefix.
    # 
    # Note Pico connects to the server with WebSockets, so will replace http/https
    # with ws/wss (you can configure either).
    url: http://localhost:8001

    # Heartbeat interval in seconds.
    # 
    # To verify the connection to the server is ok, the listener sends a
    # heartbeat to the upstream at the '--server.heartbeat-interval'
    # interval, with a timeout of '--server.heartbeat-timeout'.`,
    heartbeat_interval: 10

    # Heartbeat timeout in seconds.,
    # 
    # To verify the connection to the server is ok, the listener sends a
    # heartbeat to the upstream at the '--server.heartbeat-interval'
    heartbeat_timeout: 10

forwarder:
    # Forwarder timeout in seconds.
    # 
    # This is the timeout between a listener receiving a request from Pico then
    # forwarding it to the configured forward address, and receiving a response.
    # 
    # If the upstream does not respond within the given timeout a
    # '504 Gateway Timeout' is returned to the client.
    timeout: 10

admin:
    # The host/port to listen for incoming admin connections.
    # 
    # If the host is unspecified it defaults to all listeners, such as
    # '--admin.bind-addr :9000' will listen on '0.0.0.0:9000'
    bind_addr: :9000

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
