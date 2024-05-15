# Configure

## Sources

Piko server and agent support both YAML configuration and command-line flags.

The YAML file path can be set using `--config.path`.

See `piko server -h` and `piko agent -h` for the available configuration
options.

### Variable Substitution

When enabling `--config.expand-env`, Piko will expand environment variables
in the loaded YAML configuration. This will replace references to `${VAR}`
and `$VAR` with the corresponding environment variable.

If the environment variable is not defined, it will be replaced with an empty
string. You can also define a default value using form `${VAR:default}`.

## Server

The Piko server is run using `piko server`. It has the following configuration:
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
    gateway_timeout: 15s

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
    # Piko will generate a unique random identifier for the node and append it to
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
    # service), such as '--cluster.join piko.prod-piko-ns'.
    # 
    # Each address must include the host, and may optionally include a port. If no
    # port is given, the gossip port of this node is used.
    # 
    # Note each node propagates membership information to the other known nodes,
    # so the initial set of configured members only needs to be a subset of nodes.
    join: []

    # Whether the server node should abort if it is configured with more than one
    # node to join (excluding itself) but fails to join any members.
    abort_if_join_fails: true

auth:
    # Secret key to authenticate HMAC endpoint connection JWTs.
    token_hmac_secret_key: ""

    # Public key to authenticate RSA endpoint connection JWTs.
    token_rsa_public_key: ""

    # Public key to authenticate ECDSA endpoint connection JWTs.
    token_ecdsa_public_key: ""

    # Audience of endpoint connection JWT token to verify.
    #
    # If given the JWT 'aud' claim must match the given audience. Otherwise it
    # is ignored.
    token_audience: ""

    # Issuer of endpoint connection JWT token to verify.
    #
    # If given the JWT 'iss' claim must match the given issuer. Otherwise it
    # is ignored.
    token_issuer: ""

server:
    # Maximum duration after a shutdown signal is received (SIGTERM or
    # SIGINT) to gracefully shutdown the server node before terminating.
    # This includes handling in-progress HTTP requests, gracefully closing
    # connections to upstream listeners, announcing to the cluster the node is
    # leaving...
    graceful_shutdown_timeout: 1m

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

### Cluster

To deploy Piko as a cluster, configure `--cluster.join` to a list of cluster
members in the cluster to join.

The addresses may be either addresses of specific nodes, such as
`10.26.104.14`, or a domain name that resolves to the IP addresses of all nodes
in the cluster.

To deploy Piko to Kubernetes, you can create a headless service whose domain
resolves to the IP addresses of the pods in the service, such as
`piko.prod-piko-ns.svc.cluster.local`. When Piko starts, it will then attempt
to join the other pods.

### Authentication

To authenticate upstream endpoint connections, Piko can use a
[JSON Web Token (JWT)](https://jwt.io/) provided by your application.

You configure Piko with the secret key or public key to verify the JWT, then
configure the upstream endpoint with the JWT. Piko will then verify endpoints
connection token.

Piko supports HMAC, RSA, and ECDSA JWT algorithms, specifically HS256, HS384,
HS512, RSA256, RSA384, RSA512, EC256, EC384, and EC512.

The server has the following configuration options to pass a secret key or
public key:
- `auth.token_hmac_secret_key`: Add HMAC secret key
- `auth.token_rsa_public_key`: Add RSA public key
- `auth.token_ecdsa_public_key`: Add ECDSA public key

If no keys secret or public keys are given, Piko will allow unauthenticated
endpoint connections.

Piko will verify the `exp` (expiry) and `iat` (issued at) claims if given, and
drop the connection to the upstream endpoint once its token expires.

By default Piko will not verify the `aud` (audience) or `iss` (issuer) claims,
though you can enable these checks with `auth.token_audience` and
`auth.token_issuer` respectively.

You may also include Piko specific fields in your JWT. Piko supports the
`piko.endpoints` claim which contains an array of endpoint IDs the token is
permitted to register. Such as if the JWT includes claim
`"piko": {"endpoints": ["endpoint-123"]}`, it will be permitted to register
endpoint ID `endpoint-123` but not `endpoint-xyz`.

Note Piko does (yet) not authenticate proxy requests as proxy clients will
typically be deployed to the same network as the Pcio server. Your upstream
services may then authenticate incoming requests if needed after they've been
forwarded by Piko.

## Agent

The Piko agent is run using `piko agent`. It has the following configuration:
```
# The endpoints to register with the Piko server.
# 
# Each endpoint has an ID and forwarding address. The agent will register the
# endpoint with the Piko server then receive incoming requests for that endpoint
# and forward them to the configured address.
# 
# '--endpoints' is a comma separated list of endpoints with format:
# '<endpoint ID>/<forward addr>'. Such as '--endpoints 6ae6db60/localhost:3000'
# will register the endpoint '6ae6db60' then forward incoming requests to
# 'localhost:3000'.
# 
# You may register multiple endpoints which have their own connection to Piko,
# such as '--endpoints 6ae6db60/localhost:3000,941c3c2e/localhost:4000'.
#
# (Required).
endpoints: []

server:
    # Piko server URL.
    # 
    # The listener will add path /piko/v1/listener/:endpoint_id to the given URL,
    # so if you include a path it will be used as a prefix.
    # 
    # Note Piko connects to the server with WebSockets, so will replace http/https
    # with ws/wss (you can configure either).
    url: http://localhost:8001

    # Heartbeat interval.
    # 
    # To verify the connection to the server is ok, the listener sends a
    # heartbeat to the upstream at the '--server.heartbeat-interval'
    # interval, with a timeout of '--server.heartbeat-timeout'.`,
    heartbeat_interval: 10s

    # Heartbeat timeout.
    # 
    # To verify the connection to the server is ok, the listener sends a
    # heartbeat to the upstream at the '--server.heartbeat-interval'
    heartbeat_timeout: 10s

auth:
    # An API key to authenticate the connection to Piko.
    api_key: ""

forwarder:
    # Forwarder timeout.
    # 
    # This is the timeout between a listener receiving a request from Piko then
    # forwarding it to the configured forward address, and receiving a response.
    # 
    # If the upstream does not respond within the given timeout a
    # '504 Gateway Timeout' is returned to the client.
    timeout: 10s

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

### Authentication

To authenticate the agent, include a JWT in `auth.api_key`. The supported JWT
formats are described above in the server configuration.
